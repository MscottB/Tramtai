package ejecutar

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/agentes"
	catalogo "tramtai/internal/agentes"
	"tramtai/internal/configuracion"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/vivo"
)

// msgFlujo es el seguimiento en vivo recién abierto; msgVivo, lo que llega por él (con el origen de Probar).
type (
	msgFlujo = vivo.Flujo[origen]
	msgVivo  = vivo.Msg[origen]
)

// enviar sube los archivos y crea el trabajo; al volver, se sigue en vivo. Mientras haya uno en marcha no se envía otro.
func (p *Pagina) enviar() tea.Cmd {
	if p.enviando || p.activo() {
		return nil
	}
	if p.c.Cli == nil {
		return sinServicio
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
			cfg, err := configuracion.CargarConfiguracion(c.Proyecto, agentes.Internos()) // instancia_global.raices_permitidas, como la CLI
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
		return msgCreado{origen: o, t: t, err: err, enviado: true, nuevo: true}
	}
}

// cargarTrabajo lee un trabajo para seguirlo; con receta, además pone su receta y su mensaje en el formulario.
func (p *Pagina) cargarTrabajo(id string, receta bool) tea.Cmd {
	if p.c.Cli == nil {
		return sinServicio
	}
	c, o := p.c, p.origen()
	return func() tea.Msg {
		t, err := c.Cli.Trabajo(c.Ctx, c.Instancia, id)
		return msgCreado{origen: o, t: t, err: err, receta: receta}
	}
}

// alCreado sigue el trabajo. Uno nuevo (enviado o corregido) vale aunque la página se ocultara mientras tanto: ya existe en
// el servicio, y así no se pierde ni se manda otra vez.
func (p *Pagina) alCreado(m msgCreado) tea.Cmd {
	if !p.vigente(m.origen) && !(m.nuevo && p.delEspacio(m.origen)) {
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
	cmds = append(cmds, p.seguir(m.t))
	if p.visible { // al volver, Mostrar las carga
		cmds = append(cmds, p.cargarRecientes())
	}
	return tea.Batch(cmds...)
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
	if !p.visible { // oculta no se sigue: Mostrar lo abre
		return nil
	}
	return p.abrirVivo(t.ID)
}

// terminar enseña el trabajo terminado. Con salida estructurada el texto en vivo puede diferir del resultado validado, y si
// no cumplía el esquema se enseña la última respuesta conservada.
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
	ctx, cancelar := context.WithCancel(p.c.Ctx)
	p.cancelar = cancelar
	eventos := p.abrir
	if eventos == nil {
		eventos = p.c.Cli.Eventos
	}
	return vivo.Abrir(ctx, p.origen(), p.c.Instancia, id, p.nflujo, eventos, p.c.Cli.Trabajo, p.esperaPara)
}

// vivoVigente dice si un mensaje del seguimiento es del trabajo y del seguimiento de ahora.
func (p *Pagina) vivoVigente(m msgVivo) bool {
	return p.vigente(m.Origen) && p.trabajo != nil && m.ID == p.trabajo.ID && m.Seq == p.nflujo
}

// alVivo aplica un evento del trabajo: el texto se suma, un aviso se enseña, una corrección vuelve a escribir la
// respuesta desde cero y el estado cambia.
func (p *Pagina) alVivo(m msgVivo) tea.Cmd {
	switch m.Clase {
	case "corte":
		p.corte = true
	case "abre":
		p.corte = false
	case "fin":
		p.terminar(m.T)
		return p.cargarRecientes()
	default:
		p.corte = false
		switch m.Tipo {
		case "texto":
			p.salida += m.Datos
		case "aviso":
			p.avisos = append(p.avisos, m.Datos)
		case "estado":
			p.estado = nucleo.Estado(m.Datos)
		case "intento":
			var i struct {
				Correccion int `json:"correccion"`
			}
			if json.Unmarshal([]byte(m.Datos), &i) == nil && i.Correccion != 0 {
				p.salida = ""
			}
		}
	}
	p.rehacerRespuesta()
	return nil
}

// cancelarTrabajo pide cancelar el trabajo en marcha (c, tras confirmar).
func (p *Pagina) cancelarTrabajo() tea.Cmd {
	switch {
	case p.c.Cli == nil:
		return sinServicio
	case p.ocupado:
		return nil
	}
	p.ocupado = true
	c, o, id := p.c, p.origen(), p.trabajo.ID
	return func() tea.Msg {
		t, err := c.Cli.Accion(c.Ctx, c.Instancia, id, "cancelar")
		return msgCancelado{origen: o, t: t, err: err}
	}
}

// corregir pide al mismo agente que corrija la respuesta (o): es otra petición, que se sigue en vivo.
func (p *Pagina) corregir() tea.Cmd {
	switch {
	case p.final == nil || p.ocupado:
		return nil
	case p.c.Cli == nil:
		return sinServicio
	case !p.final.Corregible:
		return comp.Aviso(p.tx("error.correccion_no_disponible"), comp.Advertencia)
	}
	p.ocupado = true
	c, o, id := p.c, p.origen(), p.final.ID
	return func() tea.Msg {
		t, err := c.Cli.Corregir(c.Ctx, c.Instancia, id)
		return msgCreado{origen: o, t: t, err: err, nuevo: true}
	}
}

// nuevaPrueba deja la página lista para otro mensaje: la receta se conserva y el trabajo anterior queda en el Historial.
func (p *Pagina) nuevaPrueba() tea.Cmd {
	if p.final == nil { // solo con una respuesta terminada
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

// nombreEstado es el de la Web UI, sin caracteres de dibujo en una consola ASCII.
func (p *Pagina) nombreEstado(e nucleo.Estado) string { return p.a(vivo.NombreEstado(p.lengua, e)) }

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
		aviso := e.AvisoError(p.lengua, f.CodigoError, f.MensajeError)
		lineas = append(lineas, e.Error.Render(e.Iconos.Fallo+" "+aviso[0]))
		for _, linea := range aviso[1:] {
			lineas = append(lineas, e.Tenue.Render("  "+linea))
		}
		lineas = append(lineas, "")
		if f.Corregible {
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
