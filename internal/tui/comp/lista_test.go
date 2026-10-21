package comp

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/tui/pagina"
)

func estilos() *pagina.Estilos { return &pagina.Estilos{Iconos: pagina.IconosUnicode} }

func tecla(texto string) tea.KeyPressMsg {
	switch texto {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "ctrl+enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	}
	return tea.KeyPressMsg{Code: []rune(texto)[0], Text: texto}
}

func escribir(update func(tea.Msg) tea.Cmd, texto string) {
	for _, r := range texto {
		update(tecla(string(r)))
	}
}

type api struct {
	nombre, tipo string
	activa       bool
}

func nombres(as []api) []string {
	var r []string
	for _, a := range as {
		r = append(r, a.nombre)
	}
	return r
}

func apis() []api {
	return []api{{"mi-app", "openai", true}, {"Notas", "anthropic", false}, {"Soporte", "openai", true}, {"Configuración", "anthropic", true}}
}

func TestListaBuscaYFiltra(t *testing.T) {
	l := NuevaLista(estilos(), func(a api) string { return a.nombre })
	l.Poner(apis())
	l.Buscar("NOT")
	if v := nombres(l.Visibles()); !slices.Equal(v, []string{"Notas"}) {
		t.Fatalf("buscar no distingue mayúsculas: %v", v)
	}
	l.Buscar("configuracion")
	if v := nombres(l.Visibles()); !slices.Equal(v, []string{"Configuración"}) {
		t.Fatalf("buscar no distingue tildes: %v", v)
	}
	l.Buscar("")
	l.Filtrar(func(a api) bool { return a.activa })
	if v := nombres(l.Visibles()); !slices.Equal(v, []string{"mi-app", "Soporte", "Configuración"}) {
		t.Fatalf("filtro: %v", v)
	}
	l.Buscar("so")
	if v := nombres(l.Visibles()); !slices.Equal(v, []string{"Soporte"}) {
		t.Fatalf("buscar y filtrar a la vez: %v", v)
	}
	l.Buscar("")
	l.Filtrar(nil)
	l.Ordenar(func(a, b api) int { return strings.Compare(a.nombre, b.nombre) })
	if v := nombres(l.Visibles()); !slices.Equal(v, []string{"Configuración", "Notas", "Soporte", "mi-app"}) {
		t.Fatalf("orden: %v", v)
	}
	if a, ok := l.Seleccion(); !ok || a.nombre != "Configuración" {
		t.Fatalf("la selección empieza arriba: %v %v", a, ok)
	}
	l.Update(tecla("down"))
	l.Update(tecla("down"))
	if a, _ := l.Seleccion(); a.nombre != "Soporte" {
		t.Fatalf("flecha abajo: %v", a)
	}
	if v := l.Vista(40, 10); !strings.Contains(v, "Soporte") || !strings.Contains(v, "mi-app") {
		t.Fatalf("vista:\n%s", v)
	}
	l.Buscar("zzz")
	if _, ok := l.Seleccion(); ok {
		t.Fatal("sin resultados no hay selección")
	}
	if v := l.Vista(40, 10); !strings.Contains(v, "Sin resultados") {
		t.Fatalf("lista vacía:\n%s", v)
	}
}

func TestListaGruposPlegables(t *testing.T) {
	l := NuevaLista(estilos(), func(a api) string { return a.nombre })
	l.Poner(apis())
	l.Agrupar(func(a api) string { return a.tipo })
	v := l.Vista(40, 10)
	if !strings.Contains(v, "openai (2)") || !strings.Contains(v, "anthropic (2)") || strings.Index(v, "openai") > strings.Index(v, "anthropic") {
		t.Fatalf("grupos en el orden en que aparecen:\n%s", v)
	}
	if _, ok := l.Seleccion(); ok {
		t.Fatal("el cursor empieza en la cabecera del grupo, que no es un elemento")
	}
	l.Update(tecla("enter"))
	if v := l.Vista(40, 10); strings.Contains(v, "mi-app") || !strings.Contains(v, "openai (2)") || !strings.Contains(v, "Notas") {
		t.Fatalf("Intro pliega el grupo:\n%s", v)
	}
	if len(l.Visibles()) != 2 {
		t.Fatalf("un grupo plegado no tiene visibles: %v", nombres(l.Visibles()))
	}
	l.Update(tecla("right"))
	l.Update(tecla("down"))
	if a, ok := l.Seleccion(); !ok || a.nombre != "mi-app" {
		t.Fatalf("→ despliega: %v", a)
	}
	l.Update(tecla("left"))
	if _, ok := l.Seleccion(); ok || strings.Contains(l.Vista(40, 10), "mi-app") {
		t.Fatal("← pliega el grupo del elemento y vuelve a su cabecera")
	}
	l.Buscar("notas")
	if v := l.Vista(40, 10); strings.Contains(v, "openai") || !strings.Contains(v, "anthropic (1)") {
		t.Fatalf("un grupo sin resultados no sale:\n%s", v)
	}
}

func TestListaDesplaza(t *testing.T) {
	l := NuevaLista(estilos(), func(n string) string { return n })
	var ns []string
	for i := range 30 {
		ns = append(ns, "fila-"+string(rune('a'+i%26))+strings.Repeat("x", i/26))
	}
	l.Poner(ns)
	for range 20 {
		l.Update(tecla("down"))
	}
	v := l.Vista(30, 5)
	if lineas := strings.Split(v, "\n"); len(lineas) != 5 || !strings.Contains(v, ns[20]) {
		t.Fatalf("el cursor siempre se ve:\n%s", v)
	}
}
