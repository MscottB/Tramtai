package historial

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/vivo"
)

// Las pestañas del detalle: el mensaje (con la ficha), la respuesta y el registro.
const (
	pestanaMensaje = iota
	pestanaRespuesta
	pestanaRegistro
)

// enVivo dice si el detalle sigue una petición que no terminó.
func (p *Pagina) enVivo() bool {
	return p.cancelar != nil && p.detalle != nil && !p.detalle.Estado.Terminal()
}

// vistaDetalle es la cabecera (estado, ruta y pestañas) y el visor con el texto de la pestaña.
func (p *Pagina) vistaDetalle() string {
	e, ancho := p.estilos, max(p.c.Ancho, 1)
	if p.detalle == nil {
		return e.Tenue.Render(p.tx("comun.cargando"))
	}
	t := *p.detalle
	icono, estilo := vivo.Icono(e, t.Estado)
	cab := estilo.Render(icono+" "+p.nombreEstado(t.Estado)) + e.Tenue.Render(" "+e.Iconos.Separador+" "+fecha(t.CreadoEn))
	switch {
	case p.enVivo() && p.corte:
		cab += " " + e.Advertencia.Render(p.a(idioma.T("reconectando…")))
	case p.enVivo():
		cab += " " + e.Tenue.Render(e.Iconos.Separador) + " " + e.Acento.Render(e.Iconos.Activo+" "+idioma.T("en vivo"))
	}
	return strings.Join([]string{e.Recortar(cab, ancho), e.Recortar(p.a(vivo.Ruta(e, p.lengua, t.Estado)), ancho), e.Recortar(p.pestanas(), ancho), p.visor.Vista()}, "\n")
}

// pestanas son los nombres de las pestañas, con la de ahora entre corchetes.
func (p *Pagina) pestanas() string {
	e := p.estilos
	nombres := []string{p.tx("historial.solicitud"), p.tx("ejecutar.respuesta"), p.tx("registro.titulo")}
	partes := make([]string, len(nombres))
	for i, n := range nombres {
		if i == p.pestana {
			partes[i] = e.Titulo.Render("[" + n + "]")
		} else {
			partes[i] = e.Tenue.Render(" " + n + " ")
		}
	}
	return strings.Join(partes, " ")
}

// rehacerVisor vuelve a escribir el texto de la pestaña de ahora; la respuesta en vivo sigue el final mientras no se use
// el visor.
func (p *Pagina) rehacerVisor() {
	if p.detalle == nil || p.estilos == nil {
		return
	}
	t := *p.detalle
	var texto string
	switch p.pestana {
	case pestanaMensaje:
		texto = p.textoMensaje(t)
	case pestanaRespuesta:
		texto = p.textoRespuesta(t)
	default:
		texto = p.textoRegistro(t)
	}
	p.visor.Poner(envolver(p.a(texto), p.c.Ancho))
	if p.pestana == pestanaRespuesta && p.sigue && p.enVivo() {
		p.visorTecla("G")
	}
}

// envolver parte cada línea del texto por palabras en líneas de como mucho ancho columnas: el visor, si no, parte donde caiga.
func envolver(texto string, ancho int) string {
	lineas := strings.Split(texto, "\n")
	for i, l := range lineas {
		lineas[i] = ansi.Wrap(l, max(ancho, 1), "")
	}
	return strings.Join(lineas, "\n")
}

// fecha es la fecha y hora locales de una petición.
func fecha(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

// duracion es lo que tardó una petición (duracion de formato.ts).
func duracion(desde time.Time, hasta *time.Time) string {
	if hasta == nil || desde.IsZero() {
		return ""
	}
	s := hasta.Sub(desde).Seconds()
	if s < 60 {
		return fmt.Sprintf("%.1f s", s)
	}
	return fmt.Sprintf("%d min %d s", int(s/60), int(math.Round(math.Mod(s, 60))))
}

// banner es el error de la petición: el título de su código (o el código, si no lo conoce) y el mensaje del servicio.
func (p *Pagina) banner(t nucleo.Trabajo) []string {
	if t.CodigoError == "" {
		return nil
	}
	titulo := p.tx("error." + string(t.CodigoError))
	if titulo == "error."+string(t.CodigoError) {
		titulo = string(t.CodigoError)
	}
	if t.MensajeError != "" {
		titulo += ": " + t.MensajeError
	}
	return []string{p.estilos.Iconos.Fallo + " " + titulo, ""}
}

// textoMensaje es lo que se pidió y la ficha de la petición (agente y modelo, esfuerzo, origen, tokens, coste, duración e
// identificador).
func (p *Pagina) textoMensaje(t nucleo.Trabajo) string {
	l := p.banner(t)
	l = append(l, cmp.Or(textoDe(t, p.lengua), "—"), "")
	fila := func(clave, valor string) { l = append(l, p.tx(clave)+": "+valor) }
	fila("historial.agenteModelo", modeloDe(t))
	fila("historial.esfuerzo", cmp.Or(p.esfuerzoUsado(t), "—"))
	fila("historial.origen", p.textoOrigen(t))
	fila("historial.tokens", p.a(fmt.Sprintf("%d → %d", t.Uso.TokensEntrada, t.Uso.TokensSalida)))
	if t.Uso.CostoUSD > 0 {
		fila("historial.coste", fmt.Sprintf("US$ %.4f", t.Uso.CostoUSD))
	}
	if d := duracion(t.CreadoEn, t.TerminadoEn); d != "" {
		fila("historial.duracion", d)
	}
	fila("historial.identificador", t.ID)
	return strings.Join(l, "\n")
}

// textoRespuesta es la respuesta: la que se va escribiendo si sigue en marcha, la que dejó, o por qué no hay (caducó,
// no hubo o todavía no la hay).
func (p *Pagina) textoRespuesta(t nucleo.Trabajo) string {
	l := p.banner(t)
	salida := t.Salida
	if !t.Estado.Terminal() && p.vivoSalida != "" {
		salida = p.vivoSalida
	}
	switch {
	case t.Disponibilidad == nucleo.SalidaExpirada:
		l = append(l, p.tx("historial.expirada"))
	case salida != "":
		l = append(l, salida)
	case t.Estado.Terminal():
		l = append(l, p.tx("historial.sinRespuesta"))
	default:
		l = append(l, p.tx("historial.todaviaEnCurso"))
	}
	return strings.Join(l, "\n")
}

// numero escribe un número con los separadores del idioma («200.000»); en español, como Intl, no agrupa los de cuatro cifras.
func (p *Pagina) numero(n int) string {
	s := strconv.Itoa(max(n, -n))
	sep := "."
	if p.lengua == "en" {
		sep = ","
	} else if len(s) <= 4 {
		sep = ""
	}
	if sep != "" {
		for i := len(s) - 3; i > 0; i -= 3 {
			s = s[:i] + sep + s[i:]
		}
	}
	if n < 0 {
		return "-" + s
	}
	return s
}

// tiempo es una duración corta: «250 ms» o «1,5 s».
func (p *Pagina) tiempo(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%d ms", max(int(d.Round(time.Millisecond)/time.Millisecond), 0))
	}
	s := strconv.FormatFloat(math.Round(d.Seconds()*10)/10, 'f', -1, 64)
	if p.lengua != "en" {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s + " s"
}

// nombreEsfuerzo es el nivel de esfuerzo con el nombre de la Web UI; sin nivel, «El del agente».
func (p *Pagina) nombreEsfuerzo(nivel string) string {
	if nivel == "" {
		return p.tx("esfuerzo.agente")
	}
	return p.tx("esfuerzo." + nivel)
}

// intento es lo que el registro guarda de cada intento con un agente.
type intento struct {
	Agente         string `json:"agente"`
	Modelo         string `json:"modelo"`
	Esfuerzo       string `json:"esfuerzo"`
	DelAgente      bool   `json:"del_agente"`
	ContextoTokens int    `json:"contexto_tokens"`
	Correccion     int    `json:"correccion"`
	Correcciones   int    `json:"correcciones"`
}

// esfuerzoUsado es el esfuerzo con que respondió la petición, según su último intento: el pedido o, si no se pidió, el del
// agente con «(del agente)»; sin intentos registrados, el de la solicitud. «» si no hubo esfuerzo (esfuerzoUsado de formato.ts).
func (p *Pagina) esfuerzoUsado(t nucleo.Trabajo) string {
	var ultimo *nucleo.EventoRegistro
	for i := len(p.registro) - 1; i >= 0; i-- {
		if p.registro[i].Tipo == "intento" {
			ultimo = &p.registro[i]
			break
		}
	}
	if ultimo == nil {
		if t.Solicitud.Esfuerzo != "" {
			return p.nombreEsfuerzo(t.Solicitud.Esfuerzo)
		}
		return ""
	}
	var i intento
	if json.Unmarshal([]byte(ultimo.Datos), &i) != nil || i.Esfuerzo == "" {
		return ""
	}
	if i.DelAgente {
		return p.tx("esfuerzo.delAgente", "nivel", p.nombreEsfuerzo(i.Esfuerzo))
	}
	return p.nombreEsfuerzo(i.Esfuerzo)
}

// textoIntento resume un intento: el agente y modelo, la corrección, el esfuerzo y el contexto (textoIntento de RegistroTrabajo.vue).
func (p *Pagina) textoIntento(datos string) string {
	var i intento
	if json.Unmarshal([]byte(datos), &i) != nil {
		return ""
	}
	agente := nombreAgente(i.Agente)
	if i.Modelo != "" {
		agente += "/" + i.Modelo
	}
	partes := []string{agente}
	if i.Correccion != 0 {
		partes = append([]string{p.tx("registro.correccion", "n", itoa(i.Correccion), "de", itoa(cmp.Or(i.Correcciones, i.Correccion)))}, partes...)
	}
	if i.Esfuerzo != "" {
		nivel := strings.ToLower(p.nombreEsfuerzo(i.Esfuerzo))
		if i.DelAgente {
			nivel += " (" + p.tx("registro.delAgente") + ")"
		}
		partes = append(partes, nivel)
	}
	if i.ContextoTokens != 0 {
		partes = append(partes, p.tx("registro.contexto", "n", p.numero(i.ContextoTokens)))
	}
	return strings.Join(partes, " · ")
}

// textoEntrada dice cuánto ocupa lo que recibe el agente y qué documento pesa más.
func (p *Pagina) textoEntrada(datos string) string {
	var m struct {
		Tokens     int `json:"tokens_estimados"`
		Documentos []struct {
			Nombre string `json:"nombre"`
			Tokens int    `json:"tokens_estimados"`
		} `json:"documentos"`
	}
	if json.Unmarshal([]byte(datos), &m) != nil {
		return ""
	}
	texto := p.tx("registro.entrada", "n", p.numero(m.Tokens))
	if len(m.Documentos) > 0 {
		texto += fmt.Sprintf(" (%s ~%s)", m.Documentos[0].Nombre, p.numero(m.Documentos[0].Tokens))
	}
	return texto
}

// textoValidacion resume por qué la respuesta no cumplía el esquema: el primer problema y cuántos más.
func (p *Pagina) textoValidacion(datos string) string {
	var v struct {
		Problemas []struct {
			Texto string `json:"texto"`
		} `json:"problemas"`
	}
	if json.Unmarshal([]byte(datos), &v) != nil || len(v.Problemas) == 0 {
		return ""
	}
	texto := p.tx("registro.validacion", "texto", v.Problemas[0].Texto)
	if n := len(v.Problemas) - 1; n > 0 {
		texto += " " + p.tx("registro.validacionMas", "n", itoa(n))
	}
	return texto
}

// etapaDe agrupa los estados en las etapas del resumen del registro (etapaDe de RegistroTrabajo.vue).
var etapaDe = map[nucleo.Estado]string{nucleo.EnCola: "cola", nucleo.PreparandoEntrada: "preparacion", nucleo.EsperandoAgente: "preparacion",
	nucleo.Ejecutando: "ejecucion", nucleo.CancelacionSolicitada: "ejecucion", nucleo.ValidandoSalida: "validacion"}

// textoRegistro es la hora de cada paso de la petición (estados con su intento, avisos, entrada y validación) y, arriba,
// cuánto duró cada etapa.
func (p *Pagina) textoRegistro(t nucleo.Trabajo) string {
	switch {
	case p.errRegistro:
		return p.tx("registro.noDisponible")
	case p.registro == nil && p.cargaReg:
		return p.tx("comun.cargando")
	}
	var l []string
	for i, ev := range p.registro {
		var texto string
		switch ev.Tipo {
		case "estado":
			texto = p.nombreEstado(nucleo.Estado(ev.Datos))
			if i+1 < len(p.registro) && p.registro[i+1].Tipo == "intento" {
				texto += " · " + p.textoIntento(p.registro[i+1].Datos)
			}
		case "aviso":
			texto = p.tx("registro.aviso", "texto", ev.Datos)
		case "entrada":
			texto = p.textoEntrada(ev.Datos)
		case "validacion":
			texto = p.textoValidacion(ev.Datos)
		}
		if texto != "" {
			l = append(l, ev.CreadoEn.Local().Format("15:04:05.000")+"  +"+p.tiempo(ev.CreadoEn.Sub(t.CreadoEn))+"  "+p.a(texto))
		}
	}
	if resumen := p.etapas(); resumen != "" {
		l = append([]string{resumen, ""}, l...)
	}
	return strings.Join(l, "\n")
}

// etapas son las estaciones de la ruta con lo que duró cada una, como el resumen bajo el título del registro: cada tramo
// cuenta desde su estado hasta el siguiente.
func (p *Pagina) etapas() string {
	var estados []nucleo.EventoRegistro
	for _, ev := range p.registro {
		if ev.Tipo == "estado" {
			estados = append(estados, ev)
		}
	}
	suma := map[string]time.Duration{}
	for i := 0; i+1 < len(estados); i++ {
		if etapa, ok := etapaDe[nucleo.Estado(estados[i].Datos)]; ok {
			suma[etapa] += estados[i+1].CreadoEn.Sub(estados[i].CreadoEn)
		}
	}
	var partes []string
	for _, etapa := range []string{"cola", "preparacion", "ejecucion", "validacion"} {
		if d, ok := suma[etapa]; ok {
			partes = append(partes, p.tx("ruta."+etapa)+" "+p.tiempo(d))
		}
	}
	return strings.Join(partes, " · ")
}
