package conectar

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	catalogo "tramtai/internal/agentes"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/vivo"
)

// Vista es lo abierto (el formulario o el detalle) o la pestaña de ahora, con la fila de pestañas arriba.
func (p *Pagina) Vista() string {
	e, ancho, alto := p.estilos, p.c.Ancho, p.c.Alto
	if e == nil {
		return ""
	}
	if p.form != nil {
		return p.vistaForm()
	}
	if v, ok := e.Preambulo(p.lengua, ancho, p.sinServ, !p.cargado && p.err == ""); ok {
		return v
	}
	if !p.cargado { // la carga falló: el mensaje del servicio tal cual y cómo reintentar, sin invitar a crear la primera
		return "\n" + e.Error.Width(ancho).Render(p.tx("comun.errorCarga")+": "+p.err) + "\n" + e.Tenue.Render(e.Recortar("r "+p.tx("comun.reintentar"), ancho))
	}
	if p.detalle != "" {
		return p.vistaDetalle()
	}
	lineas := []string{p.pestanas()}
	if p.pestana == pestanaClave {
		return p.conVisor(lineas, p.textoClave())
	}
	if p.resumen.SugerirApiPropia {
		lineas = append(lineas, e.Advertencia.Render(e.Recortar(p.tx("claves.sugerencia", "n", strconv.Itoa(p.resumen.AppConClave24h)), ancho)))
	}
	switch {
	case p.buscando:
		lineas = append(lineas, e.Acento.Render(e.Recortar(p.tx("puntos.buscarApi")+": "+p.entrada+e.Iconos.Vertical, ancho)))
	case p.filtros.hay():
		lineas = append(lineas, e.Tenue.Render(e.Recortar(p.resumenFiltros(), ancho)))
	case alto >= 12:
		lineas = append(lineas, e.Tenue.Render(e.Recortar(p.tx("conectar.subtitulo"), ancho)))
	}
	switch {
	case len(p.datos.Puntos) == 0: // quien empieza busca la URL y la clave: el vacío lleva antes a la clave del espacio
		lineas = append(lineas, "")
		lineas = append(lineas, e.Envolver(p.tx("puntos.vacio"), ancho, 0)...)
		lineas = append(lineas, "", e.Tenue.Render(e.Recortar("n "+p.tx("puntos.nuevo")+" "+e.Iconos.Separador+" → "+p.tx("puntos.verClave"), ancho)))
	case p.ninguna():
		lineas = append(lineas, "", e.Tenue.Render(e.Recortar(p.tx("puntos.sinResultados"), ancho)),
			e.Tenue.Render(e.Recortar("l "+p.tx("puntos.limpiarFiltros"), ancho)))
	default:
		lineas = append(lineas, p.lista.Vista(ancho, max(alto-len(lineas), 1)))
	}
	return recortarAlto(strings.Join(lineas, "\n"), alto)
}

// recortarAlto deja el texto en alto líneas como mucho.
func recortarAlto(texto string, alto int) string {
	l := strings.Split(texto, "\n")
	return strings.Join(l[:min(len(l), max(alto, 1))], "\n")
}

// conVisor pinta las líneas de arriba y, en lo que queda, el texto en el visor (que se desplaza con las flechas).
func (p *Pagina) conVisor(arriba []string, texto string) string {
	p.visor.Tamano(max(p.c.Ancho, 1), max(p.c.Alto-len(arriba), 1))
	p.visor.Poner(strings.Join(p.estilos.Envolver(texto, p.c.Ancho, 0), "\n")) // por palabras: el visor partiría a media palabra
	return recortarAlto(strings.Join(append(arriba, p.visor.Vista()), "\n"), p.c.Alto)
}

// pestanas son las de la Web UI: las APIs por aplicación (con cuántas hay) y la clave del espacio. La elegida va entre
// corchetes, como los filtros de Agentes.
func (p *Pagina) pestanas() string {
	e := p.estilos
	nombres := []string{p.tx("puntos.pestana", "n", strconv.Itoa(len(p.datos.Puntos))), p.tx("puntos.principal")}
	partes := make([]string, len(nombres))
	for i, n := range nombres {
		if i == p.pestana {
			partes[i] = e.Titulo.Render("[" + n + "]")
		} else {
			partes[i] = e.Tenue.Render(n)
		}
	}
	return e.Recortar(strings.Join(partes, " "), p.c.Ancho)
}

// fila es una API en la lista: su nombre, si está activa y lo que se ha usado.
func (p *Pagina) fila(x contrato.EstadoPunto) string {
	e := p.estilos
	icono, estilo, est := e.Iconos.Activo, e.Exito, p.tx("puntos.activo")
	switch {
	case !activo(x):
		icono, estilo, est = e.Iconos.Inactivo, e.Tenue, p.tx("puntos.desactivado")
	case x.Error != "":
		icono, estilo = e.Iconos.Fallo, e.Error
	}
	fila := pagina.Rellenar(e.Recortar(x.Nombre, 20), 20) + " " + estilo.Render(pagina.Rellenar(icono+" "+est, 11)) + " "
	if pr, ok := p.datos.Presupuestos[x.Nombre]; ok { // la barra corta de su límite más usado, como las de la Web UI en la fila
		mas := 0.0
		for _, b := range barras(p.lengua, pr) {
			mas = max(mas, b.fraccion)
		}
		fila += p.barra(mas, 8) + " " + strconv.Itoa(int(math.Floor(100*mas+1e-9))) + " % "
	}
	return fila + e.Tenue.Render(p.textoUso(x.Nombre))
}

// barra dibuja la fracción usada en ancho columnas: verde, amarilla desde el 80 % y roja al 100 %, como la Web UI; sin
// color se lee por lo lleno.
func (p *Pagina) barra(fraccion float64, ancho int) string {
	e := p.estilos
	lleno, vacio := "█", "░"
	if e.ASCII {
		lleno, vacio = "#", "-"
	}
	n := int(math.Round(min(fraccion, 1) * float64(ancho)))
	estilo := e.Exito
	switch {
	case fraccion >= 1:
		estilo = e.Error
	case fraccion >= 0.8:
		estilo = e.Advertencia
	}
	return estilo.Render(strings.Repeat(lleno, n)) + e.Tenue.Render(strings.Repeat(vacio, ancho-n))
}

// textoUso son las peticiones de la API que conserva el historial, como en la Web UI: cuántas, cuántas con error y la última.
func (p *Pagina) textoUso(nombre string) string {
	u, ok := p.datos.Uso[nombre]
	if !ok {
		return p.tx("puntos.sinUso")
	}
	periodo := p.estilos.Periodo(p.lengua, p.horas)
	partes := []string{p.tx("puntos.uso", "n", strconv.Itoa(u.Peticiones), "periodo", periodo)}
	if u.Peticiones == 1 {
		partes[0] = p.tx("puntos.usoUno", "periodo", periodo)
	}
	if u.Errores > 0 {
		partes = append(partes, p.tx("puntos.usoErrores", "n", strconv.Itoa(u.Errores)))
	}
	if u.Ultima != nil {
		partes = append(partes, p.tx("puntos.usoUltima", "cuando", vivo.Hace(p.ahora(), *u.Ultima)))
	}
	return strings.Join(partes, " "+p.estilos.Iconos.Separador+" ")
}

// vistaDetalle es la cabecera de la API (nombre, o «API lista» recién creada, estado y grupo) y su detalle en el visor.
func (p *Pagina) vistaDetalle() string {
	e := p.estilos
	x, _ := p.elegida()
	titulo := x.Nombre
	if p.creada {
		titulo = p.tx("puntos.listo")
	}
	est, estilo := p.tx("puntos.activo"), e.Exito
	switch {
	case !activo(x):
		est, estilo = p.tx("puntos.desactivado"), e.Tenue
	case x.Error != "":
		estilo = e.Error
	}
	cab := e.Titulo.Render(titulo) + " " + estilo.Render(e.Iconos.Separador+" "+est)
	if x.Grupo != "" {
		cab += e.Tenue.Render(" " + e.Iconos.Separador + " " + x.Grupo)
	}
	return p.conVisor([]string{e.Recortar(cab, p.c.Ancho), ""}, p.textoDetalle(x))
}

// urls son las de cada contrato que acepta la API, como GuiaUrls de la Web UI.
func (p *Pagina) urls(base string, acepta []string) []string {
	e := p.estilos
	var l []string
	for _, c := range []struct{ id, clave, sufijo string }{{"openai", "conectar.urlOpenai", "/openai/v1"},
		{"anthropic", "conectar.urlAnthropic", "/anthropic"}, {"nativa", "conectar.apiPropia", "/api/v1"}} {
		for _, a := range acepta {
			if a == c.id {
				l = append(l, e.Tenue.Render(p.tx(c.clave)), "  "+base+c.sufijo)
			}
		}
	}
	return l
}

// textoDetalle es lo que enseña DetalleApi de la Web UI: cómo conectarla (sus URL y su clave, que solo se ve recién creada
// o regenerada), su uso y el consumo frente a su presupuesto.
func (p *Pagina) textoDetalle(x contrato.EstadoPunto) string {
	e := p.estilos
	l := []string{e.Titulo.Render(p.tx("puntos.conectar")), p.tx("puntos.conectarTexto"), ""}
	if x.URL != "" {
		l = append(l, p.urls(x.URL, x.Contratos)...)
		l = append(l, "", e.Tenue.Render(p.tx("conectar.clave")))
		if p.claveVista != "" {
			l = append(l, "  "+p.claveVista, e.Advertencia.Render(p.tx("puntos.claveUnaVez")))
		} else {
			l = append(l, p.tx("puntos.claveOculta"))
		}
	} else {
		l = append(l, p.tx("puntos.activaParaConectar"))
	}
	if x.Error != "" && activo(x) {
		l = append(l, "", e.Error.Render(x.Error))
	}
	if m := idModelo(x); m != "" {
		l = append(l, "", e.Tenue.Render(p.tx("receta.modelo")+": ")+m)
	}
	l = append(l, "", p.textoUso(x.Nombre))
	if est, ok := p.datos.Presupuestos[x.Nombre]; ok {
		l = append(append(l, ""), p.lineasBarras(est)...)
	}
	return p.estilos.A(strings.Join(l, "\n"))
}

// textoClave es la pestaña de la clave del espacio: para qué sirve, el estado de la clave (o la clave misma, recién
// creada o regenerada), sus URL y el presupuesto del espacio, si tiene. La clave va arriba, para que se vea sin desplazar
// el visor: solo se ve esta vez.
func (p *Pagina) textoClave() string {
	e := p.estilos
	base := p.resumen.Base
	if base == "" && p.c.Cli != nil {
		base = p.c.Cli.Base
	}
	l := []string{e.Titulo.Render(strings.TrimSpace(p.tx("conectar.directa", "nombre", p.nombre))), p.tx("conectar.directaTexto"), "",
		e.Tenue.Render(p.tx("conectar.clave"))}
	switch {
	case p.claveEspacio != "":
		l = append(l, "  "+p.claveEspacio, e.Advertencia.Render(p.tx("puntos.claveUnaVez")))
	case p.errClave != "":
		l = append(l, e.Error.Render(p.tx("comun.errorCarga")+": "+p.errClave), e.Tenue.Render("r "+p.tx("comun.reintentar")))
	case p.estadoClave.Origen == "variable":
		l = append(l, p.tx("conectar.claveVariable", "variable", p.estadoClave.Variable))
	case p.estadoClave.TieneClave:
		l = append(l, p.tx("conectar.conClave"))
	default:
		l = append(l, p.tx("conectar.sinClave"))
	}
	l = append(l, "", e.Titulo.Render(p.tx("guia.titulo")))
	l = append(l, p.urls(base, contratos)...)
	if grupos := p.gruposModelo(); len(grupos) > 0 { // qué poner en model, como los chips de la Web UI
		l = append(l, "", e.Tenue.Render(p.tx("conectar.modeloAntes")+" model "+p.tx("conectar.modeloDespues")))
		for _, g := range grupos {
			l = append(l, e.Tenue.Render(g.titulo+":")+" "+strings.Join(g.ids, "  "))
		}
	}
	if conLimites(p.espacio) {
		l = append(l, "", e.Titulo.Render(p.tx("presupuesto.espacio")), p.tx("presupuesto.espacioTexto"))
		l = append(l, p.lineasBarras(p.espacio)...)
	}
	return e.A(strings.Join(l, "\n"))
}

// grupoModelo son los ids del catálogo de un grupo: los perfiles o los de un agente.
type grupoModelo struct {
	titulo string
	ids    []string
}

// gruposModelo agrupa los valores de model como gruposModelo de la Web UI: primero los perfiles y luego cada agente en el
// orden de sus fichas y, al final y por nombre, los registrados sin ficha (el mismo orden que da el servicio a la Web UI
// en /api/v1/agentes), cada uno con sus ids agente/modelo.
func (p *Pagina) gruposModelo() []grupoModelo {
	var orden []string
	for _, f := range catalogo.Todos() {
		orden = append(orden, f.Nombre)
	}
	var otros []string
	for _, m := range p.catalogo {
		if m.Agente != "" && !slices.Contains(orden, m.Agente) && !slices.Contains(otros, m.Agente) {
			otros = append(otros, m.Agente)
		}
	}
	slices.Sort(otros)
	de := func(prefijo string) []string {
		var ids []string
		for _, m := range p.catalogo {
			if strings.HasPrefix(m.ID, prefijo) {
				ids = append(ids, m.ID)
			}
		}
		return ids
	}
	var r []grupoModelo
	if ids := de("perfil/"); len(ids) > 0 {
		r = append(r, grupoModelo{p.tx("conectar.perfiles"), ids})
	}
	for _, a := range append(orden, otros...) {
		if ids := de(a + "/"); len(ids) > 0 {
			r = append(r, grupoModelo{catalogo.NombreVisible(a), ids})
		}
	}
	return r
}

// conLimites dice si el presupuesto tiene algún límite: sin ninguno, el espacio no tiene presupuesto.
func conLimites(e nucleo.EstadoPresupuesto) bool {
	return e.Diario.Limites != (nucleo.Limites{}) || e.Mensual.Limites != (nucleo.Limites{})
}

// barra es una de BarrasPresupuesto.vue: un límite definido, con su título, «usado / límite» y la fracción usada.
type barra struct {
	titulo, texto string
	fraccion      float64
}

// barras son las de BarrasPresupuesto.vue, con los mismos campos y cuentas: una por cada límite mayor que cero, del día y
// del mes, en peticiones, tokens y USD.
func barras(lengua string, e nucleo.EstadoPresupuesto) []barra {
	var r []barra
	for _, per := range []struct {
		clave string
		p     nucleo.EstadoPeriodo
	}{{"presupuesto.diario", e.Diario}, {"presupuesto.mensual", e.Mensual}} {
		for _, u := range []struct {
			clave         string
			limite, usado float64
			usd           bool
		}{{"presupuesto.peticiones", float64(per.p.Limites.Peticiones), float64(per.p.Consumo.Peticiones), false},
			{"presupuesto.tokens", float64(per.p.Limites.Tokens), float64(per.p.Consumo.Tokens), false},
			{"presupuesto.usd", per.p.Limites.USD, per.p.Consumo.USD, true}} {
			if u.limite <= 0 {
				continue
			}
			r = append(r, barra{titulo: datos.Texto(lengua, per.clave) + " · " + datos.Texto(lengua, u.clave),
				texto: numero(lengua, u.usado, u.usd) + " / " + numero(lengua, u.limite, u.usd), fraccion: u.usado / u.limite})
		}
	}
	return r
}

// numero es como lo escribe la Web UI: los USD con dos decimales y $ delante; el resto como toLocaleString (pagina.Numero).
func numero(lengua string, n float64, usd bool) string {
	if usd {
		return "$" + pagina.Decimal(lengua, n, 2)
	}
	return pagina.Numero(lengua, int(math.Round(n)))
}

// fecha es la del día como toLocaleDateString: d/m/aaaa en español y m/d/aaaa en inglés.
func fecha(lengua string, t time.Time) string {
	t = t.Local()
	if lengua == "es" {
		return fmt.Sprintf("%d/%d/%d", t.Day(), int(t.Month()), t.Year())
	}
	return fmt.Sprintf("%d/%d/%d", int(t.Month()), t.Day(), t.Year())
}

// lineasBarras pinta las barras: verde, amarilla desde el 80 % y roja al 100 %, como la Web UI; sin color se leen por el
// texto y lo lleno de la barra. Al final, cuándo se reinicia cada periodo.
func (p *Pagina) lineasBarras(est nucleo.EstadoPresupuesto) []string {
	e := p.estilos
	ancho := max(min(p.c.Ancho, 40), 10)
	var l []string
	for _, b := range barras(p.lengua, est) {
		titulo := e.Recortar(b.titulo, max(ancho-lipgloss.Width(b.texto)-1, 1))
		l = append(l, pagina.Rellenar(titulo, ancho-lipgloss.Width(b.texto))+b.texto, p.barra(b.fraccion, ancho))
	}
	return append(l, e.Tenue.Render(p.tx("presupuesto.reinicios", "dia", fecha(p.lengua, est.Diario.Reinicio), "mes", fecha(p.lengua, est.Mensual.Reinicio))))
}

// vistaForm es el título del formulario, una línea que explica para qué es y los campos, desplazados para que el que tiene
// el foco se vea siempre.
func (p *Pagina) vistaForm() string {
	e, f := p.estilos, p.form
	titulo, texto := idioma.T("Filtros y orden"), ""
	switch f.modo {
	case modoNueva:
		titulo, texto = p.tx("puntos.nuevo"), p.tx("puntos.nuevoTexto")
	case modoAjustes:
		titulo, texto = p.tx("puntos.ajustesDe", "nombre", f.nombre), p.tx("presupuesto.editorTexto")
	}
	cab := []string{e.Titulo.Render(e.Recortar(titulo, p.c.Ancho))}
	if texto != "" && p.c.Alto >= 14 {
		cab = append(cab, e.Tenue.Render(e.Recortar(texto, p.c.Ancho)))
	}
	if f.modo != modoFiltros && p.c.Alto >= 12 { // la receta entera no cabe aquí: dónde se edita lo demás
		for _, l := range e.Envolver(e.A(idioma.T("El respaldo, la respuesta con forma y las correcciones se editan en Probar: p desde el detalle.")), p.c.Ancho, 2) {
			cab = append(cab, e.Tenue.Render(l))
		}
	}
	cab = append(cab, "")
	cuerpo := strings.Split(f.f.Vista(), "\n")
	alto := max(p.c.Alto-len(cab), 1)
	foco := f.f.LineaFoco()
	if foco < f.desde {
		f.desde = foco
	}
	if foco+3 > f.desde+alto { // la etiqueta, el campo y su error
		f.desde = foco + 3 - alto
	}
	f.desde = max(min(f.desde, len(cuerpo)-alto), 0)
	return strings.Join(append(cab, cuerpo[f.desde:min(f.desde+alto, len(cuerpo))]...), "\n")
}
