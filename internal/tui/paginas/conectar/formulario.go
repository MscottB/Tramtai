package conectar

import (
	"cmp"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"

	catalogo "tramtai/internal/agentes"
	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
)

// filtros son los de la lista de la Web UI: la búsqueda, el grupo, el estado, el agente, el contrato, lo que tienen (todo a
// la vez), si tienen peticiones y el orden. En grupo y agente, todos es «sin filtro» ("" es «sin grupo» y «el
// predeterminado»).
type filtros struct {
	busqueda, grupo, estado, agente, contrato, uso, orden string
	tiene                                                 []string
}

const todos = "\x00"

var sinFiltros = filtros{grupo: todos, estado: "todas", agente: todos, uso: "todas", orden: "nombre"}

// hay dice si algún filtro deja fuera APIs; el orden no cuenta.
func (f filtros) hay() bool {
	return f.busqueda != "" || f.grupo != todos || f.estado != "todas" || f.agente != todos || f.contrato != "" || f.uso != "todas" ||
		len(f.tiene) > 0
}

// caracteristicas son las del filtro «Tiene» de la Web UI, en su orden, con su texto.
var caracteristicas = []struct{ id, clave string }{{"json", "puntos.carJson"}, {"respaldo", "puntos.carRespaldo"},
	{"presupuesto", "puntos.carPresupuesto"}, {"instrucciones", "puntos.carInstrucciones"}, {"forzado", "puntos.carForzado"},
	{"restringida", "puntos.carRestringida"}}

// tiene dice si la API tiene esa característica, como caracteristicas en ListaApis.vue.
func (p *Pagina) tiene(x contrato.EstadoPunto, c string) bool {
	switch c {
	case "json":
		return len(x.Esquema) > 0
	case "respaldo":
		return len(x.Respaldo) > 0
	case "presupuesto":
		_, ok := p.datos.Presupuestos[x.Nombre]
		return ok
	case "instrucciones":
		return strings.TrimSpace(x.Instrucciones) != ""
	case "forzado":
		return x.ForzarModelo
	case "restringida":
		return len(x.ModelosPermitidos) > 0
	}
	return false
}

// contratos son los de la Web UI, en su orden.
var contratos = []string{"openai", "anthropic", "nativa"}

func (p *Pagina) nombreContrato(c string) string {
	switch c {
	case "openai":
		return "OpenAI"
	case "anthropic":
		return "Anthropic"
	}
	return p.tx("contrato.nativa")
}

// estado es el del filtro de estado: detenidas, con error o activas.
func estado(x contrato.EstadoPunto) string {
	switch {
	case !activo(x):
		return "detenidas"
	case x.Error != "":
		return "error"
	}
	return "activas"
}

// pasa dice si una API cumple los filtros (sin la búsqueda, que la hace la lista).
func (p *Pagina) pasa(x contrato.EstadoPunto) bool {
	f := p.filtros
	return (f.grupo == todos || x.Grupo == f.grupo) && (f.estado == "todas" || f.estado == estado(x)) &&
		(f.agente == todos || x.Agente == f.agente) && (f.contrato == "" || slices.Contains(x.Contratos, f.contrato)) &&
		!slices.ContainsFunc(f.tiene, func(c string) bool { return !p.tiene(x, c) }) &&
		(f.uso == "todas" || (f.uso == "con") == (p.datos.Uso[x.Nombre].Peticiones > 0))
}

// comparar es el orden elegido: por nombre, las más usadas o la última petición, y a igualdad por nombre.
func (p *Pagina) comparar(a, b contrato.EstadoPunto) int {
	ua, ub := p.datos.Uso[a.Nombre], p.datos.Uso[b.Nombre]
	ultima := func(t *time.Time) time.Time {
		if t == nil {
			return time.Time{}
		}
		return *t
	}
	switch p.filtros.orden {
	case "uso":
		return cmp.Or(ub.Peticiones-ua.Peticiones, strings.Compare(a.Nombre, b.Nombre))
	case "ultima":
		return cmp.Or(ultima(ub.Ultima).Compare(ultima(ua.Ultima)), strings.Compare(a.Nombre, b.Nombre))
	}
	return strings.Compare(a.Nombre, b.Nombre)
}

func hayGrupos(l []contrato.EstadoPunto) bool {
	return slices.ContainsFunc(l, func(x contrato.EstadoPunto) bool { return x.Grupo != "" })
}

// aplicarFiltros pone las APIs en la lista con los filtros, el orden y, si alguna tiene grupo, los grupos (los que no
// tienen, al final).
func (p *Pagina) aplicarFiltros() {
	if p.estilos == nil {
		return
	}
	texto := p.filtros.busqueda
	if p.buscando {
		texto = p.entrada
	}
	grupos := hayGrupos(p.datos.Puntos)
	p.lista.Buscar(texto)
	p.lista.Filtrar(p.pasa)
	p.lista.Ordenar(func(a, b contrato.EstadoPunto) int {
		if grupos && a.Grupo != b.Grupo {
			switch {
			case a.Grupo == "":
				return 1
			case b.Grupo == "":
				return -1
			}
			return strings.Compare(a.Grupo, b.Grupo)
		}
		return p.comparar(a, b)
	})
	if grupos {
		p.lista.Agrupar(p.grupoDe)
	} else {
		p.lista.Agrupar(nil)
	}
	p.lista.Poner(p.datos.Puntos)
}

// ninguna dice si los filtros y la búsqueda no dejan ninguna API.
func (p *Pagina) ninguna() bool {
	texto := nucleo.Plegar(p.filtros.busqueda)
	if p.buscando {
		texto = nucleo.Plegar(p.entrada)
	}
	return !slices.ContainsFunc(p.datos.Puntos, func(x contrato.EstadoPunto) bool {
		return p.pasa(x) && strings.Contains(nucleo.Plegar(buscable(x)), texto)
	})
}

// opcion es una de las de un campo Opcion: lo que se ve y su valor.
type opcion struct{ etiqueta, valor string }

func etiquetas(ops []opcion) []string {
	r := make([]string, len(ops))
	for i, o := range ops {
		r[i] = o.etiqueta
	}
	return r
}

func etiquetaDe(ops []opcion, valor string) string {
	if i := slices.IndexFunc(ops, func(o opcion) bool { return o.valor == valor }); i >= 0 {
		return ops[i].etiqueta
	}
	return ""
}

// Los modos del formulario.
const (
	modoNueva   = "nueva"
	modoAjustes = "ajustes"
	modoFiltros = "filtros"
)

// formulario es el formulario abierto: crear una API, sus ajustes o los filtros.
type formulario struct {
	f        comp.Formulario
	modo     string
	nombre   string // la API que se ajusta
	ids      []string
	opciones map[string][]opcion // las de cada campo Opcion
	inicial  string
	desde    int // la primera línea que se ve
}

// valor es lo escrito en un campo; en una opción, su valor (no lo que se ve).
func (f *formulario) valor(id string) string {
	if ops, ok := f.opciones[id]; ok {
		v := f.f.Valor(id)
		if i := slices.IndexFunc(ops, func(o opcion) bool { return o.etiqueta == v }); i >= 0 {
			return ops[i].valor
		}
	}
	return f.f.Valor(id)
}

func (f *formulario) valores() string {
	var b strings.Builder
	for _, id := range f.ids {
		b.WriteString(f.f.Valor(id) + "\x00")
	}
	return b.String()
}

// cambiado dice si se perdería algo al cerrarlo: los filtros no se pierden.
func (f *formulario) cambiado() bool { return f.modo != modoFiltros && f.valores() != f.inicial }

func nuevoFormulario(p *Pagina, modo, nombre string, opciones map[string][]opcion, campos ...comp.Campo) *formulario {
	f := &formulario{f: comp.NuevoFormulario(p.estilos, campos...), modo: modo, nombre: nombre, opciones: opciones}
	for _, c := range campos {
		f.ids = append(f.ids, c.ID)
	}
	f.f.Update(tea.WindowSizeMsg{Width: max(p.c.Ancho, 10)})
	f.inicial = f.valores()
	return f
}

// usd lee un importe en dólares, con punto o coma; vacío es 0.
func usd(texto string) (float64, bool) {
	texto = strings.TrimSpace(strings.ReplaceAll(texto, ",", "."))
	if texto == "" {
		return 0, true
	}
	v, err := strconv.ParseFloat(texto, 64)
	return v, err == nil && v >= 0 && !math.IsInf(v, 0)
}

func cifra(n int) string {
	if n <= 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func importe(v float64) string {
	if v <= 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// camposPresupuesto son los del editor de presupuesto: día y mes; peticiones, tokens y USD.
func (p *Pagina) camposPresupuesto(pr *nucleo.Presupuesto) []comp.Campo {
	var v nucleo.Presupuesto
	if pr != nil {
		v = *pr
	}
	var campos []comp.Campo
	for _, per := range []struct {
		id, clave string
		l         nucleo.Limites
	}{{"d", "presupuesto.diario", v.Diario}, {"m", "presupuesto.mensual", v.Mensual}} {
		titulo := p.tx(per.clave) + " · "
		campos = append(campos, comp.Numero(per.id+"p", titulo+p.tx("presupuesto.peticiones"), cifra(per.l.Peticiones)),
			comp.Numero(per.id+"t", titulo+p.tx("presupuesto.tokens"), cifra(per.l.Tokens)),
			comp.Texto(per.id+"u", titulo+p.tx("presupuesto.usd"), importe(per.l.USD)))
	}
	return campos
}

func (f *formulario) presupuesto() *nucleo.Presupuesto {
	d, _ := usd(f.f.Valor("du"))
	m, _ := usd(f.f.Valor("mu"))
	pr := nucleo.Presupuesto{Diario: nucleo.Limites{Peticiones: f.f.Entero("dp"), Tokens: f.f.Entero("dt"), USD: d},
		Mensual: nucleo.Limites{Peticiones: f.f.Entero("mp"), Tokens: f.f.Entero("mt"), USD: m}}
	if pr.Vacio() {
		return nil
	}
	return &pr
}

// opcionesModelo son las del modelo de la receta, como SelectorModelo y el modelo de Probar: el predeterminado del espacio
// (vacío) y los ids del catálogo; el de la API, si el catálogo no lo trae, también.
func (p *Pagina) opcionesModelo(actual string) []opcion {
	ops := []opcion{{p.tx("modelo.esPredeterminado"), ""}}
	for _, m := range p.catalogo {
		if m.Predeterminado {
			ops[0].etiqueta += " (" + m.ID + ")"
		}
		ops = append(ops, opcion{m.ID, m.ID})
	}
	if actual != "" && etiquetaDe(ops, actual) == "" {
		ops = append(ops, opcion{actual, actual})
	}
	return ops
}

func (p *Pagina) opcionesEsfuerzo() []opcion {
	ops := []opcion{{p.tx("esfuerzo.agente"), ""}}
	for _, n := range nucleo.NivelesEsfuerzo {
		ops = append(ops, opcion{p.tx("esfuerzo." + n), n})
	}
	return ops
}

// abrirForm abre el formulario de una API nueva (x nil) o los ajustes de x, como FormularioApi de la Web UI: la receta
// (instrucciones, modelo y esfuerzo), los modelos permitidos y el presupuesto; al crear, también el nombre, el grupo y, en
// las opciones avanzadas, la dirección, la compatibilidad y los alias.
func (p *Pagina) abrirForm(x *contrato.EstadoPunto) tea.Cmd {
	var v contrato.EstadoPunto
	if x != nil {
		v = *x
	}
	ops := map[string][]opcion{"esfuerzo": p.opcionesEsfuerzo()}
	receta := []comp.Campo{comp.Area("instrucciones", p.tx("receta.instrucciones"), v.Instrucciones)}
	var aviso tea.Cmd
	if len(p.catalogo) > 0 { // del catálogo o escrito a mano, como SelectorModelo con opcionEscrita
		ops["modelo"] = p.opcionesModelo(idModelo(v))
		receta = append(receta, comp.Opcion("modelo", p.tx("receta.modelo"), etiquetas(ops["modelo"]), etiquetaDe(ops["modelo"], idModelo(v))),
			comp.Texto("escrito", idioma.T("O escribe agente/modelo"), "").Ejemplo("claude/sonnet"))
	} else { // sin catálogo, texto libre: el servicio lo valida al guardar
		receta = append(receta, comp.Texto("modelo", p.tx("receta.modelo"), idModelo(v)).Ejemplo("perfil/rapido"))
		aviso = comp.Aviso(idioma.T("El catálogo de modelos está vacío: escribe el modelo a mano."), comp.Advertencia)
		if p.errCat != "" {
			aviso = comp.Aviso(idioma.Tf("No llegó el catálogo de modelos (%s): escribe el modelo a mano; r lo vuelve a pedir.", p.errCat), comp.Advertencia)
		}
	}
	receta = append(receta, comp.Interruptor("forzar", p.tx("receta.forzar"), v.ForzarModelo),
		comp.Opcion("esfuerzo", p.tx("esfuerzo.titulo"), etiquetas(ops["esfuerzo"]), etiquetaDe(ops["esfuerzo"], v.Esfuerzo)))
	permitidos := comp.Texto("permitidos", p.tx("puntos.permitidos"), strings.Join(v.ModelosPermitidos, ", ")).Ejemplo(p.tx("puntos.permitidosHint"))
	if x != nil {
		campos := append(append(receta, permitidos), p.camposPresupuesto(v.Presupuesto)...)
		p.form = nuevoFormulario(p, modoAjustes, v.Nombre, ops, campos...)
		p.form.f.Validar = p.validar
		return aviso
	}
	ops["direccion"] = []opcion{{p.tx("puntos.ruta"), "ruta"}, {p.tx("puntos.puertoPropio"), "puerto"}}
	campos := append([]comp.Campo{comp.Texto("nombre", p.tx("puntos.nombre"), "").Ejemplo(p.tx("puntos.nombreHint")),
		comp.Texto("grupo", p.tx("puntos.grupo"), "")}, receta...)
	avanzadas := []comp.Campo{comp.Opcion("direccion", p.tx("puntos.direccion"), etiquetas(ops["direccion"]), ""),
		comp.Numero("puerto", p.tx("puntos.puerto"), "")}
	for _, c := range contratos {
		avanzadas = append(avanzadas, comp.Interruptor("c:"+c, p.tx("puntos.compatibilidad")+": "+p.nombreContrato(c), true))
	}
	avanzadas = append(append(avanzadas, permitidos, comp.Area("alias", p.tx("puntos.alias"), "").Ejemplo("gpt-4o-mini=claude/haiku")),
		p.camposPresupuesto(nil)...)
	campos = append(append(campos, comp.Pliegue("avanzadas", p.tx("comun.opcionesAvanzadas"), false)), comp.Plegados("avanzadas", avanzadas...)...)
	p.form = nuevoFormulario(p, modoNueva, "", ops, campos...)
	p.form.f.Validar = p.validar
	return aviso
}

// modelo es el de la receta: el escrito a mano, si lo hay, gana a la opción elegida.
func (f *formulario) modelo() string {
	if e := strings.TrimSpace(f.f.Valor("escrito")); e != "" {
		return e
	}
	return strings.TrimSpace(f.valor("modelo"))
}

// agentesDelCatalogo son los agentes que conoce el catálogo, para validar un modelo escrito a mano.
func (p *Pagina) agentesDelCatalogo() []string {
	var r []string
	for _, m := range p.catalogo {
		if m.Agente != "" && !slices.Contains(r, m.Agente) {
			r = append(r, m.Agente)
		}
	}
	return r
}

// alias lee los alias de modelo, uno por línea (nombre=agente/modelo); con una línea mal escrita, la devuelve.
func alias(texto string) (map[string]string, string) {
	r := map[string]string{}
	for _, l := range strings.Split(texto, "\n") {
		if l = strings.TrimSpace(l); l == "" {
			continue
		}
		origen, destino, _ := strings.Cut(l, "=")
		if strings.TrimSpace(origen) == "" || strings.TrimSpace(destino) == "" {
			return nil, l
		}
		r[strings.TrimSpace(origen)] = strings.TrimSpace(destino)
	}
	return r, ""
}

func (p *Pagina) validar(f *comp.Formulario) map[string]string {
	errores := map[string]string{}
	for _, id := range []string{"du", "mu"} {
		if _, ok := usd(f.Valor(id)); !ok {
			errores[id] = idioma.T("Escribe un número, por ejemplo 2.50")
		}
	}
	// Escrito a mano: un perfil o un agente que el catálogo conoce, como opcionEscrita de la Web UI.
	if e := strings.TrimSpace(f.Valor("escrito")); e != "" && len(p.catalogo) > 0 {
		agente, _, _ := strings.Cut(e, "/")
		if conocidos := p.agentesDelCatalogo(); !strings.HasPrefix(e, "perfil/") && !slices.Contains(conocidos, agente) {
			errores["escrito"] = p.tx("modelo.libreRegla", "agentes", strings.Join(conocidos, ", "))
		}
	}
	if p.form == nil || p.form.modo != modoNueva {
		return errores
	}
	if !contrato.NombrePuntoValido(strings.TrimSpace(f.Valor("nombre"))) {
		errores["nombre"] = p.tx("puntos.nombreRegla")
	}
	if p.form.valor("direccion") == "puerto" {
		if n := f.Entero("puerto"); n < 1024 || n > 65535 {
			errores["puerto"] = p.tx("puntos.puertoRegla")
		}
	}
	if _, mal := alias(f.Valor("alias")); mal != "" {
		errores["alias"] = p.tx("puntos.aliasInvalido", "linea", mal)
	}
	return errores
}

// lista es una lista escrita con comas o espacios.
func lista(texto string) []string {
	return strings.FieldsFunc(texto, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
}

// enviar manda el formulario: crea la API, guarda sus ajustes o aplica los filtros.
func (p *Pagina) enviar() tea.Cmd {
	f := p.form
	modelo := f.modelo()
	switch f.modo {
	case modoFiltros:
		p.filtros.tiene = nil
		for _, c := range caracteristicas {
			if f.f.Marcado("tiene:" + c.id) {
				p.filtros.tiene = append(p.filtros.tiene, c.id)
			}
		}
		for _, id := range []string{"grupo", "estado", "agente", "contrato", "uso", "orden"} {
			if _, ok := f.opciones[id]; !ok {
				continue
			}
			v := f.valor(id)
			switch id {
			case "grupo":
				p.filtros.grupo = v
			case "estado":
				p.filtros.estado = v
			case "agente":
				p.filtros.agente = v
			case "contrato":
				p.filtros.contrato = v
			case "uso":
				p.filtros.uso = v
			case "orden":
				p.filtros.orden = v
			}
		}
		p.form = nil
		p.aplicarFiltros()
		return nil
	case modoAjustes:
		return p.ajustar(f.nombre, cliente.AjustesPunto{Instrucciones: f.f.Valor("instrucciones"), Modelo: modelo,
			ForzarModelo: f.f.Marcado("forzar") && modelo != "", Esfuerzo: f.valor("esfuerzo"), ModelosPermitidos: lista(f.f.Valor("permitidos")),
			Presupuesto: f.presupuesto()})
	}
	al, _ := alias(f.f.Valor("alias"))
	cfg := contrato.ConfigPunto{Nombre: strings.TrimSpace(f.f.Valor("nombre")), Grupo: strings.TrimSpace(f.f.Valor("grupo")),
		Instrucciones: strings.TrimSpace(f.f.Valor("instrucciones")), ForzarModelo: f.f.Marcado("forzar") && modelo != "",
		Esfuerzo: f.valor("esfuerzo"), ModelosPermitidos: lista(f.f.Valor("permitidos")), Presupuesto: f.presupuesto()}
	if len(al) > 0 {
		cfg.AliasModelos = al
	}
	if f.valor("direccion") == "puerto" {
		cfg.Puerto = f.f.Entero("puerto")
	}
	for _, c := range contratos {
		if f.f.Marcado("c:" + c) {
			cfg.Contratos = append(cfg.Contratos, c)
		}
	}
	switch { // el modelo como lo guarda la configuración: un perfil o el agente con su modelo
	case strings.HasPrefix(modelo, "perfil/"):
		cfg.PerfilModelo = strings.TrimPrefix(modelo, "perfil/")
	case modelo != "":
		cfg.Agente, cfg.Modelo, _ = strings.Cut(modelo, "/")
	}
	return p.crear(cfg)
}

// abrirFiltros abre los filtros y el orden de la Web UI en un formulario de opciones (← → cambia cada una): el grupo, si
// hay grupos, y el agente, si las APIs usan más de uno, como en la Web UI.
func (p *Pagina) abrirFiltros() {
	f := p.filtros
	ops := map[string][]opcion{}
	var campos []comp.Campo
	poner := func(id, clave string, o []opcion, valor string) {
		ops[id] = o
		campos = append(campos, comp.Opcion(id, p.tx(clave), etiquetas(o), etiquetaDe(o, valor)))
	}
	if hayGrupos(p.datos.Puntos) {
		o := []opcion{{p.tx("puntos.todosGrupos"), todos}}
		var nombres []string
		for _, x := range p.datos.Puntos {
			if x.Grupo != "" && !slices.Contains(nombres, x.Grupo) {
				nombres = append(nombres, x.Grupo)
			}
		}
		slices.Sort(nombres)
		for _, g := range nombres {
			o = append(o, opcion{g, g})
		}
		poner("grupo", "puntos.filtroGrupo", append(o, opcion{p.tx("puntos.sinGrupo"), ""}), f.grupo)
	}
	poner("estado", "puntos.filtroEstado", p.opcionesEstado(), f.estado)
	var agentes []string
	for _, x := range p.datos.Puntos {
		if !slices.Contains(agentes, x.Agente) {
			agentes = append(agentes, x.Agente)
		}
	}
	if len(agentes) > 1 {
		o := []opcion{{p.tx("puntos.agentesTodos"), todos}}
		for _, a := range agentes {
			o = append(o, opcion{p.nombreAgente(a), a})
		}
		slices.SortFunc(o[1:], func(a, b opcion) int { return strings.Compare(a.etiqueta, b.etiqueta) })
		poner("agente", "puntos.filtroAgente", o, f.agente)
	}
	o := []opcion{{p.tx("puntos.contratosTodos"), ""}}
	for _, c := range contratos {
		o = append(o, opcion{p.nombreContrato(c), c})
	}
	poner("contrato", "puntos.filtroContrato", o, f.contrato)
	for _, c := range caracteristicas {
		campos = append(campos, comp.Interruptor("tiene:"+c.id, p.tx("puntos.filtroCaracteristicas")+": "+p.tx(c.clave), slices.Contains(f.tiene, c.id)))
	}
	poner("uso", "puntos.filtroUso", p.opcionesUso(), f.uso)
	poner("orden", "puntos.orden", []opcion{{p.tx("puntos.ordenNombre"), "nombre"}, {p.tx("puntos.ordenUso"), "uso"},
		{p.tx("puntos.ordenUltima"), "ultima"}}, f.orden)
	p.form = nuevoFormulario(p, modoFiltros, "", ops, campos...)
}

func (p *Pagina) opcionesEstado() []opcion {
	return []opcion{{p.tx("puntos.estadoTodas"), "todas"}, {p.tx("puntos.estadoActivas"), "activas"},
		{p.tx("puntos.estadoDetenidas"), "detenidas"}, {p.tx("puntos.estadoError"), "error"}}
}

func (p *Pagina) opcionesUso() []opcion {
	return []opcion{{p.tx("puntos.usoTodas"), "todas"}, {p.tx("puntos.usoCon"), "con"}, {p.tx("puntos.usoSin"), "sin"}}
}

// nombreAgente es el nombre visible del agente de una API; sin agente propio, el predeterminado del espacio.
func (p *Pagina) nombreAgente(a string) string {
	if a == "" {
		return p.tx("puntos.agentePredeterminado")
	}
	return catalogo.NombreVisible(a)
}

// resumenFiltros dice qué filtros hay puestos: «Grupo: web · Estado: Activas».
func (p *Pagina) resumenFiltros() string {
	f := p.filtros
	var partes []string
	poner := func(clave, valor string) { partes = append(partes, p.tx(clave)+": "+valor) }
	if f.busqueda != "" {
		poner("puntos.buscarApi", f.busqueda)
	}
	if f.grupo != todos {
		poner("puntos.filtroGrupo", cmp.Or(f.grupo, p.tx("puntos.sinGrupo")))
	}
	if f.estado != "todas" {
		poner("puntos.filtroEstado", etiquetaDe(p.opcionesEstado(), f.estado))
	}
	if f.agente != todos {
		poner("puntos.filtroAgente", p.nombreAgente(f.agente))
	}
	if f.contrato != "" {
		poner("puntos.filtroContrato", p.nombreContrato(f.contrato))
	}
	if len(f.tiene) > 0 {
		var nombres []string
		for _, c := range caracteristicas {
			if slices.Contains(f.tiene, c.id) {
				nombres = append(nombres, p.tx(c.clave))
			}
		}
		poner("puntos.filtroCaracteristicas", strings.Join(nombres, ", "))
	}
	if f.uso != "todas" {
		poner("puntos.filtroUso", etiquetaDe(p.opcionesUso(), f.uso))
	}
	return strings.Join(partes, " · ")
}
