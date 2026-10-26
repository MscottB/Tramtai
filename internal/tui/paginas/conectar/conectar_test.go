package conectar_test

import (
	"context"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/agentes"
	"tramtai/internal/cliente"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
	"tramtai/internal/servicio"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/conectar"
	"tramtai/internal/tui/paginas/prueba"
)

// Las pruebas de Mis APIs van por su interfaz, con el arnés de las páginas (prueba.App): teclas, los mensajes de pagina y
// comp, y lo que enseña Vista(). Lo que trae el servicio tras una tecla se espera con HastaVer o Hasta antes de mirarlo, y
// lo que la página cambia en el servicio se lee en el servicio.

var (
	aviso    = prueba.Aviso
	contexto = prueba.Contexto
	reClave  = regexp.MustCompile(`tt_[A-Za-z0-9_-]+`)
)

// abierta inicia la página contra ese servicio: arranca con «Cargando…» y espera la lista.
func abierta(t *testing.T, cli *cliente.Cliente, id string) *prueba.App {
	t.Helper()
	return abiertaCon(t, contexto(t, cli, id))
}

func abiertaCon(t *testing.T, c pagina.Contexto) *prueba.App {
	t.Helper()
	a := prueba.Conducir(t, conectar.Nueva("es"))
	cmd := a.P.Iniciar(c)
	if v := a.Vista(); !strings.Contains(v, "Cargando…") {
		t.Fatalf("arranca con «Cargando…»:\n%s", v)
	}
	a.Correr(cmd)
	a.EsperarCarga()
	return a
}

func crear(t *testing.T, cli *cliente.Cliente, id string, p contrato.ConfigPunto) string {
	t.Helper()
	if p.Agente == "" {
		p.Agente = "simulado"
	}
	_, clave, err := cli.CrearPunto(context.Background(), id, p)
	if err != nil {
		t.Fatal(err)
	}
	return clave
}

func punto(t *testing.T, cli *cliente.Cliente, id, nombre string) (contrato.EstadoPunto, bool) {
	t.Helper()
	l, err := cli.Puntos(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(l.Puntos, func(p contrato.EstadoPunto) bool { return p.Nombre == nombre })
	if i < 0 {
		return contrato.EstadoPunto{}, false
	}
	return l.Puntos[i], true
}

// hastaServicio espera, entregando a la página lo que llegue, a que la API cumpla cond en el servicio.
func hastaServicio(t *testing.T, a *prueba.App, cli *cliente.Cliente, id, nombre string, cond func(contrato.EstadoPunto, bool) bool) {
	t.Helper()
	a.Hasta(func() bool { return cond(punto(t, cli, id, nombre)) })
}

// hastaAviso hace la acción y espera, entregando a la página lo que llegue, al primer aviso que sale tras ella: lo trae
// la respuesta del servicio, que no se da por llegada con solo asentar la página.
func hastaAviso(a *prueba.App, accion func()) comp.NuevoAviso {
	desde := len(a.Fuera)
	accion()
	var av comp.NuevoAviso
	a.Hasta(func() bool {
		var ok bool
		av, ok = aviso(a.Fuera[desde:])
		return ok
	})
	return av
}

func pregunta(fuera []tea.Msg) (comp.Pregunta, bool) {
	for _, m := range fuera {
		if q, ok := m.(comp.Pregunta); ok {
			return q, true
		}
	}
	return comp.Pregunta{}, false
}

func copiado(fuera []tea.Msg) string {
	for _, m := range fuera {
		if c, ok := m.(prueba.Portapapeles); ok {
			return string(c)
		}
	}
	return ""
}

// TestApisPorElServicio: crear (con un modelo del catálogo), ver la clave una sola vez, copiarla, activar y detener,
// regenerar la clave y eliminar, todo contra el servicio, como el TestAccesosPorElServicio de la terminal antigua.
func TestApisPorElServicio(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	a.Entregar(tea.WindowSizeMsg{Width: 90, Height: 40}) // la clave mide más de 60 columnas: así cabe en una línea
	if v := a.Vista(); !strings.Contains(v, "Todavía no hay APIs por aplicación") || !strings.Contains(v, "Nueva API") {
		t.Fatalf("sin APIs, el vacío de la Web UI:\n%s", v)
	}
	if a.P.Titulo() != "Mis APIs" || a.P.Ayuda() != "conectar" || a.P.EnCampo() {
		t.Fatalf("título %q, ayuda %q", a.P.Titulo(), a.P.Ayuda())
	}

	// Crear: n abre el formulario, el nombre se valida, el modelo se elige del catálogo y Ctrl+S la crea.
	a.Pulsar("n")
	if v := a.Vista(); !a.P.EnCampo() || !strings.Contains(v, "Nombre") || !strings.Contains(v, "se editan en Probar") {
		t.Fatalf("n abre el formulario, que dice dónde se edita el resto de la receta:\n%s", v)
	}
	a.Escribir("Mi App")
	a.Pulsar("ctrl+s")
	if v := a.Vista(); !strings.Contains(v, "Minúsculas, números") {
		t.Fatalf("un nombre no válido no se envía:\n%s", v)
	}
	for range 6 {
		a.Pulsar("backspace")
	}
	a.Escribir("mi-app")
	a.Pulsar("tab", "tab", "tab") // grupo, instrucciones y modelo
	if v := a.Vista(); !strings.Contains(v, "‹ Predeterminado (simulado) ›") {
		t.Fatalf("el modelo es una opción del catálogo, con el predeterminado del espacio primero:\n%s", v)
	}
	a.Pulsar("right")
	if v := a.Vista(); !strings.Contains(v, "‹ perfil/equilibrado ›") {
		t.Fatalf("→ elige el siguiente del catálogo:\n%s", v)
	}
	a.Pulsar("ctrl+s")
	a.HastaVer("API lista", "Copia la clave ahora")
	clave := reClave.FindString(a.Vista())
	if clave == "" {
		t.Fatalf("al crearla se abre el detalle con la clave, que solo se ve esta vez:\n%s", a.Vista())
	}
	if p, ok := punto(t, cli, id, "mi-app"); !ok || p.PerfilModelo != "equilibrado" {
		t.Fatalf("la API está en el servicio, con el perfil elegido: %+v", p)
	}
	if c := copiado(a.Pulsar("y")); c != clave {
		t.Fatalf("y copia la clave: %q", c)
	}
	if p, ok := a.P.(pagina.Pendiente); !ok || !strings.Contains(p.Pendiente(), "no se vuelve a mostrar") {
		t.Fatal("con la clave a la vista, salir pregunta")
	}

	// Cerrar con la clave a la vista pregunta; Cancelar sigue en el detalle.
	q, ok := pregunta(a.Pulsar("esc"))
	if !ok || !strings.Contains(q.Texto, "¿Cerrar sin copiar la clave?") {
		t.Fatalf("Esc con la clave a la vista pregunta: %+v", q)
	}
	a.Entregar(comp.Cancelado{})
	if !strings.Contains(a.Vista(), clave) {
		t.Fatal("Cancelar deja la clave a la vista")
	}
	a.Pulsar("esc")
	a.Entregar(comp.Confirmado{})
	if v := a.Vista(); strings.Contains(v, clave) || !strings.Contains(v, "mi-app") {
		t.Fatalf("confirmar cierra el detalle y olvida la clave:\n%s", v)
	}

	// Detener y activar desde el detalle.
	a.Pulsar("enter")
	if v := a.Vista(); !strings.Contains(v, "La clave se ve una vez") {
		t.Fatalf("sin clave a la vista, el detalle lo explica:\n%s", v)
	}
	a.Pulsar("a")
	a.HastaVer("Detenida")
	hastaServicio(t, a, cli, id, "mi-app", func(p contrato.EstadoPunto, _ bool) bool { return !p.Activo })
	a.Pulsar("a")
	a.HastaVer("Activa")
	hastaServicio(t, a, cli, id, "mi-app", func(p contrato.EstadoPunto, _ bool) bool { return p.Activo })

	// Regenerar la clave: pregunta y enseña la nueva una vez.
	q, ok = pregunta(a.Pulsar("t"))
	if !ok || !strings.Contains(q.Texto, "¿Regenerar la clave de mi-app?") {
		t.Fatalf("t pregunta antes de regenerar: %+v", q)
	}
	a.Entregar(comp.Confirmado{})
	a.Hasta(func() bool { n := reClave.FindString(a.Vista()); return n != "" && n != clave })

	// Eliminar: pregunta, recuerda cuál y cierra el detalle.
	q, ok = pregunta(a.Pulsar("x"))
	if !ok || !strings.Contains(q.Texto, "Eliminar mi-app") {
		t.Fatalf("x pregunta antes de eliminar: %+v", q)
	}
	a.Entregar(comp.Confirmado{})
	a.HastaVer("Todavía no hay APIs por aplicación")
	if _, ok := punto(t, cli, id, "mi-app"); ok {
		t.Fatal("la API ya no está en el servicio")
	}
}

// TestApisFiltrosYGrupos: los grupos plegables de la Web UI (los sin grupo al final), la búsqueda, los filtros de grupo,
// estado y «Tiene», quitar filtros y g, que activa o detiene el grupo entero.
func TestApisFiltrosYGrupos(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, contrato.ConfigPunto{Nombre: "mi-app", Grupo: "web", Instrucciones: "Sé breve."})
	crear(t, cli, id, contrato.ConfigPunto{Nombre: "otra", Grupo: "web"})
	crear(t, cli, id, contrato.ConfigPunto{Nombre: "notas"})
	if err := cli.ActivarPunto(context.Background(), id, "notas", false); err != nil {
		t.Fatal(err)
	}
	a := abierta(t, cli, id)
	v := a.Vista()
	if !strings.Contains(v, "APIs por aplicación (3)") || !strings.Contains(v, "web (2)") || !strings.Contains(v, "Sin grupo (1)") ||
		strings.Index(v, "web (2)") > strings.Index(v, "Sin grupo (1)") {
		t.Fatalf("los grupos, con los sin grupo al final:\n%s", v)
	}

	// La búsqueda.
	a.Pulsar("/")
	if !a.P.EnCampo() {
		t.Fatal("con la búsqueda abierta, las letras son del campo")
	}
	a.Escribir("otra")
	a.Pulsar("enter")
	if v := a.Vista(); !strings.Contains(v, "otra") || strings.Contains(v, "mi-app") || strings.Contains(v, "notas") {
		t.Fatalf("la búsqueda deja otra:\n%s", v)
	}
	a.Pulsar("l")
	if v := a.Vista(); !strings.Contains(v, "mi-app") || !strings.Contains(v, "notas") {
		t.Fatalf("l quita los filtros:\n%s", v)
	}

	// El filtro de grupo y el de estado, en el formulario de filtros.
	a.Pulsar("f")
	a.Pulsar("right", "ctrl+s") // Grupo: web
	if v := a.Vista(); !strings.Contains(v, "mi-app") || strings.Contains(v, "notas") || !strings.Contains(v, "Grupo: web") {
		t.Fatalf("el filtro de grupo deja web:\n%s", v)
	}
	a.Pulsar("l", "f", "tab", "right", "right", "ctrl+s") // Estado: Detenidas
	if v := a.Vista(); !strings.Contains(v, "notas") || strings.Contains(v, "mi-app") {
		t.Fatalf("el filtro de estado deja las detenidas:\n%s", v)
	}
	a.Pulsar("f", "tab", "right", "ctrl+s") // Estado: Con error, que ninguna tiene
	if v := a.Vista(); !strings.Contains(v, "Ninguna API coincide con el filtro.") {
		t.Fatalf("sin resultados, lo dice:\n%s", v)
	}
	a.Pulsar("l")

	// «Tiene»: las seis características de la Web UI; Instrucciones deja la que las tiene.
	a.Pulsar("f")
	if v := a.Vista(); !strings.Contains(v, "Tiene: Respuesta JSON") {
		t.Fatalf("el formulario trae las características:\n%s", v)
	}
	a.Pulsar("tab", "tab", "tab", "tab", "tab", "tab") // estado, contrato, JSON, respaldo, presupuesto, instrucciones
	a.Pulsar("space", "ctrl+s")
	if v := a.Vista(); !strings.Contains(v, "mi-app") || strings.Contains(v, "otra") || !strings.Contains(v, "Tiene: Instrucciones") {
		t.Fatalf("el filtro «Tiene» deja las que tienen instrucciones:\n%s", v)
	}
	a.Pulsar("l")

	// g en la cabecera de web: todas activas, así que las detiene; otra vez, las activa.
	a.Pulsar("home", "g")
	for _, n := range []string{"mi-app", "otra"} {
		hastaServicio(t, a, cli, id, n, func(p contrato.EstadoPunto, _ bool) bool { return !p.Activo })
	}
	a.HastaVer("otra                 ○ Detenida")
	if p, _ := punto(t, cli, id, "notas"); p.Activo {
		t.Fatal("g no toca otros grupos")
	}
	a.Pulsar("down", "g") // desde una de sus APIs también vale
	for _, n := range []string{"mi-app", "otra"} {
		hastaServicio(t, a, cli, id, n, func(p contrato.EstadoPunto, _ bool) bool { return p.Activo })
	}
	a.HastaVer("otra                 ● Activa")

	// Si g falla a medias (otra ya no existe), avisa y la lista se recarga igual: otra desaparece.
	if err := cli.EliminarPunto(context.Background(), id, "otra"); err != nil {
		t.Fatal(err)
	}
	a.Pulsar("home")
	if av := hastaAviso(a, func() { a.Pulsar("g") }); av.Tipo != comp.Fallo {
		t.Fatalf("el fallo se avisa: %+v", av)
	}
	a.Hasta(func() bool { return !strings.Contains(a.Vista(), "otra") })
}

// TestEditarPresupuesto: e abre los ajustes de la API; el presupuesto del día y del mes se guarda con AjustarPunto y,
// vacío, se quita.
func TestEditarPresupuesto(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, contrato.ConfigPunto{Nombre: "mi-app", Instrucciones: "Sé breve."})
	a := abierta(t, cli, id)
	a.Pulsar("e")
	if v := a.Vista(); !strings.Contains(v, "Ajustes de mi-app") || !strings.Contains(v, "breve.") || !strings.Contains(v, "‹ simulado ›") {
		t.Fatalf("e abre los ajustes con lo que tiene:\n%s", v)
	}
	a.Pulsar("tab", "tab", "tab", "tab", "tab", "tab") // instrucciones, modelo, escrito a mano, forzar, esfuerzo y permitidos
	a.Escribir("50")
	a.Pulsar("tab", "tab")
	a.Escribir("2,5")
	a.Pulsar("ctrl+s")
	hastaServicio(t, a, cli, id, "mi-app", func(p contrato.EstadoPunto, _ bool) bool {
		return p.Presupuesto != nil && p.Presupuesto.Diario.Peticiones == 50 && p.Presupuesto.Diario.USD == 2.5
	})
	a.Hasta(func() bool { return !a.P.EnCampo() })
	if p, _ := punto(t, cli, id, "mi-app"); p.Instrucciones != "Sé breve." || p.Agente != "simulado" {
		t.Fatalf("lo demás sigue: %+v", p)
	}

	// Un USD que no es un número no se envía.
	a.Pulsar("e", "tab", "tab", "tab", "tab", "tab", "tab", "tab", "tab")
	a.Escribir("x")
	a.Pulsar("ctrl+s")
	if !a.P.EnCampo() {
		t.Fatal("un USD no válido no se envía")
	}
	a.Pulsar("esc")
	a.Entregar(comp.Confirmado{}) // cerrar sin guardar

	// Vaciarlo lo quita.
	a.Pulsar("e", "tab", "tab", "tab", "tab", "tab", "tab", "backspace", "backspace", "tab", "tab")
	for range 3 {
		a.Pulsar("backspace")
	}
	a.Pulsar("ctrl+s")
	hastaServicio(t, a, cli, id, "mi-app", func(p contrato.EstadoPunto, _ bool) bool { return p.Presupuesto == nil })
}

// TestConsumoComoLaWeb: las barras del consumo salen de los mismos campos de /api/v1/puntos y con las mismas cuentas que
// BarrasPresupuesto.vue: una barra por límite definido (día y mes; peticiones, tokens y USD), «usado / límite» y la
// fracción usada. Cada fila de la lista lleva la barra corta de su límite más usado.
func TestConsumoComoLaWeb(t *testing.T) {
	e := nucleo.EstadoPresupuesto{
		Diario: nucleo.EstadoPeriodo{Limites: nucleo.Limites{Peticiones: 10, USD: 2},
			Consumo: nucleo.Consumo{Peticiones: 8, Tokens: 999, USD: 0.5}},
		Mensual: nucleo.EstadoPeriodo{Limites: nucleo.Limites{Tokens: 20000}, Consumo: nucleo.Consumo{Tokens: 12345}},
	}
	quiere := [][3]string{{"Diario · Peticiones", "8 / 10", "0.80"}, {"Diario · USD", "$0,50 / $2,00", "0.25"},
		{"Mensual · Tokens", "12.345 / 20.000", "0.62"}}
	if got := conectar.Barras("es", e); !slices.Equal(got, quiere) {
		t.Fatalf("barras:\n%v\nquiere\n%v", got, quiere)
	}
	if got := conectar.Barras("en", e); got[2][0] != "Monthly · Tokens" || got[2][1] != "12,345 / 20,000" {
		t.Fatalf("en inglés: %v", got)
	}
	e.Diario.Consumo.Peticiones = 1234
	e.Diario.Limites.Peticiones = 1000
	if got := conectar.Barras("es", e); got[0][1] != "1234 / 1000" || got[0][2] != "1.23" {
		t.Fatalf("como toLocaleString en español, sin separar 4 cifras: %v", got)
	}

	// Contra el servicio: dos peticiones por la API cuentan en su barra del día.
	cli, id := prueba.Servicio(t)
	clave := crear(t, cli, id, contrato.ConfigPunto{Nombre: "mi-app",
		Presupuesto: &nucleo.Presupuesto{Diario: nucleo.Limites{Peticiones: 10}, Mensual: nucleo.Limites{USD: 2}}})
	for range 2 {
		req, _ := http.NewRequest("POST", cli.Base+"/p/mi-app/api/v1/trabajos", strings.NewReader(`{"mensajes":[{"rol":"usuario","contenido":"hola"}]}`))
		req.Header.Set("Authorization", "Bearer "+clave)
		req.Header.Set("Content-Type", "application/json")
		r, err := http.DefaultClient.Do(req)
		if err != nil || r.StatusCode >= 300 {
			t.Fatalf("petición por la API: %v %v", r, err)
		}
		r.Body.Close()
	}
	a := abierta(t, cli, id)
	a.Entregar(tea.WindowSizeMsg{Width: 70, Height: 40})
	a.HastaVer("██░░░░░░ 20 %", "2 peticiones en")
	a.Pulsar("enter")
	a.HastaVer("Diario · Peticiones", "2 / 10", "Mensual · USD", "$0,00 / $2,00", "Se reinicia el día")
	if v := a.Vista(); strings.Contains(v, "Diario · Tokens") {
		t.Fatalf("sin límite no hay barra:\n%s", v)
	}

	// R reinicia el consumo, con pregunta.
	q, ok := pregunta(a.Pulsar("R"))
	if !ok || !strings.Contains(q.Texto, "Reiniciar el consumo de mi-app") {
		t.Fatalf("R pregunta: %+v", q)
	}
	if av := hastaAviso(a, func() { a.Entregar(comp.Confirmado{}) }); av.Texto != "Consumo reiniciado" {
		t.Fatalf("avisa: %+v", av)
	}
	a.HastaVer("0 / 10")
}

// conPresupuestoDelEspacio es el servicio de las pruebas con un presupuesto mensual de 4 peticiones en el espacio.
func conPresupuestoDelEspacio(t *testing.T) (*cliente.Cliente, string) {
	t.Helper()
	return prueba.Ajustado(t, func(o *servicio.Opciones) {
		global := os.Getenv("TRAMTAI_CONFIG_GLOBAL")
		if err := os.WriteFile(global, []byte("predeterminados:\n  agente: simulado\npresupuesto:\n  mensual:\n    peticiones: 4\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		c, err := configuracion.CargarConfiguracion("", agentes.Internos())
		if err != nil {
			t.Fatal(err)
		}
		o.Config = c
	})
}

// TestClaveDelEspacio: la segunda pestaña, como en la Web UI: las URL del espacio, qué poner en model (los ids del
// catálogo agrupados), la clave oculta que se ve con v, se copia con y y se regenera con t (con pregunta), y el
// presupuesto del espacio con su reinicio.
func TestClaveDelEspacio(t *testing.T) {
	cli, id := conPresupuestoDelEspacio(t)
	a := abierta(t, cli, id)
	a.Entregar(tea.WindowSizeMsg{Width: 70, Height: 60})
	clave, err := cli.ClaveInstancia(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if v := a.Vista(); !strings.Contains(v, "Clave del espacio") {
		t.Fatalf("la pestaña se ve:\n%s", v)
	}
	a.Pulsar("right")
	v := a.Vista()
	if !strings.Contains(v, clave[:6]+"•••") || strings.Contains(v, clave) || !strings.Contains(v, "/openai/v1") || !strings.Contains(v, "/anthropic") {
		t.Fatalf("las URL y la clave oculta:\n%s", v)
	}
	for _, s := range []string{"Modelo: el campo model elige agente y modelo", "Perfiles", "perfil/equilibrado", "perfil/rapido"} {
		if !strings.Contains(v, s) {
			t.Fatalf("falta %q de los modelos:\n%s", s, v)
		}
	}
	a.Pulsar("v")
	if !strings.Contains(a.Vista(), clave) {
		t.Fatalf("v enseña la clave:\n%s", a.Vista())
	}
	if c := copiado(a.Pulsar("y")); c != clave {
		t.Fatalf("y copia la clave: %q", c)
	}
	for _, s := range []string{"Presupuesto del espacio", "Mensual · Peticiones", "0 / 4"} {
		if !strings.Contains(a.Vista(), s) {
			t.Fatalf("falta %q:\n%s", s, a.Vista())
		}
	}

	q, ok := pregunta(a.Pulsar("t"))
	if !ok || !strings.Contains(q.Texto, "¿Regenerar la clave de Tramtai?") {
		t.Fatalf("t pregunta: %+v", q)
	}
	if av := hastaAviso(a, func() { a.Entregar(comp.Confirmado{}) }); !strings.Contains(av.Texto, "Clave regenerada") {
		t.Fatalf("avisa: %+v", av)
	}
	nueva, _ := cli.ClaveInstancia(context.Background(), id)
	if nueva == clave {
		t.Fatal("la clave cambia en el servicio")
	}
	a.HastaVer(nueva)

	q, ok = pregunta(a.Pulsar("R"))
	if !ok || !strings.Contains(q.Texto, "Reiniciar el consumo del espacio") {
		t.Fatalf("R pregunta: %+v", q)
	}
	if av := hastaAviso(a, func() { a.Entregar(comp.Confirmado{}) }); av.Texto != "Consumo reiniciado" {
		t.Fatalf("avisa: %+v", av)
	}
	a.HastaVer("0 / 4")
	a.Pulsar("left")
	if v := a.Vista(); !strings.Contains(v, "Todavía no hay APIs") {
		t.Fatalf("← vuelve a las APIs:\n%s", v)
	}
}

func TestEscVuelve(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, contrato.ConfigPunto{Nombre: "mi-app"})
	a := abierta(t, cli, id)
	a.Pulsar("enter")
	if f := a.Pulsar("esc"); len(f) != 0 || !strings.Contains(a.Vista(), "APIs por aplicación (1)") {
		t.Fatalf("Esc cierra el detalle: %v\n%s", f, a.Vista())
	}
	a.Pulsar("/")
	if f := a.Pulsar("esc"); len(f) != 0 || a.P.EnCampo() {
		t.Fatalf("Esc cierra la búsqueda: %v", f)
	}
	a.Pulsar("n")
	if f := a.Pulsar("esc"); len(f) != 0 || a.P.EnCampo() {
		t.Fatalf("Esc cierra el formulario sin cambios: %v", f)
	}
	if f := a.Pulsar("esc"); len(f) != 1 || f[0] != (pagina.Volver{}) {
		t.Fatalf("sin nada abierto, Esc vuelve: %+v", f)
	}
	a.Pulsar("right")
	if f := a.Pulsar("esc"); len(f) != 1 || f[0] != (pagina.Volver{}) {
		t.Fatalf("en la clave del espacio, Esc vuelve: %+v", f)
	}
}

// TestIrAProbarYAlHistorial: desde el detalle, p abre la API en Probar y h su historial.
func TestIrAProbarYAlHistorial(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, contrato.ConfigPunto{Nombre: "mi-app"})
	a := abierta(t, cli, id)
	a.Pulsar("enter")
	if f := a.Pulsar("p"); len(f) != 1 || f[0] != (pagina.IrA{Pagina: "ejecutar", Datos: pagina.ConPunto{Nombre: "mi-app"}}) {
		t.Fatalf("p lleva a Probar: %+v", f)
	}
	if f := a.Pulsar("h"); len(f) != 1 || f[0] != (pagina.IrA{Pagina: "historial", Datos: pagina.FiltroPunto{Nombre: "mi-app"}}) {
		t.Fatalf("h lleva al Historial: %+v", f)
	}
}

func TestServicioCaido(t *testing.T) {
	// hastaError espera a que salga hacia la app un ErrorServicio de conexión.
	hastaError := func(a *prueba.App) {
		a.Hasta(func() bool {
			return slices.ContainsFunc(a.Fuera, func(m tea.Msg) bool { e, ok := m.(pagina.ErrorServicio); return ok && pagina.NoResponde(e.Err) })
		})
	}
	sin := prueba.Conducir(t, conectar.Nueva("es"))
	sin.Correr(sin.P.Iniciar(contexto(t, nil, "global")))
	hastaError(sin) // sin cliente
	cli, id, cerrar := prueba.Arrancar(t)
	a := abierta(t, cli, id)
	cerrar()
	a.Pulsar("r")
	hastaError(a) // con el servicio cerrado, y la página no se cierra
	a.HastaVer("El servicio no responde · r reintenta")
}

// TestErrorDeCarga: si la lista no llega, se dice con el mensaje del servicio y r reintenta, sin invitar a crear la
// primera: en la primera carga y también al recargar una lista vacía. Con APIs a la vista, el error va en un aviso y la
// lista no se tapa.
func TestErrorDeCarga(t *testing.T) {
	cli, id := prueba.Servicio(t)
	proxy, fallar := prueba.ConFallo(t, cli)
	fallar("la configuración no se pudo leer")
	a := abierta(t, proxy, id)
	if v := a.Vista(); !strings.Contains(v, "No se pudo cargar: la configuración no se pudo leer") || !strings.Contains(v, "r Reintentar") ||
		strings.Contains(v, "Todavía no hay APIs") {
		t.Fatalf("el error de carga:\n%s", v)
	}
	fallar("")
	a.Pulsar("r")
	a.HastaVer("Todavía no hay APIs")

	// Cargó vacía y la recarga falla: el error no se pierde.
	fallar("el servicio no pudo leer las APIs")
	a.Pulsar("r")
	a.HastaVer("No se pudo cargar: el servicio no pudo leer las APIs", "r Reintentar")
	if v := a.Vista(); strings.Contains(v, "Todavía no hay APIs") {
		t.Fatalf("no invita a crear la primera:\n%s", v)
	}
	fallar("")
	a.Pulsar("r")
	a.HastaVer("Todavía no hay APIs")

	// Con APIs, un fallo al recargar es un aviso y la lista sigue.
	crear(t, cli, id, contrato.ConfigPunto{Nombre: "mi-app"})
	a.Pulsar("r")
	a.HastaVer("mi-app")
	fallar("otra vez no")
	if av := hastaAviso(a, func() { a.Pulsar("r") }); av.Texto != "otra vez no" || av.Tipo != comp.Fallo {
		t.Fatalf("un aviso de fallo: %+v", av)
	}
	if v := a.Vista(); !strings.Contains(v, "mi-app") {
		t.Fatalf("la lista sigue:\n%s", v)
	}
}

// TestRespuestasViejas: una carga pedida antes de volver a iniciar la página se descarta, aunque llegue después; un cambio
// ya hecho en el servicio en el mismo espacio sí se aplica, aunque la página se haya ocultado. (Al volver a iniciarla, la
// app cancela el Ctx de antes: ese cambio ya no llega.)
func TestRespuestasViejas(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, contrato.ConfigPunto{Nombre: "mi-app"})
	a := abierta(t, cli, id)

	vieja := a.Recoger(a.Retener(func() { a.Pulsar("r") })) // la carga de antes, con solo mi-app
	crear(t, cli, id, contrato.ConfigPunto{Nombre: "otra"})
	a.Correr(a.P.Iniciar(contexto(t, cli, id)))
	a.HastaVer("otra")
	if len(vieja) != 1 {
		t.Fatalf("la carga vieja trae un mensaje: %v", vieja)
	}
	a.Entregar(vieja[0])
	if v := a.Vista(); !strings.Contains(v, "otra") || !strings.Contains(v, "APIs por aplicación (2)") {
		t.Fatalf("la carga vieja se descarta:\n%s", v)
	}

	// Una carga de otro espacio tampoco vale.
	otro := a.Recoger(a.Retener(func() { a.Pulsar("r") }))
	c := contexto(t, cli, id)
	c.Instancia = "otro"
	a.P.Iniciar(c) // la app cambió de espacio; su carga no se corre
	a.Entregar(otro[0])
	if v := a.Vista(); !strings.Contains(v, "Cargando…") {
		t.Fatalf("la carga de otro espacio se descarta:\n%s", v)
	}

	// Un cambio retenido del mismo espacio se aplica aunque llegue tarde, con la página dejada atrás: al volver de la Ayuda
	// la app no la inicia otra vez, y su Ctx sigue vivo.
	a = abierta(t, cli, id)
	accion := a.Retener(func() { a.Pulsar("a") }) // detener mi-app, la primera
	a.Soltar(accion)
	hastaServicio(t, a, cli, id, "mi-app", func(p contrato.EstadoPunto, _ bool) bool { return !p.Activo })
	a.HastaVer("mi-app               ○ Detenida")
}

// TestCabe: la lista, el detalle, el formulario, los filtros y la clave del espacio caben en el área de 60×20, también con
// una ventana más estrecha, y las preguntas caben en la caja de la app.
func TestCabe(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, contrato.ConfigPunto{Nombre: "mi-app-con-un-nombre-bastante-largo", Grupo: "web",
		Presupuesto: &nucleo.Presupuesto{Diario: nucleo.Limites{Peticiones: 10, Tokens: 1000, USD: 2}, Mensual: nucleo.Limites{Peticiones: 100}}})
	crear(t, cli, id, contrato.ConfigPunto{Nombre: "notas"})
	a := abierta(t, cli, id)
	for _, tam := range [][2]int{{60, 17}, {40, 10}} {
		a.Entregar(tea.WindowSizeMsg{Width: tam[0], Height: tam[1]})
		comprobar := func(donde string) {
			t.Helper()
			v := a.Vista()
			if lipgloss.Height(v) > tam[1] || lipgloss.Width(v) > tam[0] {
				t.Errorf("%s a %dx%d mide %dx%d:\n%s", donde, tam[0], tam[1], lipgloss.Width(v), lipgloss.Height(v), v)
			}
		}
		comprobar("lista")
		a.Pulsar("down", "enter")
		comprobar("detalle")
		a.Pulsar("esc", "n")
		comprobar("formulario")
		a.Pulsar("esc", "f")
		comprobar("filtros")
		a.Pulsar("esc", "right")
		comprobar("clave del espacio")
		a.Pulsar("left")
	}
	a.Entregar(tea.WindowSizeMsg{Width: 60, Height: 17})
	var preguntas []string
	a.Pulsar("home", "down", "enter")
	for _, k := range []string{"t", "x", "R"} {
		q, _ := pregunta(a.Pulsar(k))
		a.Entregar(comp.Cancelado{})
		preguntas = append(preguntas, q.Texto)
	}
	a.Pulsar("esc", "right")
	q, _ := pregunta(a.Pulsar("t"))
	preguntas = append(preguntas, q.Texto)
	for _, texto := range preguntas {
		if texto == "" {
			t.Fatal("falta una pregunta")
		}
		v := (&comp.Confirmacion{Texto: texto}).Vista(&pagina.Estilos{Iconos: pagina.IconosUnicode}, 58, 17)
		if h := lipgloss.Height(v); h > 17 || !strings.Contains(v, "s confirma · Esc cancela") {
			t.Fatalf("la pregunta mide %d filas:\n%s", h, v)
		}
	}
}
