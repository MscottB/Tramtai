package agentes

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"tramtai/internal/adaptadores"
	catalogo "tramtai/internal/agentes"
	"tramtai/internal/idioma"
	"tramtai/internal/tui/datos"
)

// estado es lo que enseña un agente: el mismo cálculo que estadoAgente de la Web UI (web/src/agentes.ts).
type estado int

const (
	estListo estado = iota
	estDesconectado
	estNoInstalado
	estFaltaClave // con clave de API: falta la clave
	estSinSesion  // con suscripción: su CLI no tiene sesión
	estUsaClave
	estSesionNoResponde
	estRevisar
)

// estadoDe clasifica un agente por su conexión y el código de su problema. Uno cuyo CLI ya no está es «no instalado»
// aunque esté desconectado: la Web UI lo pone entre los que se pueden instalar.
func estadoDe(a adaptador) estado {
	switch {
	case a.ProblemaCodigo == adaptadores.ProblemaNoInstalado:
		return estNoInstalado
	case a.Desconectado:
		return estDesconectado
	case a.Problema == "" || a.ProblemaCodigo == adaptadores.ProblemaAviso:
		return estListo // un aviso, como el AGENTS.md de Codex, no impide usarlo
	}
	switch a.ProblemaCodigo {
	case adaptadores.ProblemaFaltaClave:
		if a.Autenticacion == adaptadores.AutenticacionSuscripcion {
			return estSinSesion
		}
		return estFaltaClave
	case adaptadores.ProblemaUsaClave:
		return estUsaClave
	case adaptadores.ProblemaSesion:
		return estSesionNoResponde
	}
	return estRevisar
}

// tx es un texto de la Web UI (datos.Texto) en el idioma de la sesión, con sus {marcadores} rellenos.
func (p *Pagina) tx(clave string, pares ...string) string {
	return datos.Textof(p.lengua, clave, pares...)
}

// etiqueta es la del estado en la tarjeta de la Web UI (agente.listo, agente.faltaClave…): «Sin conectar» vale para el
// que falta la clave y el que no tiene sesión, y «Revisar» para el resto de problemas.
func (p *Pagina) etiqueta(e estado) string {
	switch e {
	case estListo:
		return p.tx("agente.listo")
	case estDesconectado:
		return p.tx("agente.desconectado")
	case estNoInstalado:
		return p.tx("agente.noInstalado")
	case estFaltaClave, estSinSesion:
		return p.tx("agente.faltaClave")
	}
	return p.tx("agente.revisar")
}

// nombre es el nombre visible del agente (el de su ficha); el id si no tiene.
func nombre(agente string) string { return catalogo.NombreVisible(agente) }

// programa es el CLI o servicio que hay detrás (Claude Code); el id si no tiene ficha.
func programa(agente string) string {
	if f, ok := catalogo.Buscar(agente); ok && f.Programa != "" {
		return f.Programa
	}
	return agente
}

func codigo(agente string) string {
	if f, ok := catalogo.Buscar(agente); ok && f.Codigo != "" {
		return f.Codigo
	}
	letras := []rune(agente)
	return strings.ToUpper(string(letras[:min(2, len(letras))]))
}

// ordenar pone primero los listos, luego los que falta conectar y al final los no instalados; dentro de cada grupo, el
// orden de sus fichas y, los que no la tienen, por nombre.
func ordenar(lista []adaptador) []adaptador {
	rango := func(a adaptador) int {
		switch e := estadoDe(a); {
		case e == estNoInstalado:
			return 2
		case e == estListo:
			return 0
		}
		return 1
	}
	posicion := func(agente string) int {
		todos := catalogo.Todos()
		if i := slices.IndexFunc(todos, func(f adaptadores.Ficha) bool { return f.Nombre == agente }); i >= 0 {
			return i
		}
		return len(todos)
	}
	lista = slices.Clone(lista)
	slices.SortStableFunc(lista, func(a, b adaptador) int {
		return cmp.Or(rango(a)-rango(b), posicion(a.Agente)-posicion(b.Agente), strings.Compare(a.Agente, b.Agente))
	})
	return lista
}

func rellenar(texto string, ancho int) string {
	return texto + strings.Repeat(" ", max(ancho-lipgloss.Width(texto), 0))
}

func (p *Pagina) autenticacion(a adaptador) string {
	if a.Autenticacion == adaptadores.AutenticacionSuscripcion {
		return p.tx("agentes.suscripcion")
	}
	return p.tx("agentes.claveApi")
}

// fila es una línea de la lista: su código, su nombre, su estado y, si está listo, cómo está conectado.
func (p *Pagina) fila(a adaptador) string {
	e := p.estilos
	est := estadoDe(a)
	icono := "!"
	switch est {
	case estListo:
		icono = e.Iconos.Ok
	case estDesconectado, estNoInstalado:
		icono = e.Iconos.Inactivo
	}
	var resto []string
	if est == estListo {
		resto = append(resto, p.autenticacion(a))
	}
	if a.Version != "" {
		resto = append(resto, a.Version)
	}
	return rellenar(codigo(a.Agente), 3) + " " + rellenar(e.Recortar(nombre(a.Agente), 16), 16) + " " +
		rellenar(icono+" "+p.etiqueta(est), 16) + strings.Join(resto, " "+e.Iconos.Separador+" ")
}

// Vista es el título, el resumen por estados y la lista; con un detalle abierto, el detalle.
func (p *Pagina) Vista() string {
	e, ancho, alto := p.estilos, p.c.Ancho, p.c.Alto
	if e == nil {
		return ""
	}
	if p.detalle != "" {
		a, _ := p.elegido()
		cabecera := e.Titulo.Render(e.Recortar(nombre(a.Agente), ancho)) + " " + e.Tenue.Render(e.Iconos.Separador+" "+p.etiqueta(estadoDe(a)))
		return cabecera + "\n\n" + p.visor.Vista()
	}
	lineas := []string{e.Titulo.Render(e.Recortar(p.Titulo(), ancho))}
	switch {
	case p.sinServ:
		return strings.Join(append(lineas, "", e.Advertencia.Render(e.Recortar(idioma.T("El servicio no responde · r reintenta"), ancho))), "\n")
	case !p.cargado && p.err == "":
		return strings.Join(append(lineas, "", e.Tenue.Render(idioma.T("Cargando…"))), "\n")
	}
	if alto >= 12 {
		lineas = append(lineas, e.Tenue.Render(e.Recortar(p.tx("agentes.subtitulo"), ancho)))
	}
	if p.err != "" {
		lineas = append(lineas, e.Error.Render(e.Recortar(p.err, ancho)))
	}
	if p.cargado {
		lineas = append(lineas, e.Recortar(p.resumen(), ancho), "")
		if p.cargando {
			lineas[len(lineas)-1] = e.Tenue.Render(idioma.T("Cargando…"))
		}
		lineas = append(lineas, p.lista.Vista(ancho, max(alto-len(lineas), 1)))
	}
	return strings.Join(lineas, "\n")
}

// Los filtros de la Web UI (opcionesFiltro de Agentes.vue), en su orden: todos, listos, falta conectar y no instalados.
var filtros = []struct {
	clave string
	deja  func(estado) bool
}{
	{"agentes.filtroTodos", func(estado) bool { return true }},
	{"agentes.filtroListos", func(e estado) bool { return e == estListo }},
	{"agentes.filtroConectar", func(e estado) bool { return e != estListo && e != estNoInstalado }},
	{"agentes.filtroInstalar", func(e estado) bool { return e == estNoInstalado }},
}

// resumen son los filtros con cuántos agentes deja cada uno; el elegido va entre corchetes, como las pestañas.
func (p *Pagina) resumen() string {
	partes := make([]string, len(filtros))
	for i, f := range filtros {
		n := 0
		for _, a := range p.agentes {
			if f.deja(estadoDe(a)) {
				n++
			}
		}
		texto := p.tx(f.clave, "n", strconv.Itoa(n))
		if i == p.filtro {
			partes[i] = p.estilos.Titulo.Render("[" + texto + "]")
		} else {
			partes[i] = p.estilos.Tenue.Render(texto)
		}
	}
	return strings.Join(partes, " ")
}

// filtrar cambia de filtro (paso 1 o -1, dando la vuelta) y deja en la lista solo esos agentes.
func (p *Pagina) filtrar(paso int) {
	p.filtro = (p.filtro + paso + len(filtros)) % len(filtros)
	deja := filtros[p.filtro].deja
	p.lista.Filtrar(func(a adaptador) bool { return deja(estadoDe(a)) })
}

func (p *Pagina) si(v bool) string {
	if v {
		return p.tx("comun.si")
	}
	return p.tx("comun.no")
}

// textoDetalle es lo que se ve al abrir un agente: qué le pasa y qué hacer, sus datos técnicos, su catálogo de modelos
// y, si falta, cómo instalarlo.
func (p *Pagina) textoDetalle(a adaptador) string {
	est := estadoDe(a)
	cli := programa(a.Agente)
	var l []string
	add := func(s ...string) { l = append(l, s...) }
	if cli != nombre(a.Agente) {
		add(cli, "")
	}
	switch est {
	case estDesconectado:
		add(p.tx("agentes.desconectadoTexto"))
	case estListo:
		if a.Autenticacion == adaptadores.AutenticacionSuscripcion {
			add(p.tx("agentes.listoSuscripcion"))
			if a.Cuenta != "" {
				add(p.tx("agentes.cuenta", "cuenta", a.Cuenta))
			}
		} else {
			add(p.tx("agentes.listoClave"))
		}
		if a.Problema != "" {
			add(a.Problema)
		}
	case estFaltaClave:
		if catalogo.AdmiteSuscripcion(a.Agente) {
			add(idioma.T("Elige cómo conectarlo: con tu suscripción (tecla a) o con una clave de API."), p.tx("agentes.opcionSuscripcionTexto", "cli", cli),
				idioma.T("Para la clave de API, ejecuta esto en una terminal (pide la clave sin mostrarla) y pulsa c para volver a comprobar:"))
		} else {
			add(idioma.T("Necesita una clave de API. Ejecuta esto en una terminal (pide la clave sin mostrarla) y pulsa c para volver a comprobar:"))
		}
		add("  " + comandoClave(a))
	case estSinSesion:
		add(a.Problema)
	case estNoInstalado:
		add(idioma.Tf("Instala su CLI y pulsa c para volver a comprobar. Si está en otra ruta, indícala en comandos_agentes.%s.", a.Agente))
		if f, ok := catalogo.Buscar(a.Agente); ok && f.URLInstalacion != "" {
			add(p.tx("agentes.comoInstalar") + ": " + f.URLInstalacion)
		}
	case estUsaClave:
		add(p.tx("agentes.usaClave", "cli", cli))
	case estSesionNoResponde:
		add(p.tx("agentes.sesionNoResponde", "cli", cli))
	default:
		add(a.Problema)
	}
	if est != estNoInstalado {
		add("")
		fila := func(k, v string) { add(k + ": " + v) }
		fila(p.tx("agentes.version"), cmp.Or(a.Version, p.tx("agentes.sinVersion")))
		fila(p.tx("agentes.autenticacion"), p.autenticacion(a))
		enVivo := p.tx("agentes.alTerminar")
		if a.Capacidades.Streaming == "tokens" {
			enVivo = p.tx("agentes.fragmentos")
		}
		fila(p.tx("agentes.enVivo"), enVivo)
		fila(p.tx("agentes.json"), p.si(a.Capacidades.SalidaEstructurada))
		fila(p.tx("agentes.informaModelo"), p.si(a.Capacidades.ModeloEfectivoInformado))
		esfuerzos := p.tx("comun.no")
		if n := catalogo.Niveles(a.Agente); len(n) > 0 {
			esfuerzos = strings.Join(n, " · ")
		}
		fila(p.tx("agentes.esfuerzo"), esfuerzos)
		fila(idioma.T("Modelos"), p.textoModelos(a.Agente))
		if a.Terminos != "" {
			add("", a.Terminos)
		}
	}
	return strings.Join(l, "\n")
}

// textoModelos es el catálogo del agente: el predeterminado de su CLI y los demás, sin repetir el nombre del agente.
func (p *Pagina) textoModelos(agente string) string {
	if err := p.errModel[agente]; err != "" {
		return err
	}
	ids, ya := p.modelos[agente]
	if !ya {
		return idioma.T("Cargando…")
	}
	var nombres []string
	for _, id := range ids {
		m := strings.TrimPrefix(id, agente+"/")
		if id == agente {
			m = p.tx("agentes.predeterminadoCli")
		}
		nombres = append(nombres, m)
	}
	if len(nombres) == 0 {
		return "—"
	}
	return strings.Join(nombres, " · ")
}

// comandoClave es el comando que guarda la clave del agente, con la primera variable de su ficha.
func comandoClave(a adaptador) string {
	var variable string
	if f, ok := catalogo.Buscar(a.Agente); ok && len(f.VariablesClave) > 0 {
		variable = f.VariablesClave[0]
	}
	if variable == "" { // como la Web UI: la variable que nombra el problema
		variable = regexp.MustCompile(`[A-Z_]+_API_KEY`).FindString(a.Problema)
	}
	return "tramtai claves guardar " + variable
}
