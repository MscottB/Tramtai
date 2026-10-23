package tui

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
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

// conducir lleva la terminal con el arnés de las páginas (prueba.App). Al acabar la prueba se cierra, como al salir:
// lo que las páginas dejaron en marcha se corta.
func conducir(t *testing.T, m *modelo) *prueba.App {
	t.Cleanup(m.cerrar)
	return prueba.ConducirApp(t, m)
}

// correr corre un Cmd de la terminal con el arnés hasta que todo se asienta, y dice si pidió salir.
func correr(t *testing.T, m *modelo, cmd tea.Cmd) bool {
	t.Helper()
	return salio(conducir(t, m).Correr(cmd))
}

// salio dice si entre lo que salió de la terminal está tea.Quit.
func salio(fuera []tea.Msg) bool {
	return slices.ContainsFunc(fuera, func(m tea.Msg) bool { _, ok := m.(tea.QuitMsg); return ok })
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
	ctx         context.Context
	instancia   string
	cambios     int // los CambioEspacio que le llegan
	avisos      int // los NuevoAviso que le llegan: no debería ninguno, son de la app
}

func (p *paginaFalsa) Titulo() string { return "Configuración" }
func (p *paginaFalsa) Iniciar(c pagina.Contexto) tea.Cmd {
	p.iniciada++
	p.ancho, p.alto, p.ctx, p.instancia = c.Ancho, c.Alto, c.Ctx, c.Instancia
	return nil
}
func (p *paginaFalsa) Update(msg tea.Msg) (pagina.Pagina, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		p.texto += msg.Text
	case tea.WindowSizeMsg:
		p.ancho, p.alto = msg.Width, msg.Height
	case pagina.CambioEspacio:
		p.cambios++
	case comp.NuevoAviso:
		p.avisos++
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

// TestTamanoMinimoConAlgoPendiente: con la ventana pequeña y algo que se perdería, q pregunta y la pregunta se ve; Esc
// sigue en la terminal, s sale y, con la pregunta abierta, otra q también: nunca queda en un bucle.
func TestTamanoMinimoConAlgoPendiente(t *testing.T) {
	aviso := idioma.T("Agranda la ventana (mínimo 60×20)")
	for _, tam := range [][2]int{{59, 20}, {60, 12}, {40, 10}} {
		m := nuevo(Opciones{})
		m.Update(tea.WindowSizeMsg{Width: tam[0], Height: tam[1]})
		m.paginas[pagina.PaginaInicio] = &paginaOcupada{motivo: "Probar tiene una petición en marcha"}
		if _, cmd := m.Update(tecla("q")); sale(cmd) {
			t.Fatalf("%v: con algo en marcha, q pregunta", tam)
		}
		v := ansi.Strip(m.View().Content)
		for _, quiere := range []string{aviso, "Probar tiene una", "s confirma"} {
			if !strings.Contains(v, quiere) {
				t.Fatalf("%v: la pregunta se ve con la ventana pequeña; falta %q:\n%s", tam, quiere, v)
			}
		}
		if lineas := strings.Split(v, "\n"); len(lineas) > tam[1] || lipgloss.Width(v) > tam[0] {
			t.Fatalf("%v: cabe en la ventana:\n%s", tam, v)
		}
		if _, cmd := m.Update(tecla("esc")); sale(cmd) || m.pregunta != nil {
			t.Fatalf("%v: Esc cierra la pregunta y sigue en la terminal", tam)
		}
		m.Update(tecla("q"))
		if _, cmd := m.Update(tecla("s")); !sale(cmd) {
			t.Fatalf("%v: s sale", tam)
		}
		m.Update(tecla("q"))
		if _, cmd := m.Update(tecla("q")); !sale(cmd) {
			t.Fatalf("%v: con la pregunta de salir abierta, otra q sale", tam)
		}
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
	a := conducir(t, m)
	if salio(a.Correr(m.conectar(false, true, false))) || m.instancia != global || m.caido != nil {
		t.Fatalf("conecta con el espacio global: %q %v", m.instancia, m.caido)
	}
	aviso := idioma.T("El servicio no responde · r reintenta")
	if strings.Contains(m.View().Content, aviso) {
		t.Fatal("con el servicio en marcha no hay aviso")
	}
	cerrar()
	if salio(a.Correr(m.conectar(false, false, false))) {
		t.Fatal("la comprobación no cierra la terminal")
	}
	if !strings.Contains(m.View().Content, aviso) {
		t.Fatalf("avisa de que el servicio no responde:\n%s", m.View().Content)
	}
	_, cmd := m.Update(tecla("r"))
	if cmd == nil || salio(a.Correr(cmd)) {
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

// TestUnSoloCatalogo: un texto de la terminal vive en un solo catálogo. Lo que comparte con la Web UI sale de datos.Texto;
// con idioma.T solo va lo propio de la terminal, así que ningún literal de idioma.T repite un texto de textos.json.
func TestUnSoloCatalogo(t *testing.T) {
	var textos map[string]map[string]string
	bruto, err := os.ReadFile("datos/textos.json")
	if err != nil || json.Unmarshal(bruto, &textos) != nil {
		t.Fatalf("textos.json: %v", err)
	}
	web := map[string]string{}
	for clave, texto := range textos["es"] {
		web[texto] = clave
	}
	err = filepath.WalkDir(".", func(ruta string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(ruta, ".go") || strings.HasSuffix(ruta, "_test.go") {
			return err
		}
		archivo, err := parser.ParseFile(token.NewFileSet(), ruta, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(archivo, func(n ast.Node) bool {
			llamada, ok := n.(*ast.CallExpr)
			if !ok || len(llamada.Args) == 0 {
				return true
			}
			sel, ok := llamada.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if x, _ := sel.X.(*ast.Ident); x == nil || x.Name != "idioma" || (sel.Sel.Name != "T" && sel.Sel.Name != "Tf") {
				return true
			}
			if lit, ok := llamada.Args[0].(*ast.BasicLit); ok {
				if texto, _ := strconv.Unquote(lit.Value); web[texto] != "" {
					t.Errorf("%s: %q ya es %s de la Web UI: usa datos.Texto", ruta, texto, web[texto])
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestColoresDeDesign: los colores de la terminal son los tokens de docs/diseno/DESIGN.md, de día y de noche, y el acento
// es el verde azulado de la marca.
func TestColoresDeDesign(t *testing.T) {
	texto, err := os.ReadFile("../../docs/diseno/DESIGN.md")
	if err != nil {
		t.Fatal(err)
	}
	design := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^  ([a-z-]+): "(#[0-9a-f]{6})"$`).FindAllStringSubmatch(string(texto), -1) {
		design[m[1]] = m[2]
	}
	for nombre, valor := range tokens {
		if design[nombre] != valor {
			t.Errorf("%s: la terminal usa %s y DESIGN.md dice %q", nombre, valor, design[nombre])
		}
	}
	t.Setenv("NO_COLOR", "")
	m := nuevo(Opciones{})
	if r, g, b, _ := m.estilos.Acento.GetForeground().RGBA(); r>>8 != 0x2d || g>>8 != 0xd4 || b>>8 != 0xbf {
		t.Fatalf("de noche, el acento es acento-noche: %x %x %x", r>>8, g>>8, b>>8)
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

// paginaOcupada tiene algo que se perdería al salir.
type paginaOcupada struct {
	paginaFalsa
	motivo string
}

func (p *paginaOcupada) Pendiente() string { return p.motivo }

// TestSalirPreguntaSiHayAlgoEnMarcha: q sale sin preguntar si nada se pierde; con una petición en marcha o cambios sin
// guardar, pregunta y dice qué, y solo s sale.
func TestSalirPreguntaSiHayAlgoEnMarcha(t *testing.T) {
	m := nuevo(Opciones{})
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	o := &paginaOcupada{motivo: "Probar tiene una petición en marcha"}
	m.paginas[pagina.PaginaInicio] = o
	if _, cmd := m.Update(tecla("q")); sale(cmd) {
		t.Fatal("con algo en marcha, q pregunta")
	}
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "Probar tiene una petición en marcha") {
		t.Fatalf("la pregunta dice qué se pierde:\n%s", v)
	}
	if _, cmd := m.Update(tecla("n")); sale(cmd) || m.pregunta != nil {
		t.Fatal("n no sale y cierra la pregunta")
	}
	m.Update(tecla("q"))
	if _, cmd := m.Update(tecla("s")); !sale(cmd) {
		t.Fatal("s sale")
	}
	o.motivo = ""
	if _, cmd := m.Update(tecla("q")); !sale(cmd) {
		t.Fatal("sin nada en marcha, q sale directamente")
	}
}

// TestErrorSeQuedaYSeVeEntero: un error no se va solo; ! lo enseña entero, aunque tenga varias líneas, y la siguiente
// tecla lo quita de la línea de estado.
func TestErrorSeQuedaYSeVeEntero(t *testing.T) {
	m := nuevo(Opciones{})
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	falsa := &paginaFalsa{}
	m.paginas[pagina.PaginaInicio] = falsa
	largo := "El servicio rechazó la petición porque el esquema no es válido en la línea 3\ny esta es la segunda línea del detalle"
	m.Update(comp.NuevoAviso{Texto: largo, Tipo: comp.Fallo})
	v := ansi.Strip(m.View().Content)
	if !strings.Contains(v, "El servicio rechazó") || !strings.Contains(v, "! ver el error") {
		t.Fatalf("el error se ve y el pie dice cómo leerlo entero:\n%s", v)
	}
	m.Update(tecla("!"))
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "segunda línea del detalle") {
		t.Fatalf("! lo enseña entero:\n%s", v)
	}
	m.Update(tecla("esc"))
	m.Update(tecla("a"))
	if v := ansi.Strip(m.View().Content); strings.Contains(v, "El servicio rechazó") || falsa.texto != "a" {
		t.Fatalf("la caja se cierra, la tecla siguiente llega a la página y el error deja la línea:\n%s", v)
	}
	m.Update(tecla("!"))
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "segunda línea del detalle") {
		t.Fatalf("! vuelve a enseñar el último error:\n%s", v)
	}
}

// TestAvisoNoLlegaALasPaginas: un aviso es de la app; tampoco un fallo, que no caduca, se reparte a las páginas.
func TestAvisoNoLlegaALasPaginas(t *testing.T) {
	m := nuevo(Opciones{})
	falsa := &paginaFalsa{}
	m.paginas[pagina.PaginaInicio] = falsa
	m.Update(comp.NuevoAviso{Texto: "guardado", Tipo: comp.Exito})
	m.Update(comp.NuevoAviso{Texto: "no se pudo leer el archivo", Tipo: comp.Fallo})
	if falsa.avisos != 0 {
		t.Fatalf("a la página le llegaron %d avisos", falsa.avisos)
	}
}

// paginaTeclas tiene más teclas de las que caben en la Ayuda a 60×20.
type paginaTeclas struct{ paginaFalsa }

func (p *paginaTeclas) Teclas() []pagina.Tecla {
	var teclas []pagina.Tecla
	for i := range 20 {
		teclas = append(teclas, pagina.Tecla{Tecla: string(rune('a' + i)), Accion: "acción " + strconv.Itoa(i)})
	}
	return teclas
}

// TestAyudaConTodasLasTeclas: a 60×20, la Ayuda de una página con muchas teclas las enseña al empezar su tema y se
// desplaza hasta la última, la de la app; y la que abre la página por su cuenta (a en Inicio) también trae todas, las
// suyas y las comunes.
func TestAyudaConTodasLasTeclas(t *testing.T) {
	m := nuevo(Opciones{})
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m.paginas[pagina.PaginaInicio] = &paginaTeclas{}
	m.Update(tecla("?"))
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "acción 0") {
		t.Fatalf("la Ayuda empieza por las teclas de la página:\n%s", v)
	}
	for range 40 {
		if strings.Contains(ansi.Strip(m.View().Content), "Ctrl+E  espacio") {
			break
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "Ctrl+E  espacio") {
		t.Fatalf("se desplaza hasta la última:\n%s", v)
	}
	m = nuevo(Opciones{})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m.paginas[pagina.PaginaInicio] = &paginaTeclas{}
	m.Update(pagina.IrA{Pagina: pagina.PaginaAyuda, Datos: pagina.ConAyuda{Pagina: pagina.PaginaInicio}})
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "acción 19") || !strings.Contains(v, "Ctrl+E  espacio") {
		t.Fatalf("la Ayuda que pide la página trae sus teclas y las comunes:\n%s", v)
	}
}

// TestExclamacionSoloDondeFunciona: el pie solo anuncia «! ver el error» donde ! lo abre. En un campo ! es una letra
// más, y en el menú no abre nada.
func TestExclamacionSoloDondeFunciona(t *testing.T) {
	m := nuevo(Opciones{})
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	falsa := &paginaFalsa{campo: true}
	m.paginas[pagina.PaginaInicio] = falsa
	m.Update(comp.NuevoAviso{Texto: "no se pudo leer el archivo", Tipo: comp.Fallo})
	pie := func() string { l := strings.Split(ansi.Strip(m.View().Content), "\n"); return l[len(l)-1] }
	if strings.Contains(pie(), "!") {
		t.Fatalf("en un campo el pie no anuncia !: %q", pie())
	}
	m.Update(tecla("!"))
	if m.pregunta != nil || falsa.texto != "!" {
		t.Fatalf("en un campo, ! se escribe: %q", falsa.texto)
	}
	falsa.campo = false
	m.Update(comp.NuevoAviso{Texto: "no se pudo leer el archivo", Tipo: comp.Fallo})
	if !strings.Contains(pie(), "! ver el error") {
		t.Fatalf("fuera de un campo, sí: %q", pie())
	}
	m.Update(tecla("tab"))
	m.Update(comp.NuevoAviso{Texto: "no se pudo leer el archivo", Tipo: comp.Fallo})
	if strings.Contains(pie(), "!") {
		t.Fatalf("en el menú, no: %q", pie())
	}
}

// TestCancelarSeVeEnElPie: con una petición en marcha, «c Cancelar la petición» es lo primero del pie de Probar y se ve a
// 60×20 y a 80×24, en la respuesta y en la receta; c pregunta antes de cancelar.
func TestCancelarSeVeEnElPie(t *testing.T) {
	cli, id, b := prueba.ConBloqueado(t)
	tr, err := cli.Crear(context.Background(), id, nucleo.SolicitudEjecucion{Agente: "simulado",
		Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "esperar"}}})
	if err != nil {
		t.Fatal(err)
	}
	<-b.Entro
	for _, tam := range [][2]int{{60, 20}, {80, 24}} {
		m := nuevoEn(Opciones{Datos: t.TempDir()}, "es")
		m.cli, m.instancia = cli, id
		m.Update(tea.WindowSizeMsg{Width: tam[0], Height: tam[1]})
		a := conducir(t, m)
		a.Correr(m.ir(pagina.PaginaEjecutar, pagina.SeguirTrabajo{ID: tr.ID}, true))
		a.HastaVer("en vivo")
		pie := func() string { l := strings.Split(ansi.Strip(a.Vista()), "\n"); return l[len(l)-1] }
		if !strings.HasPrefix(pie(), "c Cancelar la petición") {
			t.Fatalf("%v: en la respuesta, el pie empieza por c: %q", tam, pie())
		}
		a.Pulsar("left")
		if !strings.HasPrefix(pie(), "c Cancelar la petición") {
			t.Fatalf("%v: en la receta, también: %q", tam, pie())
		}
		a.Pulsar("c")
		if v := ansi.Strip(a.Vista()); !strings.Contains(v, "¿Cancelar la petición en marcha?") {
			t.Fatalf("%v: c pregunta antes de cancelar:\n%s", tam, v)
		}
		a.Pulsar("esc")
	}
}

// TestTeclasComunesSiempreSeVen: a 60×20 y a 80×24, fuera de un campo, el pie de cada página enseña «Tab menú», «? ayuda»
// y «q salir» aunque las teclas de la página no quepan; ? abre la lista entera de las teclas de la página.
func TestTeclasComunesSiempreSeVen(t *testing.T) {
	for _, tam := range [][2]int{{60, 20}, {80, 24}} {
		for _, clave := range []string{pagina.PaginaInicio, pagina.PaginaAgentes, pagina.PaginaEjecutar, pagina.PaginaHistorial} {
			m := nuevoEn(Opciones{Datos: t.TempDir()}, "es")
			m.Update(tea.WindowSizeMsg{Width: tam[0], Height: tam[1]})
			correr(t, m, m.ir(clave, nil, false))
			lineas := strings.Split(ansi.Strip(m.View().Content), "\n")
			pie := lineas[len(lineas)-1]
			for _, quiere := range []string{"Tab menú", "? ayuda", "q salir"} {
				if !strings.Contains(pie, quiere) {
					t.Errorf("%s a %dx%d: el pie enseña %q: %q", clave, tam[0], tam[1], quiere, pie)
				}
			}
		}
	}
	m := nuevoEn(Opciones{Datos: t.TempDir()}, "es")
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m.paginas[pagina.PaginaInicio] = &paginaFalsa{}
	m.Update(tecla("?"))
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "a       anota") || !strings.Contains(v, "Ctrl+E  espacio") {
		t.Fatalf("? enseña todas las teclas de la página y las comunes:\n%s", v)
	}
}
