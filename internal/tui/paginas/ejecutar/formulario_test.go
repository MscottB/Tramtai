package ejecutar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/prueba"
)

// Las pruebas del formulario: lo pegado, las plantillas y el esquema, el modelo y el esfuerzo, y los archivos.

// TestPegarVariasLineas: lo pegado (3 líneas, 5000 caracteres) entra entero en el mensaje, e Intro no lo envía.
func TestPegarVariasLineas(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	linea := strings.Repeat("x", 1666)
	texto := linea + "\n" + linea + "\n" + linea
	if len([]rune(texto)) != 5000 {
		t.Fatalf("la prueba pega 5000 caracteres: %d", len(texto))
	}
	p.form.Enfocar("mensaje")
	_, cmd := p.Update(tea.PasteMsg{Content: texto})
	correr(p, cmd)
	if got := p.form.Valor("mensaje"); got != texto {
		t.Fatalf("lo pegado entra entero: %d caracteres, %d líneas", len(got), strings.Count(got, "\n")+1)
	}
	if !p.EnCampo() {
		t.Fatal("pegar empieza a editar el campo")
	}
	pulsar(p, "enter")
	if p.trabajo != nil || p.enviando {
		t.Fatal("Intro en el mensaje salta de línea, no envía")
	}
	if got := p.form.Valor("mensaje"); strings.Count(got, "\n") != 3 {
		t.Fatalf("Intro añade una línea: %d", strings.Count(got, "\n"))
	}
	if lista, _ := cli.Trabajos(ctx, id, 10); len(lista) != 0 {
		t.Fatalf("no se mandó nada: %d", len(lista))
	}
	pulsar(p, "ctrl+s")
	hasta(t, p, func() bool { return p.final != nil })
	if p.final.Estado != nucleo.Completado || !strings.HasPrefix(p.salida, linea) {
		t.Fatalf("Ctrl+S lo envía: %+v", p.final.Estado)
	}
}

// TestPlantillaRellenaEsquema: con la salida estructurada, la plantilla pone su esquema y sus instrucciones; con un esquema
// propio pregunta antes de cambiarlo.
func TestPlantillaRellenaEsquema(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	p.form.Enfocar("estructurada")
	p.editando = true
	pulsar(p, "space")
	if !p.form.Marcado("estructurada") || !strings.Contains(p.leer().esquema, `"resumen"`) {
		t.Fatalf("activar la salida estructurada pone el esquema de ejemplo: %q", p.leer().esquema)
	}
	p.form.Enfocar("plantilla")
	if fuera := pulsar(p, "enter"); len(fuera) != 0 {
		t.Fatalf("sobre el esquema de ejemplo no pregunta: %v", fuera)
	}
	esquema := p.leer().esquema
	if !strings.Contains(esquema, `"nombre"`) || !strings.Contains(esquema, `"format": "date"`) || !strings.Contains(esquema, "\n  ") {
		t.Fatalf("pone el esquema de «Extraer datos», con sangría:\n%s", esquema)
	}
	if !strings.HasPrefix(p.form.Valor("instrucciones"), "Extrae del texto") {
		t.Fatalf("y sus instrucciones, que estaban vacías: %q", p.form.Valor("instrucciones"))
	}
	// Otra plantilla sobre un esquema propio: pregunta; cancelar lo deja y confirmar lo cambia.
	pulsar(p, "right")
	fuera := pulsar(p, "enter")
	if len(fuera) != 1 {
		t.Fatalf("pregunta antes de cambiar el esquema: %v", fuera)
	}
	if q := fuera[0].(comp.Pregunta); !strings.Contains(q.Texto, "Cambiar el esquema") {
		t.Fatalf("la pregunta de la Web UI: %q", q.Texto)
	}
	p.Update(comp.Cancelado{})
	if p.leer().esquema != esquema || !p.EnCampo() {
		t.Fatal("cancelar deja el esquema y sigue editando")
	}
	pulsar(p, "enter")
	p.Update(comp.Confirmado{})
	if p.leer().esquema == esquema || !strings.Contains(p.leer().esquema, `"categoria"`) {
		t.Fatalf("confirmar pone la otra plantilla:\n%s", p.leer().esquema)
	}
	if !strings.HasPrefix(p.form.Valor("instrucciones"), "Extrae del texto") {
		t.Fatal("las instrucciones propias no se tocan")
	}
}

// TestEsquemaInvalidoAvisa: al salir del esquema lo comprueba el servicio y su error se ve bajo el campo; sin arreglarlo
// no se envía.
func TestEsquemaInvalidoAvisa(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	escribir(p, "mensaje", "hola")
	p.form.Enfocar("estructurada")
	pulsar(p, "space")
	aJSON(p)
	p.form.Enfocar("esquema")
	p.form.Poner("esquema", "")
	escribir(p, "esquema", `{"type":"object","properties":{"x":{"type":"nada"}}}`)
	if p.errEsquema != "" {
		t.Fatal("mientras se escribe no se pregunta al servicio")
	}
	pulsar(p, "tab")
	if p.errEsquema == "" {
		t.Fatal("al salir del campo, el servicio dice qué está mal")
	}
	p.form.Enfocar("esquema")
	if v := p.Vista(); !strings.Contains(v, p.errEsquema[:20]) {
		t.Fatalf("el error se ve bajo el campo (%q):\n%s", p.errEsquema, v)
	}
	pulsar(p, "esc", "ctrl+s")
	if p.trabajo != nil || p.enviando {
		t.Fatal("con el esquema mal no se envía")
	}
	if p.form.Foco() != "esquema" {
		t.Fatalf("enseña el campo con el error: %s", p.form.Foco())
	}
	// Un JSON roto también: el servicio da la línea; escribir otra cosa quita el error viejo.
	p.form.Poner("esquema", "")
	escribir(p, "esquema", "{")
	pulsar(p, "tab")
	if p.errEsquema == "" {
		t.Fatal("un JSON roto también se avisa")
	}
	p.form.Enfocar("esquema")
	escribir(p, "esquema", "}")
	if p.errEsquema != "" {
		t.Fatal("al cambiar el esquema, el error del servicio se olvida")
	}
}

// TestModeloYEsfuerzo: el modelo viene del catálogo y el esfuerzo ofrece los niveles de su agente; ConAgente lo elige.
func TestModeloYEsfuerzo(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	if len(p.catalogo) == 0 || !strings.Contains(p.opciones["modelo"].etiquetas[0], "Predeterminado") {
		t.Fatalf("el catálogo da los modelos: %+v", p.opciones["modelo"])
	}
	_, cmd := p.Update(pagina.ConAgente{Agente: "simulado"})
	correr(p, cmd)
	if p.valorOpcion("modelo") != "simulado" || p.form.Foco() != "mensaje" {
		t.Fatalf("Probar con un agente lo deja elegido: %q", p.valorOpcion("modelo"))
	}
	p.form.Enfocar("forzar")
	if p.form.Foco() != "forzar" {
		t.Fatal("con modelo se ofrece forzar")
	}
	p.form.Enfocar("modelo")
	p.editando = true
	for i := 0; i < 20 && p.valorOpcion("modelo") != ""; i++ {
		pulsar(p, "left")
	}
	if p.valorOpcion("modelo") != "" || p.form.Foco() != "modelo" {
		t.Fatalf("← vuelve al predeterminado y el foco sigue en el modelo: %q %s", p.valorOpcion("modelo"), p.form.Foco())
	}
	p.form.Enfocar("forzar")
	if p.form.Foco() == "forzar" {
		t.Fatal("sin modelo no hay forzar")
	}
}

func TestCompletarRuta(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "informe.txt"), nil, 0o600)
	os.WriteFile(filepath.Join(dir, "indice.md"), nil, 0o600)
	os.Mkdir(filepath.Join(dir, "notas"), 0o700)
	base := dir + string(filepath.Separator)
	if got, c := completar("a.txt, " + base + "inf"); got != "a.txt, "+base+"informe.txt" || len(c) != 1 {
		t.Fatalf("completa el único que encaja: %q %v", got, c)
	}
	if got, c := completar(base + "i"); got != base+"in" || len(c) != 2 {
		t.Fatalf("con varios, lo común: %q %v", got, c)
	}
	if got, _ := completar(base + "no"); got != base+"notas"+string(filepath.Separator) {
		t.Fatalf("una carpeta lleva su separador: %q", got)
	}
	if got, c := completar(base + "zz"); got != base+"zz" || c != nil {
		t.Fatalf("sin coincidencias no cambia: %q %v", got, c)
	}
	// Tab en el campo de archivos completa en lugar de pasar al siguiente.
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	escribir(p, "archivos", base+"inf")
	pulsar(p, "tab")
	if p.form.Foco() != "archivos" || p.form.Valor("archivos") != base+"informe.txt" {
		t.Fatalf("Tab completa: %s %q", p.form.Foco(), p.form.Valor("archivos"))
	}
	pulsar(p, "tab")
	if p.form.Foco() != "avanzadas" {
		t.Fatalf("ya completa, Tab pasa al siguiente: %s", p.form.Foco())
	}
}

// TestAvisoConversorAlAdjuntar: al dejar el campo de archivos con un PDF y sin conversor listo se avisa, con la tecla
// que lo prepara, y no se envía.
func TestAvisoConversorAlAdjuntar(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	cli, id := prueba.SinConversor(t)
	p := abierta(t, cli, id)
	pdf := filepath.Join(t.TempDir(), "informe.pdf")
	os.WriteFile(pdf, []byte("%PDF"), 0o600)
	p.form.Poner("mensaje", "resume")
	escribir(p, "archivos", pdf)
	fuera := pulsar(p, "tab")
	if a, ok := aviso(fuera); !ok || !strings.Contains(a.Texto, "Ctrl+P") {
		t.Fatalf("al dejar el campo avisa con la tecla que lo prepara: %v", fuera)
	}
	p.form.Enfocar("archivos")
	if v := p.Vista(); !strings.Contains(v, "Ctrl+P") {
		t.Fatalf("el aviso se ve junto al campo:\n%s", v)
	}
	if a, ok := aviso(pulsar(p, "ctrl+s")); !ok || !strings.Contains(a.Texto, "Ctrl+P") || p.trabajo != nil {
		t.Fatalf("no se envía sin conversor: %+v", a)
	}
	escribir(p, "archivos", ".zip")
	if a, ok := aviso(pulsar(p, "tab")); !ok || !strings.Contains(a.Texto, "ZIP") {
		t.Fatalf("un formato que no se admite se avisa: %+v", a)
	}
}

// TestRutaNoPermitida: un archivo fuera de las raíces permitidas no se sube, y el aviso dice por qué.
func TestRutaNoPermitida(t *testing.T) {
	cli, id := prueba.Servicio(t)
	global := os.Getenv("TRAMTAI_CONFIG_GLOBAL")
	permitida := t.TempDir()
	os.WriteFile(global, []byte("predeterminados:\n  agente: simulado\ninstancia_global:\n  raices_permitidas: ["+jsonTexto(permitida)+"]\n"), 0o600)
	fuera := filepath.Join(t.TempDir(), "nota.txt")
	os.WriteFile(fuera, []byte("hola"), 0o600)
	p := abierta(t, cli, id)
	p.form.Poner("mensaje", "lee")
	p.form.Poner("archivos", fuera)
	if a, ok := aviso(pulsar(p, "ctrl+s")); !ok || p.trabajo != nil || !strings.Contains(a.Texto, "raices_permitidas") {
		t.Fatalf("la ruta se comprueba antes de subir: %+v", a)
	}
}

func jsonTexto(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
