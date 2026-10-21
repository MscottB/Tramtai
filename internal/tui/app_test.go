package tui

import (
	"image/color"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"tramtai/internal/idioma"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/prueba"
)

func tecla(texto string) tea.KeyPressMsg {
	switch texto {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "ctrl+e":
		return tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: []rune(texto)[0], Text: texto}
}

// correr ejecuta un Cmd y lo que vaya saliendo de él, como haría Bubble Tea, y dice si pidió salir. Un temporizador
// que no llega a tiempo se deja sin esperar.
func correr(t *testing.T, m *modelo, cmd tea.Cmd) bool {
	t.Helper()
	if cmd == nil {
		return false
	}
	hecho := make(chan tea.Msg, 1)
	go func() { hecho <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-hecho:
	case <-time.After(10 * time.Second):
		return false
	}
	switch msg := msg.(type) {
	case nil:
		return false
	case tea.QuitMsg:
		return true
	case tea.BatchMsg:
		salio := false
		for _, c := range msg {
			salio = correr(t, m, c) || salio
		}
		return salio
	}
	_, siguiente := m.Update(msg)
	return correr(t, m, siguiente)
}

func sale(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// paginaFalsa apunta lo que recibe; con campo, hace como si un campo de texto tuviera el foco.
type paginaFalsa struct {
	campo       bool
	texto       string
	iniciada    int
	ancho, alto int
}

func (p *paginaFalsa) Titulo() string { return "Configuración" }
func (p *paginaFalsa) Iniciar(c pagina.Contexto) tea.Cmd {
	p.iniciada++
	p.ancho, p.alto = c.Ancho, c.Alto
	return nil
}
func (p *paginaFalsa) Update(msg tea.Msg) (pagina.Pagina, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		p.texto += msg.Text
	case tea.WindowSizeMsg:
		p.ancho, p.alto = msg.Width, msg.Height
	}
	return p, nil
}
func (p *paginaFalsa) Vista() string          { return "texto: " + p.texto }
func (p *paginaFalsa) Teclas() []pagina.Tecla { return []pagina.Tecla{{Tecla: "a", Accion: "anota"}} }
func (p *paginaFalsa) EnCampo() bool          { return p.campo }
func (p *paginaFalsa) Ayuda() string          { return "configuracion" }

// TestMenuComoLaWeb lee web/src/menu.ts: la terminal tiene los mismos grupos y páginas, en el mismo orden, y Ayuda al final.
func TestMenuComoLaWeb(t *testing.T) {
	web, err := os.ReadFile("../../web/src/menu.ts")
	if err != nil {
		t.Fatal(err)
	}
	var grupos, orden []string
	for _, g := range regexp.MustCompile(`grupo: '(\w+)', paginas: \[([^\]]*)\]`).FindAllStringSubmatch(string(web), -1) {
		grupos = append(grupos, g[1])
		for _, p := range regexp.MustCompile(`'(\w+)'`).FindAllStringSubmatch(g[2], -1) {
			orden = append(orden, p[1])
		}
	}
	if len(orden) != 7 || len(grupos) != 3 {
		t.Fatalf("no se entendió menu.ts: %v %v", grupos, orden)
	}
	if esperado := append(orden, "ayuda"); !slices.Equal(ordenMenu(), esperado) {
		t.Fatalf("orden: %v, la web: %v", ordenMenu(), esperado)
	}
	var mios []string
	for _, g := range gruposMenu {
		mios = append(mios, g.ID)
	}
	if !slices.Equal(mios, grupos) {
		t.Fatalf("grupos: %v, la web: %v", mios, grupos)
	}
	vista := nuevo(Opciones{}).View().Content
	ultimo := -1
	for _, p := range ordenMenu() {
		i := strings.Index(vista, datos.Texto(idiomaDe(), "menu."+p))
		if i <= ultimo {
			t.Fatalf("%s fuera de orden en el menú:\n%s", p, vista)
		}
		ultimo = i
	}
}

func TestNombresComoLaWeb(t *testing.T) {
	for _, lengua := range []string{"es", "en"} {
		m := nuevoEn(Opciones{}, lengua)
		vista := m.View().Content
		for _, p := range ordenMenu() {
			nombre := datos.Texto(lengua, "menu."+p)
			if nombre == "menu."+p {
				t.Fatalf("%s: falta menu.%s en los datos de la Web UI", lengua, p)
			}
			if titulo := m.pagina(p).Titulo(); titulo != nombre {
				t.Errorf("%s: la página %s se titula %q y en la web %q", lengua, p, titulo, nombre)
			}
			if !strings.Contains(vista, nombre) {
				t.Errorf("%s: el menú no dice %q:\n%s", lengua, nombre, vista)
			}
		}
		for _, g := range gruposMenu {
			if nombre := datos.Texto(lengua, "menu.grupo."+g.ID); !strings.Contains(vista, nombre) {
				t.Errorf("%s: el menú no dice el grupo %q", lengua, nombre)
			}
		}
	}
}

func TestAtajosNumericos(t *testing.T) {
	m := nuevo(Opciones{})
	for i, p := range ordenMenu()[:7] {
		m.Update(tecla(strconv.Itoa(i + 1)))
		if m.actual != p {
			t.Fatalf("%d lleva a %s, no a %s", i+1, m.actual, p)
		}
		if cabecera := strings.Split(m.View().Content, "\n")[0]; !strings.Contains(cabecera, m.pagina(p).Titulo()) {
			t.Fatalf("la cabecera no dice %q: %s", m.pagina(p).Titulo(), cabecera)
		}
	}
	m.Update(tecla("8"))
	m.Update(tecla("0"))
	if m.actual != "configuracion" {
		t.Fatalf("8 y 0 no son atajos: %s", m.actual)
	}
}

func TestTamanoMinimo(t *testing.T) {
	aviso := idioma.T("Agranda la ventana (mínimo 60×20)")
	m := nuevo(Opciones{})
	for _, tam := range [][2]int{{59, 20}, {60, 19}, {20, 5}} {
		m.Update(tea.WindowSizeMsg{Width: tam[0], Height: tam[1]})
		if vista := strings.TrimSpace(ansi.Strip(m.View().Content)); vista != aviso {
			t.Fatalf("%v debía ser solo el aviso:\n%s", tam, vista)
		}
	}
	if _, cmd := m.Update(tecla("q")); !sale(cmd) {
		t.Fatal("con la ventana pequeña q sigue saliendo")
	}
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	if vista := m.View().Content; strings.Contains(vista, aviso) || !strings.Contains(vista, m.pagina("inicio").Titulo()) {
		t.Fatalf("60×20 ya cabe:\n%s", vista)
	}
}

func TestRedimensionarConservaEstado(t *testing.T) {
	m := nuevo(Opciones{})
	falsa := &paginaFalsa{}
	m.paginas["historial"] = falsa
	m.Update(tecla("5"))
	m.Update(tecla("a"))
	m.Update(tecla("b"))
	m.Update(tecla("tab"))
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.actual != "historial" || m.paginas["historial"] != falsa || falsa.iniciada != 1 || !m.enMenu {
		t.Fatalf("al volver a crecer todo sigue igual: %s %d %v", m.actual, falsa.iniciada, m.enMenu)
	}
	ancho, alto := m.area()
	if falsa.ancho != ancho || falsa.alto != alto || ancho >= 120 || alto >= 40 {
		t.Fatalf("la página recibe el tamaño de su área: %d×%d, área %d×%d", falsa.ancho, falsa.alto, ancho, alto)
	}
	if vista := m.View().Content; !strings.Contains(vista, "texto: ab") {
		t.Fatalf("la página conserva lo escrito:\n%s", vista)
	}
}

func TestMenuPlegadoBajo100(t *testing.T) {
	m := nuevo(Opciones{})
	grupo := datos.Texto(m.idioma, "menu.grupo.organizar")
	m.Update(tea.WindowSizeMsg{Width: 99, Height: 30})
	if strings.Contains(m.View().Content, grupo) {
		t.Fatal("con 99 columnas el menú va plegado")
	}
	m.Update(tecla("tab"))
	if !strings.Contains(m.View().Content, grupo) {
		t.Fatal("Tab abre el menú")
	}
	m.Update(tecla("down"))
	m.Update(tecla("enter"))
	if m.actual != "agentes" || strings.Contains(m.View().Content, grupo) {
		t.Fatalf("Intro elige y pliega el menú: %s", m.actual)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if !strings.Contains(m.View().Content, grupo) {
		t.Fatal("con 100 columnas el menú se ve siempre")
	}
}

func TestServicioCaidoNoCierra(t *testing.T) {
	cli, global, cerrar := prueba.Arrancar(t)
	m := nuevo(Opciones{})
	m.cli = cli
	if correr(t, m, m.conectar(false, true)) || m.instancia != global || m.caido != nil {
		t.Fatalf("conecta con el espacio global: %q %v", m.instancia, m.caido)
	}
	aviso := idioma.T("El servicio no responde · r reintenta")
	if strings.Contains(m.View().Content, aviso) {
		t.Fatal("con el servicio en marcha no hay aviso")
	}
	cerrar()
	if correr(t, m, m.conectar(false, false)) {
		t.Fatal("la comprobación no cierra la terminal")
	}
	if !strings.Contains(m.View().Content, aviso) {
		t.Fatalf("avisa de que el servicio no responde:\n%s", m.View().Content)
	}
	_, cmd := m.Update(tecla("r"))
	if cmd == nil || correr(t, m, cmd) {
		t.Fatal("r reintenta y la terminal sigue")
	}
	if !strings.Contains(m.View().Content, aviso) || m.actual != "inicio" {
		t.Fatalf("sigue el aviso:\n%s", m.View().Content)
	}
}

func TestTeclasComunesNoEnCampo(t *testing.T) {
	m := nuevo(Opciones{})
	falsa := &paginaFalsa{campo: true}
	m.paginas["inicio"] = falsa
	for _, k := range []string{"q", "3", "?", "r"} {
		if _, cmd := m.Update(tecla(k)); sale(cmd) {
			t.Fatalf("en un campo, %s no sale", k)
		}
	}
	if falsa.texto != "q3?r" || m.actual != "inicio" {
		t.Fatalf("en un campo las teclas son letras: %q %s", falsa.texto, m.actual)
	}
	falsa.campo = false
	if _, cmd := m.Update(tecla("q")); !sale(cmd) {
		t.Fatal("fuera del campo q sale")
	}
}

func TestAnchosConTildes(t *testing.T) {
	if lipgloss.Width("Configuración") != 13 {
		t.Fatalf("Configuración mide %d", lipgloss.Width("Configuración"))
	}
	m := nuevo(Opciones{})
	m.paginas["inicio"] = &paginaFalsa{}
	for _, tam := range [][2]int{{100, 30}, {60, 20}, {80, 24}, {140, 50}} {
		m.Update(tea.WindowSizeMsg{Width: tam[0], Height: tam[1]})
		lineas := strings.Split(m.View().Content, "\n")
		if len(lineas) != tam[1] {
			t.Fatalf("%v: %d líneas", tam, len(lineas))
		}
		for _, l := range lineas {
			if lipgloss.Width(l) != tam[0] {
				t.Fatalf("%v: una línea mide %d: %q", tam, lipgloss.Width(l), ansi.Strip(l))
			}
		}
		if !strings.Contains(lineas[0], "Configuración") {
			t.Fatalf("%v: la cabecera corta el título: %q", tam, ansi.Strip(lineas[0]))
		}
	}
}

var sgr = regexp.MustCompile(`\x1b\[([0-9;:]*)m`)

// conColor dice si un texto lleva alguna secuencia de color de primer plano o de fondo.
func conColor(s string) bool {
	for _, m := range sgr.FindAllStringSubmatch(s, -1) {
		for _, p := range strings.FieldsFunc(m[1], func(r rune) bool { return r == ';' || r == ':' }) {
			if n, _ := strconv.Atoi(p); (n >= 30 && n <= 49) || (n >= 90 && n <= 107) {
				return true
			}
		}
	}
	return false
}

func TestSinColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if !conColor(nuevo(Opciones{}).View().Content) {
		t.Fatal("sin NO_COLOR hay color")
	}
	t.Setenv("NO_COLOR", "1")
	m := nuevo(Opciones{})
	m.Update(tea.BackgroundColorMsg{Color: color.White})
	m.Update(tecla("tab"))
	if vista := m.View().Content; conColor(vista) {
		t.Fatalf("NO_COLOR quita el color: %q", vista)
	}
}

func TestTemaSegunElFondo(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	m := nuevo(Opciones{})
	if !m.estilos.Oscuro {
		t.Fatal("hasta saber el fondo, el tema es oscuro")
	}
	estilos := m.estilos
	m.Update(tea.BackgroundColorMsg{Color: color.White})
	if m.estilos.Oscuro || m.estilos != estilos {
		t.Fatal("con fondo claro, tema claro, en los mismos Estilos que ya tienen las páginas")
	}
}

func TestAsciiConLaVariable(t *testing.T) {
	t.Setenv("TRAMTAI_ASCII", "1")
	m := nuevo(Opciones{})
	for _, r := range ansi.Strip(m.View().Content) {
		if r > 127 && !strings.ContainsRune("áéíóúüñÁÉÍÓÚÜÑ¿¡×", r) {
			t.Fatalf("con TRAMTAI_ASCII=1 sale %q", r)
		}
	}
}

func TestCambioEspacioReiniciaLaPagina(t *testing.T) {
	m := nuevo(Opciones{})
	falsa := &paginaFalsa{}
	m.paginas["inicio"] = falsa
	m.Update(pagina.CambioEspacio{ID: "notas_1234"})
	if m.instancia != "notas_1234" || falsa.iniciada != 1 {
		t.Fatalf("el cambio de espacio vuelve a iniciar la página: %s %d", m.instancia, falsa.iniciada)
	}
	m.Update(pagina.ErrorServicio{Err: os.ErrDeadlineExceeded})
	if !strings.Contains(m.View().Content, idioma.T("El servicio no responde · r reintenta")) {
		t.Fatal("ErrorServicio de una página enseña el aviso de la app")
	}
}
