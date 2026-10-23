package ayuda_test

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/ayuda"
	"tramtai/internal/tui/paginas/prueba"
)

// sinNavegador es lo que responde abrir una URL donde no hay navegador.
func sinNavegador(string) error { return errors.New("no hay navegador") }

// abierta abre la Ayuda sin servicio (no lo necesita) y sin navegador: ninguna prueba abre uno de verdad.
func abierta(t *testing.T, lengua string) *prueba.App {
	t.Helper()
	ayuda.ConNavegador(t, sinNavegador)
	c := prueba.Contexto(t, nil, "global")
	c.Idioma = lengua
	return prueba.Abrir(t, ayuda.Nueva(lengua), c)
}

func vista(a *prueba.App) string { return ansi.Strip(a.Vista()) }

func primeraLinea(a *prueba.App) string { return strings.TrimSpace(strings.Split(vista(a), "\n")[0]) }

// temaDe es el tema de la Ayuda que acompaña a una página de la app (su campo pagina).
func temaDe(t *testing.T, lengua, clave string) datos.Tema {
	t.Helper()
	i := slices.IndexFunc(datos.Ayuda(lengua), func(x datos.Tema) bool { return x.Pagina == clave })
	if i < 0 {
		t.Fatalf("no hay tema para %s", clave)
	}
	return datos.Ayuda(lengua)[i]
}

// TestAyudaBuscaSinTildes: / busca por palabras, como la Web UI: sin tildes ni mayúsculas, en cualquier orden, con el guion
// bajo de los códigos como espacio. Intro deja la búsqueda puesta y otro Intro abre el tema.
func TestAyudaBuscaSinTildes(t *testing.T) {
	a := abierta(t, "es")
	if v := vista(a); !strings.Contains(v, "Empezar") || !strings.Contains(v, "Presupuestos") {
		t.Fatalf("enseña los temas:\n%s", v)
	}
	a.Pulsar("/")
	if !a.P.EnCampo() {
		t.Fatal("la búsqueda es un campo: q y los números se escriben")
	}
	a.Escribir("DÓLARES")
	conTilde := vista(a)
	a.Pulsar("esc")
	a.Pulsar("/")
	a.Escribir("dolares")
	if sin := vista(a); !strings.Contains(sin, "Presupuestos") || strings.Contains(sin, "Agentes y modelos") {
		t.Fatalf("encuentra «dólares» sin tilde y deja fuera lo demás:\n%s", sin)
	} else if strings.ReplaceAll(conTilde, "DÓLARES", "") != strings.ReplaceAll(sin, "dolares", "") {
		t.Fatalf("con tilde y mayúsculas encuentra lo mismo:\n%s\n---\n%s", conTilde, sin)
	}
	a.Pulsar("esc")
	a.Pulsar("/")
	a.Escribir("llena COLA")
	if v := vista(a); !strings.Contains(v, "Algo falló") || strings.Contains(v, "Agentes y modelos") {
		t.Fatalf("las palabras en cualquier orden encuentran cola_llena:\n%s", v)
	}
	a.Pulsar("enter")
	if a.P.EnCampo() {
		t.Fatal("Intro deja la búsqueda puesta y suelta el campo")
	}
	a.Pulsar("enter")
	if l := primeraLinea(a); l != "Algo falló" {
		t.Fatalf("Intro abre el tema encontrado: %q", l)
	}
	a.Pulsar("esc", "esc", "/")
	a.Escribir("xyzzy")
	if v := vista(a); !strings.Contains(strings.Join(strings.Fields(v), " "), datos.Textof("es", "ayuda.sinResultados", "texto", "xyzzy")) {
		t.Fatalf("sin resultados lo dice:\n%s", v)
	}
}

// TestBuscarEmpiezaPorElMejor: tras buscar, el cursor está en el mejor resultado (la palabra en el título pesa más), no
// en la fila de antes.
func TestBuscarEmpiezaPorElMejor(t *testing.T) {
	a := abierta(t, "es")
	a.Pulsar("down", "down")
	a.Pulsar("/")
	a.Escribir("clave")
	a.Pulsar("enter", "enter")
	if l := primeraLinea(a); l != "Claves y APIs" {
		t.Fatalf("abre el mejor resultado: %q", l)
	}
}

// TestConAyudaAbreSuTema: el ? de cada página abre su tema (por su campo pagina, no por id: Probar es id probar y pagina
// ejecutar) con la línea «En la terminal» y todas las teclas de la página.
func TestConAyudaAbreSuTema(t *testing.T) {
	for _, clave := range []string{"inicio", "agentes", "ejecutar", "conectar", "historial", "espacios", "configuracion"} {
		a := abierta(t, "es")
		tm := temaDe(t, "es", clave)
		a.Entregar(pagina.ConAyuda{Pagina: clave, Teclas: []pagina.Tecla{{Tecla: "z", Accion: "acción de prueba"}}})
		if l := primeraLinea(a); l != tm.Titulo {
			t.Errorf("%s: abre «%s»: %q", clave, tm.Titulo, l)
		}
		v := vista(a)
		if !strings.HasPrefix(tm.Terminal, "En la terminal:") || !strings.Contains(v, "En la terminal:") {
			t.Errorf("%s: dice cómo se hace en la terminal (%q):\n%s", clave, tm.Terminal, v)
		}
		if !strings.Contains(v, "acción de prueba") {
			t.Errorf("%s: enseña las teclas de la página:\n%s", clave, v)
		}
	}
}

// TestAyudaVerEnLaWebSinNavegador: w abre el tema en el sitio. Sin navegador no falla: enseña la URL para copiarla, y y la
// copia. Con navegador, la abre y avisa.
func TestAyudaVerEnLaWebSinNavegador(t *testing.T) {
	a := abierta(t, "es")
	a.Entregar(pagina.ConAyuda{Pagina: "historial"})
	url := "https://mscottb.github.io/Tramtai/ayuda/historial.html"
	f := a.Pulsar("w")
	for _, m := range f {
		if _, caido := m.(pagina.ErrorServicio); caido {
			t.Fatalf("sin navegador no es un fallo del servicio: %+v", f)
		}
		if av, ok := m.(comp.NuevoAviso); ok && av.Tipo == comp.Fallo {
			t.Fatalf("sin navegador no hay error: %+v", av)
		}
	}
	if v := vista(a); !strings.Contains(v, url) {
		t.Fatalf("enseña la URL para copiarla:\n%s", v)
	}
	if f := a.Pulsar("y"); !slices.Contains(f, tea.Msg(prueba.Portapapeles(url))) {
		t.Fatalf("y copia la URL: %+v", f)
	}

	var mu sync.Mutex
	var pedida string
	ayuda.ConNavegador(t, func(u string) error { mu.Lock(); defer mu.Unlock(); pedida = u; return nil })
	f = a.Pulsar("w")
	mu.Lock()
	defer mu.Unlock()
	if pedida != url {
		t.Fatalf("abre el tema en el sitio: %q", pedida)
	}
	if _, ok := prueba.Aviso(f); !ok {
		t.Fatalf("avisa de que lo abrió: %+v", f)
	}
	// En Linux sin escritorio xdg-open arranca y falla después sin decir nada: la URL sigue a la vista.
	if v := vista(a); !strings.Contains(v, url) {
		t.Fatalf("la URL sigue a la vista por si no se abrió:\n%s", v)
	}
}

// TestSinResultadosVerEnLaWeb: sin resultados, la terminal dice qué hacer de verdad: w abre la Ayuda del sitio, con su
// buscador, o enseña su dirección.
func TestSinResultadosVerEnLaWeb(t *testing.T) {
	a := abierta(t, "es")
	a.Pulsar("/")
	a.Escribir("xyzzy")
	a.Pulsar("enter")
	if v := compacto(vista(a)); !strings.Contains(v, "w") || strings.Contains(v, compacto("mira la documentación completa")) {
		t.Fatalf("dice qué tecla usar en la terminal:\n%s", vista(a))
	}
	a.Pulsar("w")
	if v := vista(a); !strings.Contains(v, "https://mscottb.github.io/Tramtai/ayuda/") {
		t.Fatalf("sin navegador, enseña la dirección de la Ayuda del sitio:\n%s", v)
	}
}

// TestBusquedaLargaSeDesplaza: con una búsqueda más ancha que la ventana se ve lo último escrito y el cursor.
func TestBusquedaLargaSeDesplaza(t *testing.T) {
	a := abierta(t, "es")
	a.Pulsar("/")
	a.Escribir(strings.Repeat("palabra ", 10) + "final")
	if l := primeraLinea(a); !strings.HasSuffix(l, "final_") || lipgloss.Width(l) > 60 {
		t.Fatalf("se ve el final con el cursor: %q", l)
	}
}

// TestEscVuelve: Esc cierra lo abierto (la búsqueda, el tema abierto desde la lista) y, sin nada abierto, vuelve. El tema
// que abrió el ? de una página es la Ayuda de esa página: Esc vuelve a ella.
func TestEscVuelve(t *testing.T) {
	a := abierta(t, "es")
	a.Pulsar("enter")
	if l := primeraLinea(a); l != "Empezar" {
		t.Fatalf("Intro abre el tema: %q", l)
	}
	if f := a.Pulsar("esc"); len(f) != 0 || !strings.Contains(vista(a), "Presupuestos") {
		t.Fatalf("Esc vuelve a la lista: %+v\n%s", f, vista(a))
	}
	a.Pulsar("/")
	a.Escribir("x")
	if f := a.Pulsar("esc"); len(f) != 0 || a.P.EnCampo() {
		t.Fatalf("Esc cierra la búsqueda: %+v", f)
	}
	if f := a.Pulsar("esc"); len(f) != 1 || f[0] != (pagina.Volver{}) {
		t.Fatalf("sin nada abierto, Esc vuelve: %+v", f)
	}
	a.Entregar(pagina.ConAyuda{Pagina: "ejecutar"})
	if f := a.Pulsar("esc"); len(f) != 1 || f[0] != (pagina.Volver{}) {
		t.Fatalf("desde el ? de una página, Esc vuelve a ella: %+v", f)
	}
}

// cabe comprueba que la vista no pasa de 60×17, el área de la página en una terminal de 60×20.
func cabe(t *testing.T, a *prueba.App, donde string) {
	t.Helper()
	lineas := strings.Split(a.Vista(), "\n")
	if len(lineas) > 17 {
		t.Fatalf("%s: %d líneas:\n%s", donde, len(lineas), a.Vista())
	}
	for _, l := range lineas {
		if lipgloss.Width(l) > 60 {
			t.Fatalf("%s: una línea pasa de 60 columnas (%d):\n%s", donde, lipgloss.Width(l), a.Vista())
		}
	}
}

// TestCabeEn60x20: la lista, la búsqueda, cada tema (arriba y desplazado) y la URL sin navegador caben en el área de una
// terminal de 60×20, en los dos idiomas.
func TestCabeEn60x20(t *testing.T) {
	for _, lengua := range []string{"es", "en"} {
		a := abierta(t, lengua)
		c := prueba.Contexto(t, nil, "global")
		c.Idioma = lengua
		cabe(t, a, lengua+" lista")
		a.Pulsar("/")
		a.Escribir(strings.Repeat("palabra ", 12))
		cabe(t, a, lengua+" búsqueda")
		for i, tm := range datos.Ayuda(lengua) {
			a.Correr(a.P.Iniciar(c))
			for range i {
				a.Pulsar("down")
			}
			a.Pulsar("enter")
			if l := primeraLinea(a); l != tm.Titulo {
				t.Fatalf("%s: abre %q: %q", lengua, tm.Titulo, l)
			}
			cabe(t, a, lengua+" "+tm.ID)
			a.Pulsar("w")
			cabe(t, a, lengua+" "+tm.ID+" sin navegador")
			a.Pulsar("pgdown", "pgdown", "G")
			cabe(t, a, lengua+" "+tm.ID+" al final")
		}
	}
}

// compacto quita los espacios y los saltos: un texto partido en líneas (también tras un guion) sigue siendo el mismo.
func compacto(s string) string { return strings.Join(strings.Fields(s), "") }

// TestSinTextosDeLaWebUI (hallazgo 69): lo que solo vale en la Web UI (el recorrido por la pantalla, las capturas) no se
// pinta: queda «Ver en la web». Todo lo demás sí: los pasos, las preguntas y qué hacer con cada error.
func TestSinTextosDeLaWebUI(t *testing.T) {
	ayuda.ConNavegador(t, sinNavegador)
	c := prueba.Contexto(t, nil, "global")
	c.Ancho, c.Alto = 120, 2000 // cabe el tema entero
	for _, tm := range datos.Ayuda("es") {
		a := prueba.Abrir(t, ayuda.Nueva("es"), c)
		i := slices.IndexFunc(datos.Ayuda("es"), func(x datos.Tema) bool { return x.ID == tm.ID })
		for range i {
			a.Pulsar("down")
		}
		a.Pulsar("enter")
		v := compacto(vista(a))
		if !strings.Contains(v, compacto(tm.Terminal)) {
			t.Errorf("%s: falta la línea de la terminal %q", tm.ID, tm.Terminal)
		}
		for _, b := range tm.Bloques {
			switch b.Tipo {
			case "pantalla":
				for _, p := range b.Pares {
					if strings.Contains(v, compacto(p[1])) {
						t.Errorf("%s: pinta el recorrido de la Web UI: %q", tm.ID, p[1])
					}
				}
				if !strings.Contains(v, compacto(datos.Texto("es", "ayuda.ui.recorridoTitulo"))) || !strings.Contains(v, "wVerenlaweb") {
					t.Errorf("%s: el recorrido se ve en la web:\n%s", tm.ID, v)
				}
			case "preguntas", "errores":
				for _, p := range b.Pares {
					if !strings.Contains(v, compacto(p[0])) || !strings.Contains(v, compacto(p[1])) {
						t.Errorf("%s: falta %q", tm.ID, p[0])
					}
				}
			case "pasos":
				if !strings.Contains(v, compacto(b.Titulo)) { // la sección con título («Solo con la terminal»)
					t.Errorf("%s: falta el título %q", tm.ID, b.Titulo)
				}
				for _, l := range b.Lineas {
					if !strings.Contains(v, compacto(l)) {
						t.Errorf("%s: falta el paso %q", tm.ID, l)
					}
				}
			}
		}
	}
}
