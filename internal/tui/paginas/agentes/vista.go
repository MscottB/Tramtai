package agentes

import (
	"cmp"
	"regexp"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"

	"tramtai/internal/adaptadores"
	catalogo "tramtai/internal/agentes"
	"tramtai/internal/idioma"
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

// estadoDe clasifica un agente por su conexión y el código de su problema.
func estadoDe(a adaptador) estado {
	switch {
	case a.Desconectado:
		return estDesconectado
	case a.Problema == "" || a.ProblemaCodigo == adaptadores.ProblemaAviso:
		return estListo // un aviso, como el AGENTS.md de Codex, no impide usarlo
	}
	switch a.ProblemaCodigo {
	case adaptadores.ProblemaNoInstalado:
		return estNoInstalado
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

// etiqueta es la del estado en la tarjeta de la Web UI (agente.listo, agente.faltaClave…): «Sin conectar» vale para el
// que falta la clave y el que no tiene sesión, y «Revisar» para el resto de problemas.
func etiqueta(e estado) string {
	switch e {
	case estListo:
		return idioma.T("Listo")
	case estDesconectado:
		return idioma.T("Desconectado")
	case estNoInstalado:
		return idioma.T("No instalado")
	case estFaltaClave, estSinSesion:
		return idioma.T("Sin conectar")
	}
	return idioma.T("Revisar")
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
	return strings.ToUpper(string([]rune(agente)[:min(2, len([]rune(agente)))]))
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

func autenticacion(a adaptador) string {
	if a.Autenticacion == adaptadores.AutenticacionSuscripcion {
		return idioma.T("Suscripción (uso personal)")
	}
	return idioma.T("Clave de API")
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
		resto = append(resto, autenticacion(a))
	}
	if a.Version != "" {
		resto = append(resto, a.Version)
	}
	return rellenar(codigo(a.Agente), 3) + " " + rellenar(e.Recortar(nombre(a.Agente), 16), 16) + " " +
		rellenar(icono+" "+etiqueta(est), 16) + strings.Join(resto, " "+e.Iconos.Separador+" ")
}

// Vista es el título, el resumen por estados y la lista; con un detalle abierto, el detalle.
func (p *Pagina) Vista() string {
	e, ancho, alto := p.estilos, p.c.Ancho, p.c.Alto
	if e == nil {
		return ""
	}
	if p.detalle != "" {
		a, _ := p.elegido()
		cabecera := e.Titulo.Render(e.Recortar(nombre(a.Agente), ancho)) + " " + e.Tenue.Render(e.Iconos.Separador+" "+etiqueta(estadoDe(a)))
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
		lineas = append(lineas, e.Tenue.Render(e.Recortar(idioma.T("Los asistentes de IA que Tramtai usa para responder. Conecta al menos uno."), ancho)))
	}
	if p.err != "" {
		lineas = append(lineas, e.Error.Render(e.Recortar(p.err, ancho)))
	}
	if p.cargado {
		lineas = append(lineas, e.Tenue.Render(e.Recortar(p.resumen(), ancho)), "")
		if p.cargando {
			lineas[len(lineas)-1] = e.Tenue.Render(idioma.T("Cargando…"))
		}
		lineas = append(lineas, p.lista.Vista(ancho, max(alto-len(lineas), 1)))
	}
	return strings.Join(lineas, "\n")
}

// resumen cuenta los agentes por estado, con las etiquetas de los filtros de la Web UI.
func (p *Pagina) resumen() string {
	var listos, conectar, instalar int
	for _, a := range p.agentes {
		switch est := estadoDe(a); {
		case est == estListo:
			listos++
		case est == estNoInstalado:
			instalar++
		default:
			conectar++
		}
	}
	return strings.Join([]string{idioma.Tf("Listos (%d)", listos), idioma.Tf("Falta conectar (%d)", conectar), idioma.Tf("No instalados (%d)", instalar)},
		" "+p.estilos.Iconos.Separador+" ")
}

func si(v bool) string {
	if v {
		return idioma.T("Sí")
	}
	return idioma.T("No")
}

// textoDetalle es lo que se ve al abrir un agente: qué le pasa y qué hacer, sus datos técnicos, su catálogo de modelos
// y, si falta, cómo instalarlo.
func (p *Pagina) textoDetalle(a adaptador) string {
	est := estadoDe(a)
	cli := programa(a.Agente)
	admite := catalogo.AdmiteSuscripcion(a.Agente)
	var l []string
	add := func(s ...string) { l = append(l, s...) }
	if cli != nombre(a.Agente) {
		add(cli, "")
	}
	switch est {
	case estDesconectado:
		add(idioma.T("Desconectado: Tramtai no lo usa, ni como respaldo, hasta que lo vuelvas a conectar. Su CLI sigue instalado."))
	case estListo:
		if a.Autenticacion == adaptadores.AutenticacionSuscripcion {
			add(idioma.T("Listo · usa tu suscripción"))
			if a.Cuenta != "" {
				add(idioma.Tf("Cuenta: %s", a.Cuenta))
			}
		} else {
			add(idioma.T("Listo · usa tu clave de API"))
		}
		if a.Problema != "" {
			add(a.Problema)
		}
	case estFaltaClave:
		if admite {
			add(idioma.T("Elige cómo conectarlo: con tu suscripción (tecla a) o con una clave de API."))
			add(idioma.Tf("Usa la sesión que ya tienes iniciada en %s. Solo para uso personal en este equipo, según los términos del proveedor.", cli))
		} else {
			add(idioma.T("Necesita una clave de API."))
		}
		add(idioma.T("Con una clave de API: ejecuta esto en una terminal (pide la clave sin mostrarla) y pulsa c para volver a comprobar:"), "  "+comandoClave(a))
	case estSinSesion:
		add(a.Problema)
	case estNoInstalado:
		add(idioma.Tf("Instala su CLI y pulsa c para volver a comprobar. Si está en otra ruta, indícala en comandos_agentes.%s.", a.Agente))
		if f, ok := catalogo.Buscar(a.Agente); ok && f.URLInstalacion != "" {
			add(idioma.Tf("Cómo instalarlo: %s", f.URLInstalacion))
		}
	case estUsaClave:
		add(idioma.Tf("%s usa ahora una clave de API, no tu suscripción. Deja de usar la suscripción aquí, o vuelve a iniciar sesión en %s.", cli, cli))
	case estSesionNoResponde:
		add(idioma.Tf("La sesión de %s no responde: la última petición falló por autenticación. Inicia sesión de nuevo en %s o deja de usar la suscripción.", cli, cli))
	default:
		add(a.Problema)
	}
	if est != estNoInstalado {
		add("")
		sinVersion := idioma.T("no detectada")
		fila := func(k, v string) { add(k + ": " + v) }
		fila(idioma.T("Versión"), cmp.Or(a.Version, sinVersion))
		fila(idioma.T("Autenticación"), autenticacion(a))
		enVivo := idioma.T("Al terminar")
		if a.Capacidades.Streaming == "tokens" {
			enVivo = idioma.T("Por fragmentos")
		}
		fila(idioma.T("Respuesta en vivo"), enVivo)
		fila(idioma.T("Respuesta JSON con esquema"), si(a.Capacidades.SalidaEstructurada))
		fila(idioma.T("Informa el modelo usado"), si(a.Capacidades.ModeloEfectivoInformado))
		esfuerzos := idioma.T("No")
		if n := catalogo.Niveles(a.Agente); len(n) > 0 {
			esfuerzos = strings.Join(n, " · ")
		}
		fila(idioma.T("Esfuerzo elegible"), esfuerzos)
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
			m = idioma.T("predeterminado")
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
