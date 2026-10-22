package ejecutar

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	catalogo "tramtai/internal/agentes"
	"tramtai/internal/cliente"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
)

type (
	// msgFlujo es el seguimiento en vivo recién abierto; msgVivo, lo que llega por él.
	msgFlujo struct {
		origen
		id    string
		seq   int
		canal <-chan msgVivo
	}
	// msgVivo es un evento del trabajo (clase "evento"), un corte del flujo ("corte"), el intento de reabrirlo ("abre") o el
	// final con el trabajo terminado ("fin").
	msgVivo struct {
		origen
		id, clase   string
		seq         int
		tipo, datos string
		t           nucleo.Trabajo
	}
)

// enviar sube los archivos y crea el trabajo; al volver, se sigue en vivo. Mientras haya uno en marcha no se envía otro.
func (p *Pagina) enviar() tea.Cmd {
	if p.enviando || p.activo() {
		return nil
	}
	if p.c.Cli == nil {
		return func() tea.Msg { return pagina.ErrorServicio{Err: cliente.ErrServicioInactivo} }
	}
	d := p.leer()
	esquema, err := esquemaDe(d.esquema)
	if err != nil {
		return comp.Aviso(p.tx("receta.esquemaInvalido"), comp.Fallo)
	}
	s := solicitudDe(d, nil, esquema)
	if esquema != nil {
		s.Salida.ConservarSalidaInvalida = true // Probar enseña la respuesta que no cumple y deja corregirla
	}
	rutas := rutasDe(d.archivos)
	c, o := p.c, p.origen()
	p.enviando = true
	return func() tea.Msg {
		if err := revisarArchivos(c, rutas); err != nil {
			return msgCreado{origen: o, err: err}
		}
		if len(rutas) > 0 {
			cfg, err := nucleo.CargarConfiguracion(c.Proyecto) // instancia_global.raices_permitidas, como la CLI
			if err != nil {
				return msgCreado{origen: o, err: err}
			}
			for _, ruta := range rutas {
				permitida, err := cfg.RutaPermitida(ruta)
				if err != nil {
					return msgCreado{origen: o, err: err}
				}
				carga, err := c.Cli.SubirCarga(c.Ctx, c.Instancia, permitida)
				if err != nil {
					return msgCreado{origen: o, err: err}
				}
				s.Cargas = append(s.Cargas, carga.ID)
			}
		}
		t, err := c.Cli.Crear(c.Ctx, c.Instancia, s)
		return msgCreado{origen: o, t: t, err: err, enviado: true}
	}
}

// cargarTrabajo lee un trabajo para seguirlo; con receta, además pone su receta y su mensaje en el formulario.
func (p *Pagina) cargarTrabajo(id string, receta bool) tea.Cmd {
	if p.c.Cli == nil {
		return func() tea.Msg { return pagina.ErrorServicio{Err: cliente.ErrServicioInactivo} }
	}
	c, o := p.c, p.origen()
	return func() tea.Msg {
		t, err := c.Cli.Trabajo(c.Ctx, c.Instancia, id)
		return msgCreado{origen: o, t: t, err: err, receta: receta}
	}
}

func (p *Pagina) alCreado(m msgCreado) tea.Cmd {
	if !p.vigente(m.origen) {
		return nil
	}
	p.enviando, p.ocupado = false, false
	if m.err != nil {
		return p.fallo(m.err)
	}
	var cmds []tea.Cmd
	if m.enviado {
		p.nborrador++ // un borrador que estaba por escribirse ya no vale
		cmds = append(cmds, escribirBorrador(p.c.Datos, ""))
	}
	if m.receta {
		p.cargarReceta(recetaDeSolicitud(m.t.Solicitud), mensajeDe(m.t))
		p.api, p.punto = "", nil
	}
	p.editando = false
	return tea.Batch(append(cmds, p.seguir(m.t), p.cargarRecientes())...)
}

// seguir enseña un trabajo en la pestaña Respuesta: terminado, tal como quedó; en marcha, en vivo.
func (p *Pagina) seguir(t nucleo.Trabajo) tea.Cmd {
	p.cerrarVivo()
	p.trabajo, p.final, p.estado, p.salida, p.avisos, p.corte, p.sigue = &t, nil, t.Estado, "", nil, false, true
	p.pestana = pestRespuesta
	if t.Estado.Terminal() {
		p.terminar(t)
		return nil
	}
	p.rehacerRespuesta()
	return p.abrirVivo(t.ID)
}

// terminar enseña el trabajo terminado. Con salida estructurada el texto en vivo puede diferir del resultado validado, y si
// no cumplía el esquema se enseña la última respuesta conservada (terminar de Ejecutar.vue).
func (p *Pagina) terminar(t nucleo.Trabajo) {
	p.estado = t.Estado
	if (t.Estado == nucleo.Completado || t.CodigoError == nucleo.CodigoSalidaNoCumpleEsquema) && t.Salida != "" {
		p.salida = t.Salida
	}
	p.final = &t
	p.cerrarVivo()
	p.rehacerRespuesta()
}

// cerrarVivo corta el seguimiento en vivo.
func (p *Pagina) cerrarVivo() {
	if p.cancelar != nil {
		p.cancelar()
	}
	p.cancelar, p.canal, p.corte = nil, nil, false
}

// abrirVivo sigue los eventos del trabajo en una goroutine que los manda por un canal. Se corta con el contexto de la
// página, con otro trabajo y al ocultarse.
func (p *Pagina) abrirVivo(id string) tea.Cmd {
	p.cerrarVivo()
	p.nflujo++
	seq := p.nflujo
	ctx, cancelar := context.WithCancel(p.c.Ctx)
	p.cancelar = cancelar
	o, c, espera, abrir := p.origen(), p.c, p.esperaPara, p.abrir
	if abrir == nil {
		abrir = c.Cli.Eventos
	}
	return func() tea.Msg {
		canal := make(chan msgVivo, 64)
		go seguir(ctx, o, c, id, seq, abrir, espera, canal)
		return msgFlujo{origen: o, id: id, seq: seq, canal: canal}
	}
}

// seguir mantiene abierto el flujo SSE del trabajo. Al acabar (el cliente no entrega el evento fin) o cortarse, consulta
// el trabajo: si terminó manda fin con él; si no, espera y reabre desde el último evento visto. Cierra el canal al terminar.
func seguir(ctx context.Context, o origen, c pagina.Contexto, id string, seq int,
	abrir func(context.Context, string, string, int, func(int, string, string) error) error,
	espera func(time.Duration) <-chan time.Time, canal chan<- msgVivo) {
	defer close(canal)
	enviar := func(m msgVivo) bool {
		m.origen, m.id, m.seq = o, id, seq
		select {
		case canal <- m:
			return true
		case <-ctx.Done():
			return false
		}
	}
	desde := 0
	for intento := 0; ; intento++ {
		if intento > 0 && !enviar(msgVivo{clase: "abre"}) {
			return
		}
		_ = abrir(ctx, c.Instancia, id, desde, func(numero int, tipo, datos string) error {
			if numero > 0 && numero <= desde { // tras reconectar, lo que ya había llegado
				return nil
			}
			desde = max(desde, numero)
			if !enviar(msgVivo{clase: "evento", tipo: tipo, datos: datos}) {
				return ctx.Err()
			}
			return nil
		})
		if ctx.Err() != nil {
			return
		}
		if t, err := c.Cli.Trabajo(ctx, c.Instancia, id); err == nil && t.Estado.Terminal() {
			enviar(msgVivo{clase: "fin", t: t})
			return
		}
		if !enviar(msgVivo{clase: "corte"}) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-espera(esperaCorte):
		}
	}
}

// esperar lee el siguiente mensaje del seguimiento; con el canal cerrado, nada.
func esperar(canal <-chan msgVivo) tea.Cmd {
	return func() tea.Msg {
		m, ok := <-canal
		if !ok {
			return nil
		}
		return m
	}
}

// vivoVigente dice si un mensaje del seguimiento es del trabajo y del seguimiento de ahora.
func (p *Pagina) vivoVigente(m msgVivo) bool {
	return p.vigente(m.origen) && p.trabajo != nil && m.id == p.trabajo.ID && m.seq == p.nflujo
}

// alVivo aplica un evento del trabajo, como los de Ejecutar.vue: el texto se suma, un aviso se enseña, una corrección
// vuelve a escribir la respuesta desde cero y el estado cambia.
func (p *Pagina) alVivo(m msgVivo) tea.Cmd {
	switch m.clase {
	case "corte":
		p.corte = true
	case "abre":
		p.corte = false
	case "fin":
		p.terminar(m.t)
		return p.cargarRecientes()
	default:
		p.corte = false
		switch m.tipo {
		case "texto":
			p.salida += m.datos
		case "aviso":
			p.avisos = append(p.avisos, m.datos)
		case "estado":
			p.estado = nucleo.Estado(m.datos)
		case "intento":
			var i struct {
				Correccion int `json:"correccion"`
			}
			if json.Unmarshal([]byte(m.datos), &i) == nil && i.Correccion != 0 {
				p.salida = ""
			}
		}
	}
	p.rehacerRespuesta()
	return nil
}

// cancelarTrabajo pide cancelar el trabajo en marcha (Esc).
func (p *Pagina) cancelarTrabajo() tea.Cmd {
	if p.ocupado || p.c.Cli == nil {
		return nil
	}
	p.ocupado = true
	c, o, id := p.c, p.origen(), p.trabajo.ID
	return func() tea.Msg {
		t, err := c.Cli.Accion(c.Ctx, c.Instancia, id, "cancelar")
		return msgCancelado{origen: o, t: t, err: err}
	}
}

// puedeCorregir: «Corregir» solo existe si la respuesta no cumplía el esquema y se conservó.
func puedeCorregir(t nucleo.Trabajo) bool {
	return t.CodigoError == nucleo.CodigoSalidaNoCumpleEsquema && t.Salida != ""
}

// corregir pide al mismo agente que corrija la respuesta (o): es otra petición, que se sigue en vivo.
func (p *Pagina) corregir() tea.Cmd {
	switch {
	case p.final == nil || p.ocupado || p.c.Cli == nil:
		return nil
	case !puedeCorregir(*p.final):
		return comp.Aviso(p.tx("error.correccion_no_disponible"), comp.Advertencia)
	}
	p.ocupado = true
	c, o, id := p.c, p.origen(), p.final.ID
	return func() tea.Msg {
		t, err := c.Cli.Corregir(c.Ctx, c.Instancia, id)
		return msgCreado{origen: o, t: t, err: err}
	}
}

// nuevaPrueba deja la página lista para otro mensaje: la receta se conserva y el trabajo anterior queda en el Historial.
func (p *Pagina) nuevaPrueba() tea.Cmd {
	if p.final == nil { // como el botón de la Web UI: solo con una respuesta terminada
		return nil
	}
	p.cerrarVivo()
	p.trabajo, p.final, p.salida, p.avisos, p.conversor = nil, nil, "", nil, ""
	p.form.Poner("mensaje", "")
	p.form.Poner("archivos", "")
	p.form.Enfocar("mensaje")
	p.pestana = pestReceta
	p.rehacerRespuesta()
	return p.programarBorrador()
}

// icono es el del estado y su estilo, como en el Historial.
func (p *Pagina) icono(e nucleo.Estado) (string, func(...string) string) {
	est := p.estilos
	switch {
	case !e.Terminal():
		return est.Iconos.Activo, est.Acento.Render
	case e == nucleo.Completado:
		return est.Iconos.Ok, est.Exito.Render
	case e == nucleo.Cancelado:
		return est.Iconos.Inactivo, est.Tenue.Render
	case e == nucleo.Interrumpido:
		return est.Iconos.Fallo, est.Advertencia.Render
	}
	return est.Iconos.Fallo, est.Error.Render
}

// nombreEstado es el de la Web UI (estado.<estado>); si no lo conoce, el estado tal cual.
func (p *Pagina) nombreEstado(e nucleo.Estado) string {
	if t := p.tx("estado." + string(e)); t != "estado."+string(e) {
		return t
	}
	return string(e)
}

// rehacerRespuesta vuelve a escribir el texto de la respuesta; en vivo sigue el final mientras no se use el visor.
func (p *Pagina) rehacerRespuesta() {
	if p.estilos == nil {
		return
	}
	p.visor.Poner(p.a(p.textoRespuesta()))
	if p.sigue && p.activo() {
		p.visor.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	}
}

// textoRespuesta es lo que se ve bajo el estado: los avisos, el error, la respuesta y, al terminar, el agente y el consumo.
// Sin trabajo, lo que invita a probar.
func (p *Pagina) textoRespuesta() string {
	e := p.estilos
	if p.trabajo == nil {
		lineas := []string{p.tx("ejecutar.vacio"), "", p.tx("ejecutar.ejemplos")}
		for _, k := range []string{"ejecutar.ejemplo1", "ejecutar.ejemplo2", "ejecutar.ejemplo3"} {
			lineas = append(lineas, "  "+e.Iconos.Cursor+" "+p.tx(k))
		}
		return strings.Join(lineas, "\n")
	}
	var lineas []string
	for _, a := range p.avisos {
		lineas = append(lineas, e.Advertencia.Render("! "+a))
	}
	if f := p.final; f != nil && f.CodigoError != "" {
		titulo := p.tx("error." + string(f.CodigoError))
		if titulo == "error."+string(f.CodigoError) {
			titulo = string(f.CodigoError)
		}
		if f.MensajeError != "" {
			titulo += ": " + f.MensajeError
		}
		lineas = append(lineas, e.Error.Render(e.Iconos.Fallo+" "+titulo), "")
		if puedeCorregir(*f) {
			lineas = append(lineas, e.Acento.Render("o "+p.tx("ejecutar.corregir"))+" "+e.Tenue.Render(p.tx("ejecutar.corregirTexto")), "",
				e.Tenue.Render(p.tx("ejecutar.respuestaRecibida")))
		}
	}
	switch {
	case p.salida != "":
		lineas = append(lineas, p.salida)
	case p.activo():
		lineas = append(lineas, e.Tenue.Render(p.nombreEstado(p.estado)+"…"))
	}
	if f := p.final; f != nil {
		lineas = append(lineas, "", e.Tenue.Render(strings.Join(metadatos(*f, p.tx("comun.tokens")), " "+e.Iconos.Separador+" ")))
	}
	return strings.Join(lineas, "\n")
}

// metadatos son el agente y el modelo, los tokens, el coste y la duración del trabajo terminado.
func metadatos(t nucleo.Trabajo, tokens string) []string {
	var partes []string
	agente := catalogo.NombreVisible(t.Solicitud.Agente)
	if m := t.ModeloEfectivo; m != "" || t.ModeloEnviado != "" {
		if m == "" {
			m = t.ModeloEnviado
		}
		agente = strings.TrimSpace(agente + " " + m)
	}
	if agente != "" {
		partes = append(partes, agente)
	}
	partes = append(partes, fmt.Sprintf("%d → %d %s", t.Uso.TokensEntrada, t.Uso.TokensSalida, tokens))
	if t.Uso.CostoUSD > 0 {
		partes = append(partes, fmt.Sprintf("US$ %.4f", t.Uso.CostoUSD))
	}
	if t.TerminadoEn != nil && !t.CreadoEn.IsZero() {
		s := t.TerminadoEn.Sub(t.CreadoEn).Seconds()
		if s < 60 {
			partes = append(partes, fmt.Sprintf("%.1f s", s))
		} else {
			partes = append(partes, fmt.Sprintf("%d min %d s", int(s/60), int(math.Round(math.Mod(s, 60)))))
		}
	}
	return partes
}
