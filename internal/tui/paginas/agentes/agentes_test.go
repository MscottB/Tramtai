package agentes_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/adaptadores"
	"tramtai/internal/cliente"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/agentes"
	"tramtai/internal/tui/paginas/prueba"
)

// Las pruebas de Agentes van por su interfaz, con el arnés de las páginas (prueba.App): teclas, los mensajes de pagina y
// comp, y lo que enseña Vista(). Lo que la página cambia en el servicio se lee en el servicio.

var (
	aviso    = prueba.Aviso
	contexto = prueba.Contexto
)

// abrir inicia Agentes con ese contexto y espera la comprobación.
func abrir(t *testing.T, c pagina.Contexto) *prueba.App {
	t.Helper()
	return prueba.Abrir(t, agentes.Nueva(c.Idioma), c)
}

// abierta inicia la página contra el servicio real y espera la lista.
func abierta(t *testing.T) (*prueba.App, *cliente.Cliente, string) {
	t.Helper()
	cli, id := prueba.Servicio(t)
	a := prueba.Conducir(t, agentes.Nueva("es"))
	cmd := a.P.Iniciar(contexto(t, cli, id))
	if v := a.Vista(); !strings.Contains(v, "Cargando…") {
		t.Fatalf("arranca con «Cargando…»:\n%s", v)
	}
	a.Correr(cmd)
	a.EsperarCarga()
	return a, cli, id
}

// conAgentes abre la página contra un servicio con estos agentes además del simulado, ya comprobados: los estados los
// calcula el servicio y la página los pinta.
func conAgentes(t *testing.T, mas map[string]adaptadores.Adaptador) (*prueba.App, *cliente.Cliente, string) {
	t.Helper()
	cli, id := prueba.ConAgentes(t, mas)
	return abrir(t, contexto(t, cli, id)), cli, id
}

// detectado es un agente que se detecta así; sinCLI, uno cuyo CLI no está.
func detectado(e adaptadores.Estado) adaptadores.Adaptador { return prueba.Detectado{Estado: e} }

var sinCLI = detectado(adaptadores.Estado{Problema: "no está instalado", ProblemaCodigo: adaptadores.ProblemaNoInstalado})

// pregunta es la primera pregunta entre lo que salió hacia la app.
func pregunta(fuera []tea.Msg) (comp.Pregunta, bool) {
	for _, m := range fuera {
		if q, ok := m.(comp.Pregunta); ok {
			return q, true
		}
	}
	return comp.Pregunta{}, false
}

func TestAgentesCarga(t *testing.T) {
	a, _, _ := abierta(t)
	v := a.Vista()
	for _, quiere := range []string{"simulado", "Listo", "Listos (1)", "Falta conectar (0)"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	if a.P.Titulo() != "Agentes" || a.P.Ayuda() != "agentes" || a.P.EnCampo() {
		t.Fatalf("título %q, ayuda %q", a.P.Titulo(), a.P.Ayuda())
	}
}

// TestColorDeLinea: con color, las dos letras de cada agente van en el color de su línea (DESIGN.md: un color de línea por
// agente); sin color quedan las dos letras.
func TestColorDeLinea(t *testing.T) {
	cli, id := prueba.ConAgentes(t, map[string]adaptadores.Adaptador{"claude": detectado(adaptadores.Estado{})})
	c := contexto(t, cli, id)
	c.Estilos = &pagina.Estilos{Iconos: pagina.IconosUnicode, Color: true, Oscuro: true}
	a := abrir(t, c)
	a.Pulsar("down") // la fila elegida va resaltada entera; Claude queda sin elegir
	if v := a.Vista(); !strings.Contains(v, "CL") || !strings.Contains(v, "242;138;79") {
		t.Fatalf("el código va en su color; de noche, linea-claude-noche (#f28a4f): %q", v)
	}
}

// TestFiltroDeAgentes: la línea de los estados es el filtro de la Web UI; ← → lo cambia y la lista enseña solo esos.
func TestFiltroDeAgentes(t *testing.T) {
	a, _, _ := abierta(t)
	if v := a.Vista(); !strings.Contains(v, "[Todos (1)]") {
		t.Fatalf("empieza con todos:\n%s", v)
	}
	a.Pulsar("right")
	if v := a.Vista(); !strings.Contains(v, "[Listos (1)]") || !strings.Contains(v, "simulado") {
		t.Fatalf("→ deja los listos:\n%s", v)
	}
	a.Pulsar("right")
	if v := a.Vista(); !strings.Contains(v, "[Falta conectar (0)]") || strings.Contains(v, "simulado") {
		t.Fatalf("→ deja los que falta conectar:\n%s", v)
	}
	a.Pulsar("left", "left", "left")
	if v := a.Vista(); !strings.Contains(v, "[No instalados (0)]") {
		t.Fatalf("← da la vuelta:\n%s", v)
	}
	var pie []string
	for _, k := range a.P.Teclas() {
		pie = append(pie, k.Tecla+" "+k.Accion)
	}
	if !strings.Contains(strings.Join(pie, " · "), "← → filtro") {
		t.Fatalf("el pie dice cómo se filtra: %v", pie)
	}
}

func TestAgentesDesconectar(t *testing.T) {
	a, cli, id := abierta(t)
	fuera := a.Pulsar("d")
	if len(fuera) != 1 {
		t.Fatalf("d pregunta antes de desconectar: %v", fuera)
	}
	if q, ok := pregunta(fuera); !ok || !strings.Contains(q.Texto, "¿Desconectar simulado?") {
		t.Fatalf("la pregunta nombra al agente: %v", fuera[0])
	}
	// Cancelar no toca nada.
	a.Entregar(comp.Cancelado{})
	if r, _ := cli.Capacidades(context.Background(), id); r.Adaptadores[0].Desconectado {
		t.Fatal("cancelar no desconecta")
	}
	// Confirmar desconecta en el servicio y la página lo ve al volver a comprobar.
	a.Pulsar("d")
	fuera = a.Entregar(comp.Confirmado{})
	if av, ok := aviso(fuera); !ok || av.Texto != "simulado está desconectado" || av.Tipo != comp.Exito {
		t.Fatalf("avisa: %+v", fuera)
	}
	r, _ := cli.Capacidades(context.Background(), id)
	if !r.Adaptadores[0].Desconectado {
		t.Fatal("el servicio lo tiene desconectado")
	}
	if v := a.Vista(); !strings.Contains(v, "Desconectado") || strings.Contains(v, "Listo ") {
		t.Fatalf("la lista dice Desconectado:\n%s", v)
	}
	// Conectar no pide confirmación.
	fuera = a.Pulsar("d")
	if av, ok := aviso(fuera); !ok || av.Texto != "simulado está conectado" {
		t.Fatalf("conecta sin preguntar: %+v", fuera)
	}
	if v := a.Vista(); !strings.Contains(v, "Listo") {
		t.Fatalf("vuelve a Listo:\n%s", v)
	}
}

func TestAgentesAutenticacionSoloSiAdmite(t *testing.T) {
	a, _, _ := abierta(t)
	// El simulado no admite la suscripción: a avisa y no pregunta ni cambia nada.
	fuera := a.Pulsar("a")
	if av, ok := aviso(fuera); !ok || !strings.Contains(av.Texto, "no admite la suscripción") || len(fuera) != 1 {
		t.Fatalf("avisa que no admite: %+v", fuera)
	}
	if fuera := a.Entregar(comp.Confirmado{}); len(fuera) != 0 {
		t.Fatalf("no queda nada pendiente que confirmar: %v", fuera)
	}

	// Claude sí la admite: con clave de API y listo (va primero), pasar a la suscripción pide aceptar el riesgo y, al
	// confirmar, lo pide al servicio.
	a, _, _ = conAgentes(t, map[string]adaptadores.Adaptador{"claude": detectado(adaptadores.Estado{Autenticacion: "clave_api"})})
	fuera = a.Pulsar("a")
	q, ok := pregunta(fuera)
	if len(fuera) != 1 || !ok || !strings.Contains(q.Texto, "Ese riesgo es tuyo") || !strings.Contains(q.Texto, "Usar tu suscripción con Claude") {
		t.Fatalf("pregunta con el riesgo: %+v", fuera)
	}
	if _, ok := aviso(a.Entregar(comp.Confirmado{})); !ok {
		t.Fatal("recuerda qué iba a hacer: al confirmar lo hace y avisa")
	}

	// Con suscripción, pasar a la clave es directo, sin pregunta.
	a, _, _ = conAgentes(t, map[string]adaptadores.Adaptador{"claude": detectado(adaptadores.Estado{Autenticacion: "suscripcion"})})
	fuera = a.Pulsar("a")
	if av, ok := aviso(fuera); !ok || len(fuera) != 1 || av.Texto == "" {
		t.Fatalf("pasar a la clave sale directo y avisa: %+v", fuera)
	}

	// Sin instalar o desconectado, no se cambia aunque lo admita.
	sinInstalar, _, _ := conAgentes(t, map[string]adaptadores.Adaptador{"claude": sinCLI})
	desconectado, cli, _ := conAgentes(t, map[string]adaptadores.Adaptador{"claude": detectado(adaptadores.Estado{Autenticacion: "clave_api"})})
	if err := cli.ConectarAgente(context.Background(), "claude", false); err != nil {
		t.Fatal(err)
	}
	desconectado.Pulsar("c") // vuelve a comprobar
	for nombre, a := range map[string]*prueba.App{"sin instalar": sinInstalar, "desconectado": desconectado} {
		a.Pulsar("down") // el simulado, listo, va primero
		fuera = a.Pulsar("a")
		if _, ok := aviso(fuera); !ok || len(fuera) != 1 {
			t.Fatalf("%s no se cambia: %+v", nombre, fuera)
		}
		if fuera := a.Entregar(comp.Confirmado{}); len(fuera) != 0 {
			t.Fatalf("%s: no queda nada pendiente: %v", nombre, fuera)
		}
	}
}

func TestAgentesProbarConEste(t *testing.T) {
	a, _, _ := abierta(t)
	fuera := a.Pulsar("p")
	if len(fuera) != 1 || fuera[0] != (pagina.IrA{Pagina: "ejecutar", Datos: pagina.ConAgente{Agente: "simulado"}}) {
		t.Fatalf("p lleva a Probar con el agente: %+v", fuera)
	}
	// Uno que no está listo no se puede probar.
	a, _, _ = conAgentes(t, map[string]adaptadores.Adaptador{"codex": sinCLI})
	a.Pulsar("down")
	if _, ok := aviso(a.Pulsar("p")); !ok {
		t.Fatal("avisa que aún no está listo")
	}
}

func TestAgentesDetalles(t *testing.T) {
	a, _, _ := abierta(t)
	catalogo := a.Retener(func() { a.Pulsar("enter") })
	if v := a.Vista(); !strings.Contains(v, "simulado") || !strings.Contains(v, "Versión") || !strings.Contains(v, "Cargando…") {
		t.Fatalf("detalle con el catálogo pendiente:\n%s", v)
	}
	a.Soltar(catalogo)
	v := a.Vista()
	for _, quiere := range []string{"Listo · usa tu clave de API", "Modelos: predeterminado", "Respuesta en vivo: Al terminar", "Respuesta JSON con esquema: No"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	// Las acciones valen dentro del detalle: p lleva a Probar con este agente.
	if f := a.Pulsar("p"); len(f) != 1 || f[0] != (pagina.IrA{Pagina: "ejecutar", Datos: pagina.ConAgente{Agente: "simulado"}}) {
		t.Fatalf("p en el detalle: %+v", f)
	}
}

func TestAgentesNoInstaladoExplicaComoInstalarlo(t *testing.T) {
	a, _, _ := conAgentes(t, map[string]adaptadores.Adaptador{"codex": sinCLI, "claude": detectado(adaptadores.Estado{})})
	if v := a.Vista(); strings.Index(v, "Claude") > strings.Index(v, "Codex") || !strings.Contains(v, "No instalado") {
		t.Fatalf("los no instalados van al final:\n%s", v)
	}
	a.Pulsar("down", "down", "enter")
	v := a.Vista()
	if !strings.Contains(v, "comandos_agentes.codex") || !strings.Contains(v, "Cómo instalarlo: http") {
		t.Fatalf("el detalle dice cómo instalarlo:\n%s", v)
	}
}

func TestEscVuelve(t *testing.T) {
	a, _, _ := abierta(t)
	a.Pulsar("enter")
	if f := a.Pulsar("esc"); len(f) != 0 || !strings.Contains(a.Vista(), "[Todos (1)]") {
		t.Fatalf("Esc cierra el detalle: %v", f)
	}
	if f := a.Pulsar("esc"); len(f) != 1 || f[0] != (pagina.Volver{}) {
		t.Fatalf("sin nada abierto, Esc vuelve: %+v", f)
	}
}

func TestAgentesServicioCaido(t *testing.T) {
	sin := prueba.Conducir(t, agentes.Nueva("es"))
	if f := sin.Correr(sin.P.Iniciar(contexto(t, nil, "global"))); len(f) != 1 {
		t.Fatalf("sin cliente: ErrorServicio: %+v", f)
	}
	cli, id, cerrar := prueba.Arrancar(t)
	a := abrir(t, contexto(t, cli, id))
	cerrar()
	f := a.Pulsar("r")
	if len(f) != 1 {
		t.Fatalf("con el servicio cerrado: ErrorServicio, y la página no se cierra: %+v", f)
	}
	if e, ok := f[0].(pagina.ErrorServicio); !ok || !pagina.NoResponde(e.Err) {
		t.Fatalf("error de conexión: %+v", f[0])
	}
	if v := a.Vista(); !strings.Contains(v, "El servicio no responde · r reintenta") {
		t.Fatalf("la página lo dice:\n%s", v)
	}
}

func TestAgentesIgnoraRespuestasViejas(t *testing.T) {
	cli, id := prueba.Servicio(t)
	ctx, cancelar := context.WithCancel(context.Background())
	c := contexto(t, cli, id)
	c.Ctx = ctx
	a := prueba.Conducir(t, agentes.Nueva("es"))
	cmd := a.P.Iniciar(c)
	cancelar() // la app reinició la página o cambió de espacio
	cargando := func() bool { return strings.Contains(a.Vista(), "Cargando…") }
	if f := a.Correr(cmd); len(f) != 0 || !cargando() {
		t.Fatalf("una respuesta con el Ctx cancelado se descarta: %+v", f)
	}
	// Otra instancia: tampoco vale.
	a = prueba.Conducir(t, agentes.Nueva("es"))
	cmd = a.P.Iniciar(contexto(t, cli, id))
	a.P.Iniciar(contexto(t, cli, "otro")) // la app cambió de espacio; su comprobación no se corre
	if a.Correr(cmd); !cargando() {
		t.Fatal("una respuesta de otro espacio se descarta")
	}
	// De dos comprobaciones seguidas solo vale la última.
	a = prueba.Conducir(t, agentes.Nueva("es"))
	primera := a.P.Iniciar(contexto(t, cli, id))
	segunda := a.P.Iniciar(contexto(t, cli, id))
	if a.Correr(primera); !cargando() {
		t.Fatal("la primera ya es vieja")
	}
	if a.Correr(segunda); cargando() {
		t.Fatal("la última sí")
	}
}

// TestAgentesPintaElEstadoDelServicio: la página no clasifica a los agentes; pinta la etiqueta del estado que llega.
func TestAgentesPintaElEstadoDelServicio(t *testing.T) {
	casos := map[adaptadores.Estado]string{
		{}: "Listo",
		{Problema: "p", ProblemaCodigo: adaptadores.ProblemaAviso}:                                    "Listo",
		{Problema: "p", ProblemaCodigo: adaptadores.ProblemaNoInstalado}:                              "No instalado",
		{Problema: "p", ProblemaCodigo: adaptadores.ProblemaFaltaClave, Autenticacion: "clave_api"}:   "Sin conectar",
		{Problema: "p", ProblemaCodigo: adaptadores.ProblemaFaltaClave, Autenticacion: "suscripcion"}: "Sin conectar",
		{Problema: "p", ProblemaCodigo: adaptadores.ProblemaUsaClave}:                                 "Revisar",
		{Problema: "p"}: "Revisar",
	}
	for e, etiqueta := range casos {
		a, _, _ := conAgentes(t, map[string]adaptadores.Adaptador{"claude": detectado(e)})
		if v := a.Vista(); !strings.Contains(v, "Claude") || !strings.Contains(v, etiqueta) {
			t.Errorf("%+v: falta %q:\n%s", e, etiqueta, v)
		}
	}
}

// TestAgentesConErrorCabe: un error largo del servicio ocupa sus líneas y la lista se acorta lo que haga falta: la vista
// cabe en el área de 60×20 y la fila elegida, al final de la lista, se ve.
func TestAgentesConErrorCabe(t *testing.T) {
	mas := map[string]adaptadores.Adaptador{}
	for i := range 12 {
		mas[fmt.Sprintf("agente%02d", i)] = detectado(adaptadores.Estado{})
	}
	cli, id := prueba.ConAgentes(t, mas)
	proxy, fallar := prueba.ConFallo(t, cli)
	a := abrir(t, contexto(t, proxy, id))
	a.Pulsar("end")
	fallar(strings.Repeat("El servicio no pudo comprobar los agentes de este espacio. ", 4))
	a.Pulsar("c")
	v := a.Vista()
	if h := lipgloss.Height(v); h > 17 {
		t.Fatalf("la vista mide %d filas y el área es de 17:\n%s", h, v)
	}
	if !strings.Contains(v, "de este espacio.") || !strings.Contains(v, "▸") {
		t.Fatalf("se ven el error entero y la fila elegida:\n%s", v)
	}
}

// TestAgentesCabe comprueba que la vista, también la del detalle, cabe en la ventana mínima y en una estrecha.
func TestAgentesCabe(t *testing.T) {
	a, _, _ := conAgentes(t, map[string]adaptadores.Adaptador{"claude": detectado(adaptadores.Estado{Autenticacion: "suscripcion", Version: "2.1.0"}),
		"codex": detectado(adaptadores.Estado{Problema: "Codex no está instalado", ProblemaCodigo: adaptadores.ProblemaNoInstalado})})
	for _, tam := range [][2]int{{60, 17}, {40, 10}} {
		a.Entregar(tea.WindowSizeMsg{Width: tam[0], Height: tam[1]})
		comprobar := func(donde string) {
			v := a.Vista()
			if lipgloss.Height(v) > tam[1] || lipgloss.Width(v) > tam[0] {
				t.Errorf("%s a %dx%d mide %dx%d:\n%s", donde, tam[0], tam[1], lipgloss.Width(v), lipgloss.Height(v), v)
			}
		}
		comprobar("lista")
		a.Pulsar("enter")
		comprobar("detalle")
		a.Pulsar("esc")
	}
}
