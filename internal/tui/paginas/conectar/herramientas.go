package conectar

import (
	"maps"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
)

// La pestaña Herramientas de Mis APIs, a la par de web/src/componentes/accesos/Herramientas.vue y web/src/herramientas.ts:
// Claude Code, Codex, Continue, opencode y Kilo apuntadas a Tramtai desde el espacio. Los textos son sus claves herramientas.*.

const (
	accOlvidarHerr   = "olvidarHerramienta"
	accRestaurarHerr = "restaurarHerramienta"
)

// herramientas es el estado de la pestaña.
type herramientas struct {
	lista    comp.Lista[contrato.EstadoHerramienta]
	filas    []contrato.EstadoHerramienta
	fichas   []cliente.FichaAgente // para el modelo: qué agentes están conectados y valen en APIs
	n        int                   // la última carga pedida: solo vale su respuesta
	cargada  bool
	servidor bool   // el servicio está en modo servidor: no hay herramientas
	err      string // por qué no llegó la lista
	sel      *comp.Selector
	quien    quienModelo
}

// quienModelo es a quién se aplica el modelo que se está eligiendo.
type quienModelo struct {
	ids   []string
	todas bool
}

// msgHerramientas trae la lista de herramientas del espacio y las fichas de los agentes.
type msgHerramientas struct {
	origen
	n      int
	filas  []contrato.EstadoHerramienta
	fichas []cliente.FichaAgente
	err    error
}

// Lo que decide cada fila, como estaPuesta, puedeCambiar y puedeActivarTodas de herramientas.ts.
func estaPuesta(h contrato.EstadoHerramienta) bool {
	return h.Estado == "activa" || h.Estado == "desactualizada"
}
func puedeCambiar(h contrato.EstadoHerramienta) bool {
	return h.Estado != "solo_global" && h.Estado != "no_detectada"
}
func puedeActivarTodas(l []contrato.EstadoHerramienta) bool {
	return slices.ContainsFunc(l, func(h contrato.EstadoHerramienta) bool {
		return h.Estado == "detectada" || h.Estado == "desactivada" || h.Estado == "desactualizada"
	})
}

// cargarHerr pide la lista (y las fichas de los agentes, que no comprueban los CLI): al entrar en la pestaña, con r y tras cada
// cambio, también si falló, como la Web UI.
func (p *Pagina) cargarHerr() tea.Cmd {
	c := p.c
	if c.Cli == nil {
		p.sinServ = true
		return pagina.SinCliente
	}
	p.herr.n++
	m := msgHerramientas{origen: p.origen(), n: p.herr.n}
	return func() tea.Msg {
		m.filas, m.err = c.Cli.Herramientas(c.Ctx, c.Instancia)
		if m.err == nil {
			m.fichas, _ = c.Cli.FichasAgentes(c.Ctx, c.Instancia) // sin fichas no se filtra ni se preselecciona: el servicio elige
		}
		return m
	}
}

func (p *Pagina) alHerramientas(m msgHerramientas) tea.Cmd {
	h := &p.herr
	if !p.vigente(m.origen) || m.n != h.n {
		return nil
	}
	switch {
	case m.err == nil:
		h.cargada, h.servidor, h.err, h.filas, h.fichas = true, false, "", m.filas, m.fichas
	case nucleo.CodigoDe(m.err) == nucleo.CodigoNoDisponibleEnServidor:
		h.cargada, h.servidor, h.err, h.filas = true, true, "", nil
	default:
		if cmd := pagina.SinServicio(m.err); cmd != nil {
			p.sinServ = true
			return cmd
		}
		if len(h.filas) > 0 { // la lista que ya se ve no se tapa
			return comp.Aviso(nucleo.MensajeDe(m.err), comp.Fallo)
		}
		h.cargada, h.err = false, nucleo.MensajeDe(m.err)
		return nil
	}
	h.lista.Poner(h.filas)
	return nil
}

// accionHerr es un cambio en las herramientas: al volver, la página pide la lista otra vez, también si falló.
func (p *Pagina) accionHerr(exito string, hacer func(c pagina.Contexto) error) tea.Cmd {
	return p.accion(exito, func(c pagina.Contexto, m *msgAccion) error {
		m.herr = true
		return hacer(c)
	})
}

func (p *Pagina) activarHerr(q quienModelo, modelo string) tea.Cmd {
	return p.accionHerr(p.tx("herramientas.avisoActivada"), func(c pagina.Contexto) error {
		_, err := c.Cli.ActivarHerramientas(c.Ctx, c.Instancia, contrato.ActivarHerramientas{IDs: q.ids, Todas: q.todas, Modelo: modelo})
		return err
	})
}

func (p *Pagina) desactivarHerr(ids []string, todas bool) tea.Cmd {
	return p.accionHerr(p.tx("herramientas.avisoDesactivada"), func(c pagina.Contexto) error {
		_, err := c.Cli.DesactivarHerramientas(c.Ctx, c.Instancia, ids, todas)
		return err
	})
}

// teclaHerramientas atiende una tecla de la pestaña. Espacio activa (pidiendo el modelo) o desactiva la fila; t y T, todas; m
// cambia el modelo; o olvida; R restaura la copia (r es refrescar, en todas las pestañas); Intro vuelve a activar una
// desactualizada.
func (p *Pagina) teclaHerramientas(t string, k tea.KeyPressMsg) tea.Cmd {
	h := &p.herr
	if !h.cargada || h.servidor {
		return nil
	}
	fila, hay := h.lista.Seleccion()
	libre := !p.ocupado
	switch {
	case t == "space" && hay && libre && puedeCambiar(fila):
		if estaPuesta(fila) {
			return p.desactivarHerr([]string{fila.ID}, false)
		}
		return p.pedirModelo(quienModelo{ids: []string{fila.ID}}, p.tx("herramientas.activarTitulo", "nombre", fila.Nombre), fila.Modelo)
	case t == "enter" && hay && libre && fila.Estado == "desactualizada":
		return p.activarHerr(quienModelo{ids: []string{fila.ID}}, fila.Modelo)
	case t == "t" && libre && puedeActivarTodas(h.filas):
		return p.pedirModelo(quienModelo{todas: true}, p.tx("herramientas.activarTodasTitulo"), "")
	case t == "T" && libre && slices.ContainsFunc(h.filas, estaPuesta):
		return p.desactivarHerr(nil, true)
	case t == "m" && hay && libre && estaPuesta(fila):
		return p.pedirModelo(quienModelo{ids: []string{fila.ID}}, p.tx("herramientas.cambiarTitulo", "nombre", fila.Nombre), fila.Modelo)
	case t == "o" && hay && libre && (estaPuesta(fila) || fila.Estado == "desactivada"):
		return p.preguntar(accOlvidarHerr, fila.ID, p.tx("herramientas.olvidarTitulo", "nombre", fila.Nombre), p.tx("herramientas.olvidarTexto"))
	case t == "R" && hay && libre && fila.HayCopia:
		return p.preguntar(accRestaurarHerr, fila.ID, p.tx("herramientas.restaurarTitulo", "nombre", fila.Nombre), p.textoRestaurar(fila))
	case slices.Contains([]string{"t", "T", "m", "o", "R", "space"}, t):
		return nil
	}
	h.lista.Update(k)
	return nil
}

// textoRestaurar avisa de lo que se pierde y de qué fecha es la copia (sin fecha, sin decirla).
func (p *Pagina) textoRestaurar(h contrato.EstadoHerramienta) string {
	if f, err := time.Parse(time.RFC3339, h.FechaCopia); err == nil {
		return p.tx("herramientas.restaurarTexto", "fecha", fecha(p.lengua, f))
	}
	return p.tx("herramientas.restaurarTextoSinFecha")
}

// pedirModelo abre el selector de agente y modelo (el catálogo, sin los agentes que no valen en APIs) con el modelo actual de
// la herramienta marcado o, si no tiene, el primer agente conectado que vale en APIs.
func (p *Pagina) pedirModelo(q quienModelo, titulo, actual string) tea.Cmd {
	var items []comp.Item
	for _, m := range p.catalogo {
		if m.Agente == "" || p.valeEnApis(m.Agente) {
			items = append(items, comp.Item{Valor: m.ID, Texto: m.ID, Detalle: m.Version})
		}
	}
	if len(items) == 0 {
		return comp.Aviso(p.tx("herramientas.sinAgentes"), comp.Fallo)
	}
	s := comp.NuevoSelector(p.estilos, titulo, items)
	s.Texto = p.tx("herramientas.modeloTexto")
	if actual == "" {
		actual = p.modeloInicial(items)
	}
	s.Marcar(actual)
	p.herr.sel, p.herr.quien = &s, q
	return nil
}

// valeEnApis dice si el agente se puede usar desde APIs; sin su ficha, sí (el servicio lo rechazaría si no).
func (p *Pagina) valeEnApis(agente string) bool {
	i := slices.IndexFunc(p.herr.fichas, func(f cliente.FichaAgente) bool { return f.Agente == agente })
	return i < 0 || p.herr.fichas[i].PermitidoApps
}

// modeloInicial es el primer agente conectado y permitido en APIs, en el orden del servicio, con el modelo de su CLI; sin
// fichas que coincidan, la primera opción que no sea un perfil.
func (p *Pagina) modeloInicial(items []comp.Item) string {
	for _, f := range p.herr.fichas {
		if !f.Desconectado && f.PermitidoApps && slices.ContainsFunc(items, func(i comp.Item) bool { return i.Valor == f.Agente }) {
			return f.Agente
		}
	}
	for _, i := range items {
		if !strings.HasPrefix(i.Valor, "perfil/") {
			return i.Valor
		}
	}
	return ""
}

// elegidoHerr es el modelo elegido en el selector: activa con él lo que se preguntaba.
func (p *Pagina) elegidoHerr(valor string) tea.Cmd {
	if p.herr.sel == nil {
		return nil
	}
	q := p.herr.quien
	p.herr.sel = nil
	return p.activarHerr(q, valor)
}

// palabraEstado es el estado con las palabras de la Web UI.
func (p *Pagina) palabraEstado(estado string) string {
	if clave := "herramientas.estado." + estado; datos.Tiene(p.lengua, clave) {
		return p.tx(clave)
	}
	return estado
}

// textoMotivo es por qué una herramienta no va, en el idioma de la terminal: el código del servicio con sus datos o, si no
// hay texto para ese código, el del servicio.
func (p *Pagina) textoMotivo(h contrato.EstadoHerramienta) string {
	clave := "herramientas.motivo." + h.MotivoCodigo
	if h.MotivoCodigo == "" || !datos.Tiene(p.lengua, clave) {
		return h.Motivo
	}
	var pares []string
	for _, k := range slices.Sorted(maps.Keys(h.MotivoDatos)) {
		pares = append(pares, k, h.MotivoDatos[k])
	}
	return p.tx(clave, pares...)
}

// filaHerr es una herramienta en la lista: su nombre, su estado (símbolo y palabra, nunca solo el color) y su modelo.
func (p *Pagina) filaHerr(h contrato.EstadoHerramienta) string {
	e := p.estilos
	icono, estilo := e.Iconos.Inactivo, e.Tenue
	switch h.Estado {
	case "activa":
		icono, estilo = e.Iconos.Activo, e.Exito
	case "desactualizada":
		icono, estilo = e.Iconos.Fallo, e.Advertencia
	case "detectada":
		estilo = e.Acento
	}
	return pagina.Rellenar(e.Recortar(h.Nombre, 12), 12) + " " + estilo.Render(pagina.Rellenar(icono+" "+p.palabraEstado(h.Estado), 17)) + " " + e.Tenue.Render(h.Modelo)
}

// vistaHerramientas es la pestaña: la lista y, debajo, lo de la fila del cursor (archivo, modelo, motivo y la nota de la clave).
func (p *Pagina) vistaHerramientas(arriba []string) string {
	e, h, ancho, alto := p.estilos, &p.herr, p.c.Ancho, p.c.Alto
	l := arriba
	envolver := func(estilo lipgloss.Style, texto string) {
		for _, linea := range e.Envolver(texto, ancho, 0) {
			l = append(l, estilo.Render(linea))
		}
	}
	switch {
	case h.servidor: // la cuenta del servicio no es la de nadie: las herramientas se apuntan desde cada equipo
		l = append(l, "", e.Titulo.Render(e.Recortar(p.tx("herramientas.servidor"), ancho)))
		envolver(e.Texto, p.tx("herramientas.servidorTexto"))
	case h.err != "":
		l = append(l, "", e.Error.Width(ancho).Render(p.tx("comun.errorCarga")+": "+h.err), e.Tenue.Render(e.Recortar("r "+p.tx("comun.reintentar"), ancho)))
	case !h.cargada:
		l = append(l, "", e.Tenue.Render(p.tx("comun.cargando")))
	case !slices.ContainsFunc(h.filas, func(x contrato.EstadoHerramienta) bool { return x.Estado != "no_detectada" }):
		l = append(l, "", e.Titulo.Render(e.Recortar(p.tx("herramientas.vacio"), ancho)))
		envolver(e.Texto, p.tx("herramientas.vacioTexto"))
	default:
		if alto >= 12 {
			l = append(l, e.Tenue.Render(e.Recortar(p.tx("herramientas.texto"), ancho)))
		}
		l = append(l, h.lista.Vista(ancho, min(len(h.filas), max(alto-len(l)-5, 2))))
		if f, ok := h.lista.Seleccion(); ok {
			l = append(l, "")
			if f.Ruta != "" {
				envolver(e.Tenue, f.Ruta)
			}
			if f.Modelo != "" {
				envolver(e.Tenue, p.tx("herramientas.modelo", "modelo", f.Modelo))
			}
			switch {
			case f.Estado == "solo_global":
				envolver(e.Texto, p.tx("herramientas.soloGlobal"))
			case f.Estado == "no_detectada":
				envolver(e.Tenue, p.tx("herramientas.noDetectadaFila"))
			case p.textoMotivo(f) != "":
				envolver(e.Advertencia, p.textoMotivo(f))
			}
			if f.ClaveEnArchivo && puedeCambiar(f) {
				envolver(e.Tenue, p.tx("herramientas.claveEnArchivo"))
			}
		}
	}
	return recortarAlto(strings.Join(l, "\n"), alto)
}

// teclasHerr son las del pie de la pestaña, según la fila del cursor, como los botones de la Web UI.
func (p *Pagina) teclasHerr() []pagina.Tecla {
	h := &p.herr
	teclas := []pagina.Tecla{{Tecla: p.flechas(), Accion: idioma.T("pestañas")}}
	if h.cargada && !h.servidor {
		f, hay := h.lista.Seleccion()
		if hay && puedeCambiar(f) {
			teclas = append(teclas, pagina.Tecla{Tecla: "Espacio", Accion: p.tx("herramientas.interruptor")})
		}
		if hay && f.Estado == "desactualizada" {
			teclas = append(teclas, pagina.Tecla{Tecla: "Enter", Accion: p.tx("herramientas.volverAActivar")})
		}
		if puedeActivarTodas(h.filas) {
			teclas = append(teclas, pagina.Tecla{Tecla: "t", Accion: p.tx("herramientas.activarTodas")})
		}
		if slices.ContainsFunc(h.filas, estaPuesta) {
			teclas = append(teclas, pagina.Tecla{Tecla: "T", Accion: p.tx("herramientas.desactivarTodas")})
		}
		if hay && estaPuesta(f) {
			teclas = append(teclas, pagina.Tecla{Tecla: "m", Accion: p.tx("herramientas.cambiarModelo")})
		}
		if hay && (estaPuesta(f) || f.Estado == "desactivada") {
			teclas = append(teclas, pagina.Tecla{Tecla: "o", Accion: p.tx("herramientas.olvidar")})
		}
		if hay && f.HayCopia {
			teclas = append(teclas, pagina.Tecla{Tecla: "R", Accion: p.tx("herramientas.restaurar")})
		}
	}
	return append(teclas, pagina.Tecla{Tecla: "r", Accion: p.tx("herramientas.refrescar")})
}
