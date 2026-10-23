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
// comp, y lo que enseña Vista(). Lo que la página cambia en el servicio se lee en el servicio.

var (
	aviso    = prueba.Aviso
	contexto = prueba.Contexto
	reClave  = regexp.MustCompile(`tt_[A-Za-z0-9_-]+`)
)

// abierta inicia la página contra ese servicio: arranca con «Cargando…» y espera la lista.
func abierta(t *testing.T, cli *cliente.Cliente, id string) *prueba.App {
	t.Helper()
	a := prueba.Conducir(t, conectar.Nueva("es"))
	cmd := a.P.Iniciar(contexto(t, cli, id))
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

// TestApisPorElServicio: crear, ver la clave una sola vez, copiarla, activar y detener, regenerar la clave y eliminar,
// todo contra el servicio, como el TestAccesosPorElServicio de la terminal antigua.
func TestApisPorElServicio(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	a.Entregar(tea.WindowSizeMsg{Width: 90, Height: 30}) // la clave mide más de 60 columnas: así cabe en una línea
	if v := a.Vista(); !strings.Contains(v, "Todavía no hay APIs por aplicación") || !strings.Contains(v, "Nueva API") {
		t.Fatalf("sin APIs, el vacío de la Web UI:\n%s", v)
	}
	if a.P.Titulo() != "Mis APIs" || a.P.Ayuda() != "conectar" || a.P.EnCampo() {
		t.Fatalf("título %q, ayuda %q", a.P.Titulo(), a.P.Ayuda())
	}

	// Crear: n abre el formulario, el nombre se valida y Ctrl+S la crea.
	a.Pulsar("n")
	if !a.P.EnCampo() || !strings.Contains(a.Vista(), "Nombre") {
		t.Fatalf("n abre el formulario:\n%s", a.Vista())
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
	a.Pulsar("ctrl+s")
	v := a.Vista()
	clave := reClave.FindString(v)
	if !strings.Contains(v, "API lista") || clave == "" || !strings.Contains(v, "Copia la clave ahora") {
		t.Fatalf("al crearla se abre el detalle con la clave, que solo se ve esta vez:\n%s", v)
	}
	if _, ok := punto(t, cli, id, "mi-app"); !ok {
		t.Fatal("la API está en el servicio")
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
	if p, _ := punto(t, cli, id, "mi-app"); p.Activo {
		t.Fatal("a la detiene en el servicio")
	}
	if v := a.Vista(); !strings.Contains(v, "Detenida") {
		t.Fatalf("el detalle dice Detenida:\n%s", v)
	}
	a.Pulsar("a")
	if p, _ := punto(t, cli, id, "mi-app"); !p.Activo {
		t.Fatal("a la vuelve a activar")
	}

	// Regenerar la clave: pregunta y enseña la nueva una vez.
	q, ok = pregunta(a.Pulsar("t"))
	if !ok || !strings.Contains(q.Texto, "¿Regenerar la clave de mi-app?") {
		t.Fatalf("t pregunta antes de regenerar: %+v", q)
	}
	a.Entregar(comp.Confirmado{})
	if nueva := reClave.FindString(a.Vista()); nueva == "" || nueva == clave {
		t.Fatalf("la clave nueva se ve: %q (antes %q)\n%s", nueva, clave, a.Vista())
	}

	// Eliminar: pregunta, recuerda cuál y cierra el detalle.
	q, ok = pregunta(a.Pulsar("x"))
	if !ok || !strings.Contains(q.Texto, "Eliminar mi-app") {
		t.Fatalf("x pregunta antes de eliminar: %+v", q)
	}
	a.Entregar(comp.Confirmado{})
	if _, ok := punto(t, cli, id, "mi-app"); ok {
		t.Fatal("la API ya no está en el servicio")
	}
	if v := a.Vista(); !strings.Contains(v, "Todavía no hay APIs por aplicación") {
		t.Fatalf("vuelve a la lista vacía:\n%s", v)
	}
}

// TestApisFiltrosYGrupos: los grupos plegables de la Web UI (los sin grupo al final), la búsqueda, los filtros de grupo y
// estado, quitar filtros y g, que activa o detiene el grupo entero.
func TestApisFiltrosYGrupos(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, contrato.ConfigPunto{Nombre: "mi-app", Grupo: "web"})
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

	// g en la cabecera de web: todas activas, así que las detiene; otra vez, las activa.
	a.Pulsar("home", "g")
	for _, n := range []string{"mi-app", "otra"} {
		if p, _ := punto(t, cli, id, n); p.Activo {
			t.Fatalf("g detiene %s", n)
		}
	}
	if p, _ := punto(t, cli, id, "notas"); p.Activo {
		t.Fatal("g no toca otros grupos")
	}
	a.Pulsar("down", "g") // desde una de sus APIs también vale
	for _, n := range []string{"mi-app", "otra"} {
		if p, _ := punto(t, cli, id, n); !p.Activo {
			t.Fatalf("g activa %s", n)
		}
	}
}

// TestEditarPresupuesto: e abre los ajustes de la API; el presupuesto del día y del mes se guarda con CambiarPunto y,
// vacío, se quita.
func TestEditarPresupuesto(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, contrato.ConfigPunto{Nombre: "mi-app", Instrucciones: "Sé breve."})
	a := abierta(t, cli, id)
	a.Pulsar("e")
	if v := a.Vista(); !strings.Contains(v, "Ajustes de mi-app") || !strings.Contains(v, "breve.") {
		t.Fatalf("e abre los ajustes con lo que tiene:\n%s", v)
	}
	a.Pulsar("tab", "tab", "tab", "tab", "tab") // instrucciones, modelo, forzar, esfuerzo y permitidos
	a.Escribir("50")
	a.Pulsar("tab", "tab")
	a.Escribir("2,5")
	a.Pulsar("ctrl+s")
	p, _ := punto(t, cli, id, "mi-app")
	if p.Presupuesto == nil || p.Presupuesto.Diario.Peticiones != 50 || p.Presupuesto.Diario.USD != 2.5 || p.Instrucciones != "Sé breve." {
		t.Fatalf("el presupuesto se guarda y lo demás sigue: %+v", p)
	}
	if a.P.EnCampo() {
		t.Fatal("al guardar se cierra el formulario")
	}

	// Un USD que no es un número no se envía.
	a.Pulsar("e", "tab", "tab", "tab", "tab", "tab", "tab", "tab")
	a.Escribir("x")
	a.Pulsar("ctrl+s")
	if !a.P.EnCampo() {
		t.Fatal("un USD no válido no se envía")
	}
	a.Pulsar("esc")
	a.Entregar(comp.Confirmado{}) // cerrar sin guardar

	// Vaciarlo lo quita.
	a.Pulsar("e", "tab", "tab", "tab", "tab", "tab", "backspace", "backspace", "tab", "tab")
	for range 3 {
		a.Pulsar("backspace")
	}
	a.Pulsar("ctrl+s")
	if p, _ := punto(t, cli, id, "mi-app"); p.Presupuesto != nil {
		t.Fatalf("vacío se quita: %+v", p.Presupuesto)
	}
}

// TestConsumoComoLaWeb: las barras del consumo salen de los mismos campos de /api/v1/puntos y con las mismas cuentas que
// BarrasPresupuesto.vue: una barra por límite definido (día y mes; peticiones, tokens y USD), «usado / límite» y la
// fracción usada.
func TestConsumoComoLaWeb(t *testing.T) {
	e := nucleo.EstadoPresupuesto{
		Diario: nucleo.EstadoPeriodo{Limites: nucleo.Limites{Peticiones: 10, USD: 2},
			Consumo: nucleo.Consumo{Peticiones: 8, Tokens: 999, USD: 0.5}},
		Mensual: nucleo.EstadoPeriodo{Limites: nucleo.Limites{Tokens: 20000}, Consumo: nucleo.Consumo{Tokens: 12345}},
	}
	quiere := [][3]string{{"Diario · Peticiones", "8 / 10", "0.80"}, {"Diario · USD", "$0.50 / $2.00", "0.25"},
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
	a.Entregar(tea.WindowSizeMsg{Width: 60, Height: 40})
	a.Pulsar("enter")
	v := a.Vista()
	for _, s := range []string{"Diario · Peticiones", "2 / 10", "Mensual · USD", "$0.00 / $2.00", "Se reinicia el día", "2 peticiones en"} {
		if !strings.Contains(v, s) {
			t.Fatalf("falta %q en el detalle:\n%s", s, v)
		}
	}
	if strings.Contains(v, "Diario · Tokens") {
		t.Fatalf("sin límite no hay barra:\n%s", v)
	}

	// R reinicia el consumo, con pregunta.
	q, ok := pregunta(a.Pulsar("R"))
	if !ok || !strings.Contains(q.Texto, "Reiniciar el consumo de mi-app") {
		t.Fatalf("R pregunta: %+v", q)
	}
	if av, ok := aviso(a.Entregar(comp.Confirmado{})); !ok || av.Texto != "Consumo reiniciado" {
		t.Fatalf("avisa: %+v", av)
	}
	if v := a.Vista(); !strings.Contains(v, "0 / 10") {
		t.Fatalf("el consumo queda a cero:\n%s", v)
	}
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

// TestClaveDelEspacio: la segunda pestaña, como en la Web UI: las URL del espacio, la clave oculta que se ve con v, se
// copia con y y se regenera con t (con pregunta), y el presupuesto del espacio con su reinicio.
func TestClaveDelEspacio(t *testing.T) {
	cli, id := conPresupuestoDelEspacio(t)
	a := abierta(t, cli, id)
	a.Entregar(tea.WindowSizeMsg{Width: 70, Height: 40})
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
	if av, ok := aviso(a.Entregar(comp.Confirmado{})); !ok || !strings.Contains(av.Texto, "Clave regenerada") {
		t.Fatalf("avisa: %+v", av)
	}
	nueva, _ := cli.ClaveInstancia(context.Background(), id)
	if nueva == clave || !strings.Contains(a.Vista(), nueva) {
		t.Fatalf("se ve la clave nueva %q:\n%s", nueva, a.Vista())
	}

	q, ok = pregunta(a.Pulsar("R"))
	if !ok || !strings.Contains(q.Texto, "Reiniciar el consumo del espacio") {
		t.Fatalf("R pregunta: %+v", q)
	}
	if av, ok := aviso(a.Entregar(comp.Confirmado{})); !ok || av.Texto != "Consumo reiniciado" {
		t.Fatalf("avisa: %+v", av)
	}
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
	sin := prueba.Conducir(t, conectar.Nueva("es"))
	if f := sin.Correr(sin.P.Iniciar(contexto(t, nil, "global"))); len(f) != 1 {
		t.Fatalf("sin cliente: ErrorServicio: %+v", f)
	}
	cli, id, cerrar := prueba.Arrancar(t)
	a := abierta(t, cli, id)
	cerrar()
	f := a.Pulsar("r")
	if len(f) != 1 {
		t.Fatalf("con el servicio cerrado: ErrorServicio, y la página no se cierra: %+v", f)
	}
	if v := a.Vista(); !strings.Contains(v, "El servicio no responde · r reintenta") {
		t.Fatalf("la página lo dice:\n%s", v)
	}
}

// TestErrorDeCarga: si la lista no llega, se dice con el mensaje del servicio y r reintenta, sin invitar a crear la primera.
func TestErrorDeCarga(t *testing.T) {
	cli, id := prueba.Servicio(t)
	proxy, fallar := prueba.ConFallo(t, cli)
	fallar("la configuración no se pudo leer")
	a := abierta(t, proxy, id)
	v := a.Vista()
	if !strings.Contains(v, "No se pudo cargar: la configuración no se pudo leer") || strings.Contains(v, "Todavía no hay APIs") {
		t.Fatalf("el error de carga:\n%s", v)
	}
	fallar("")
	a.Pulsar("r")
	if v := a.Vista(); !strings.Contains(v, "Todavía no hay APIs") {
		t.Fatalf("r reintenta:\n%s", v)
	}
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
