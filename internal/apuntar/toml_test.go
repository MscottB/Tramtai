package apuntar

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

var ordenCodex = []string{`C:\Tramtai\tramtai.exe`, "herramientas", "clave", "codex"}

func valoresCodex() Valores {
	return Valores{URLOpenAI: "http://127.0.0.1:17373/p/codex/openai/v1", Modelo: "claude/sonnet", Orden: ordenCodex, Puerto: 17373}
}

const (
	arribaCodex = "# >>> tramtai\nmodel = \"claude/sonnet\"\nmodel_provider = \"tramtai\"\n# <<< tramtai\n"
	abajoCodex  = "# >>> tramtai\n[model_providers.tramtai]\nname = \"Tramtai\"\nbase_url = \"http://127.0.0.1:17373/p/codex/openai/v1\"\n" +
		"wire_api = \"responses\"\n\n[model_providers.tramtai.auth]\ncommand = 'C:\\Tramtai\\tramtai.exe'\n" +
		"args = [\"herramientas\", \"clave\", \"codex\"]\n# <<< tramtai\n"
)

// datosTOML lee un archivo de testdata/toml con saltos de línea «\n», lo haya dejado así o no git.
func datosTOML(t *testing.T, nombre string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "toml", nombre))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func aplicarCodex(t *testing.T, original string) (string, Anotacion) {
	t.Helper()
	nuevo, escrito, previos, err := Codex().Aplicar([]byte(original), valoresCodex())
	if err != nil {
		t.Fatal(err)
	}
	return string(nuevo), Anotacion{ID: "codex", Escrito: escrito, Previos: previos}
}

func leerTOML(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := toml.Unmarshal([]byte(strings.TrimPrefix(s, string(bom))), &m); err != nil {
		t.Fatalf("no es TOML válido: %v\n%s", err, s)
	}
	return m
}

func TestCodexBloquesYValidez(t *testing.T) {
	for _, original := range []string{"", datosTOML(t, "config.toml")} {
		nuevo, a := aplicarCodex(t, original)
		m := leerTOML(t, nuevo)
		if m["model_provider"] != "tramtai" || m["model"] != "claude/sonnet" {
			t.Errorf("model/model_provider = %v / %v", m["model"], m["model_provider"])
		}
		if got := en(t, m, "model_providers", "tramtai", "base_url"); got != valoresCodex().URLOpenAI {
			t.Errorf("base_url = %v", got)
		}
		if got := en(t, m, "model_providers", "tramtai", "wire_api"); got != "responses" {
			t.Errorf("wire_api = %v", got)
		}
		if got := en(t, m, "model_providers", "tramtai", "auth", "command"); got != ordenCodex[0] {
			t.Errorf("command = %v", got)
		}
		if got := en(t, m, "model_providers", "tramtai", "auth", "args"); !reflect.DeepEqual(got, []any{"herramientas", "clave", "codex"}) {
			t.Errorf("args = %v", got)
		}
		if want := arribaCodex + original + abajoCodex; nuevo != want {
			t.Errorf("archivo:\n%s\nquería:\n%s", nuevo, want)
		}
		if !Codex().Contiene([]byte(nuevo), a) {
			t.Error("Contiene debería dar true")
		}
		if quitado, vacio, err := Codex().Quitar([]byte(nuevo), a); err != nil || string(quitado) != original || vacio != (original == "") {
			t.Errorf("Quitar = %q, %v, %v; quería %q", quitado, vacio, err, original)
		}
	}
	if _, _, _, err := Codex().Aplicar(nil, Valores{Modelo: "x"}); !errors.Is(err, errSinOrden) {
		t.Errorf("sin orden: %v", err)
	}
}

func TestCodexComentaYDevuelvePrevios(t *testing.T) {
	original := datosTOML(t, "con-modelo.toml")
	nuevo, a := aplicarCodex(t, original)
	if !strings.Contains(nuevo, "# tramtai:   model = \"gpt-5\"   # el que uso\n# tramtai: model_provider = \"openai\"\n") {
		t.Errorf("no comenta las líneas previas:\n%s", nuevo)
	}
	if m := leerTOML(t, nuevo); m["model"] != "claude/sonnet" || m["model_provider"] != "tramtai" {
		t.Errorf("model = %v, model_provider = %v", m["model"], m["model_provider"])
	}
	if want := map[string]any{"model": "gpt-5", "model_provider": "openai"}; !reflect.DeepEqual(a.Previos, want) {
		t.Errorf("previos = %v", a.Previos)
	}
	for k := range a.Previos {
		if _, ok := a.Escrito[k]; !ok {
			t.Errorf("previos tiene %s, que no está en escrito", k)
		}
	}
	quitado, vacio, err := Codex().Quitar([]byte(nuevo), a)
	if err != nil || vacio || string(quitado) != original {
		t.Errorf("Quitar = %q, %v, %v; quería el original", quitado, vacio, err)
	}

	// activar otra vez (otro modelo): un solo bloque de cada y las líneas previas siguen comentadas una sola vez
	v := valoresCodex()
	v.Modelo = "claude/haiku"
	otro, _, previos, err := Codex().Aplicar([]byte(nuevo), v)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(otro), marcaIni); n != 2 {
		t.Errorf("bloques = %d, quería 2", n)
	}
	if strings.Contains(string(otro), "# tramtai: # tramtai:") || len(previos) != 0 {
		t.Errorf("reaplicar toca los previos: %v\n%s", previos, otro)
	}
	if quitado, _, err := Codex().Quitar(otro, a); err != nil || string(quitado) != original {
		t.Errorf("Quitar tras reaplicar = %q, %v", quitado, err)
	}
}

func TestCodexConservaComentariosYTablas(t *testing.T) {
	original := datosTOML(t, "config.toml")
	for nombre, variante := range map[string]string{
		"LF":             original,
		"sin salto":      strings.TrimSuffix(original, "\n"),
		"CRLF":           strings.ReplaceAll(original, "\n", "\r\n"),
		"CRLF sin salto": strings.TrimSuffix(strings.ReplaceAll(original, "\n", "\r\n"), "\r\n"),
		"BOM":            string(bom) + original,
		"solo tablas":    "[a]\nx = 1\n",
		"solo un valor":  "x = 1",
	} {
		t.Run(nombre, func(t *testing.T) {
			nuevo, a := aplicarCodex(t, variante)
			eol := "\n"
			if strings.Contains(variante, "\r\n") {
				eol = "\r\n"
			}
			cuerpo := strings.TrimPrefix(variante, string(bom))
			fin := ""
			if !strings.HasSuffix(cuerpo, "\n") && cuerpo != "" {
				fin = eol
			}
			arriba := strings.ReplaceAll(arribaCodex, "\n", eol)
			abajo := strings.ReplaceAll(abajoCodex, "\n", eol)
			if fin != "" {
				abajo = strings.TrimSuffix(abajo, eol)
			}
			if want := variante[:len(variante)-len(cuerpo)] + arriba + cuerpo + fin + abajo; nuevo != want {
				t.Errorf("fuera de los bloques cambió algo:\n%q\n%q", nuevo, want)
			}
			if quitado, _, err := Codex().Quitar([]byte(nuevo), a); err != nil || string(quitado) != variante {
				t.Errorf("Quitar no devuelve el original:\n%q\n%q (%v)", quitado, variante, err)
			}
		})
	}
	// la «clave» dentro de un texto de varias líneas y la de un perfil no son del nivel superior: no se comentan
	nuevo, _ := aplicarCodex(t, original)
	if strings.Contains(nuevo, prefijoComentado) {
		t.Errorf("comentó una línea que no era de nivel superior:\n%s", nuevo)
	}
}

func TestCodexBloqueBorradoAMano(t *testing.T) {
	original := datosTOML(t, "con-modelo.toml")
	nuevo, a := aplicarCodex(t, original)
	sinBloques := strings.Replace(strings.Replace(nuevo, arribaCodex, "", 1), abajoCodex, "", 1)
	if sinBloques == nuevo {
		t.Fatal("la prueba no quitó los bloques")
	}
	if Codex().Contiene([]byte(sinBloques), a) {
		t.Error("Contiene debería dar false sin los bloques")
	}
	quitado, vacio, err := Codex().Quitar([]byte(sinBloques), a)
	if err != nil || vacio || string(quitado) != original {
		t.Errorf("Quitar = %q, %v, %v; quería %q", quitado, vacio, err, original)
	}
	// solo falta el bloque del proveedor, o el usuario cambió el modelo: tampoco es ya lo de Tramtai
	if Codex().Contiene([]byte(strings.Replace(nuevo, abajoCodex, "", 1)), a) {
		t.Error("Contiene debería dar false sin el proveedor")
	}
	if Codex().Contiene([]byte(strings.Replace(nuevo, `model = "claude/sonnet"`, `model = "otro"`, 1)), a) {
		t.Error("Contiene debería dar false con otro modelo")
	}
	// el usuario puso su propio modelo tras borrar los bloques: no se descomenta (quedaría repetido) y no falla
	suyo := strings.Replace(sinBloques, "# tramtai:   model = \"gpt-5\"   # el que uso\n", "model = \"mio\"\n", 1)
	quitado, _, err = Codex().Quitar([]byte(suyo), a)
	if err != nil {
		t.Fatal(err)
	}
	if m := leerTOML(t, string(quitado)); m["model"] != "mio" || m["model_provider"] != "openai" {
		t.Errorf("model = %v, model_provider = %v", m["model"], m["model_provider"])
	}
	// una marca de apertura sin cierre no se toma por un bloque
	huerfano := "# >>> tramtai\nx = 1\n"
	if quitado, _, err := Codex().Quitar([]byte(huerfano), a); err != nil || string(quitado) != huerfano {
		t.Errorf("Quitar con marca huérfana = %q, %v", quitado, err)
	}
}

func TestCodexRutaWindows(t *testing.T) {
	v := valoresCodex()
	v.Orden = []string{`C:\Users\Ana María\AppData\Local\Tramtai\tramtai.exe`, "herramientas", "clave", "codex"}
	nuevo, _, _, err := Codex().Aplicar(nil, v)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(nuevo), `command = 'C:\Users\Ana María\AppData\Local\Tramtai\tramtai.exe'`+"\n") {
		t.Errorf("no es una cadena literal:\n%s", nuevo)
	}
	if got := en(t, leerTOML(t, string(nuevo)), "model_providers", "tramtai", "auth", "command"); got != v.Orden[0] {
		t.Errorf("command = %v", got)
	}

	// con una comilla simple no cabe una cadena literal: cadena básica, escapada
	v.Orden = []string{`C:\Users\O'Hara "Ana"\tramtai.exe`, "a b", `c\d`, "herramientas"}
	nuevo, _, _, err = Codex().Aplicar(nil, v)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(nuevo), `command = "C:\\Users\\O'Hara \"Ana\"\\tramtai.exe"`+"\n") {
		t.Errorf("no escapó la cadena básica:\n%s", nuevo)
	}
	m := leerTOML(t, string(nuevo))
	if got := en(t, m, "model_providers", "tramtai", "auth", "command"); got != v.Orden[0] {
		t.Errorf("command = %v", got)
	}
	if got := en(t, m, "model_providers", "tramtai", "auth", "args"); !reflect.DeepEqual(got, []any{"a b", `c\d`, "herramientas"}) {
		t.Errorf("args = %v", got)
	}

	// «%» y «&» no son problema: Codex ejecuta la orden sin shell
	v.Orden = []string{`C:\Tram&tai 100%\tramtai.exe`}
	nuevo, _, _, err = Codex().Aplicar(nil, v)
	if err != nil {
		t.Fatal(err)
	}
	if got := en(t, leerTOML(t, string(nuevo)), "model_providers", "tramtai", "auth", "command"); got != v.Orden[0] {
		t.Errorf("command = %v", got)
	}
	if strings.Contains(string(nuevo), "args") {
		t.Error("sin argumentos no debería escribir args")
	}
}

func TestCodexArchivoIlegible(t *testing.T) {
	for nombre, original := range map[string]string{
		"roto":                    datosTOML(t, "roto.toml"),
		"proveedor ya definido":   "[model_providers.tramtai]\nname = \"mio\"\n",
		"modelo en varias lineas": "model = \"\"\"\nuno\n\"\"\"\n",
		"model_provider punteado": "model_provider.x = 1\n",
	} {
		if _, _, _, err := Codex().Aplicar([]byte(original), valoresCodex()); !errors.Is(err, ErrArchivoIlegible) {
			t.Errorf("%s: err = %v", nombre, err)
		}
		if _, _, err := Codex().Quitar([]byte(original), Anotacion{}); nombre == "roto" && !errors.Is(err, ErrArchivoIlegible) {
			t.Errorf("%s: Quitar err = %v", nombre, err)
		}
	}
}

func TestCodexConElNucleo(t *testing.T) {
	// el archivo no existe: lo crea Tramtai y, al quitar, se borra
	datos, ruta, h := herramientaTemporal(t, "codex", nil, "config.toml")
	a, err := Aplicar(datos, h, AlcanceGlobal, "", valoresCodex())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(ruta)
	if !a.NoExistia || string(b) != arribaCodex+abajoCodex || a.Orden != ordenCodex[0] || a.API != "codex" {
		t.Fatalf("anotación %+v, archivo %q", a, b)
	}
	if !h.Formato.Contiene(b, a) {
		t.Error("Contiene debería dar true")
	}
	if err := Quitar(datos, h, a); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Errorf("el archivo debería haberse borrado: %v", err)
	}

	// el archivo existe con modelo previo: dos activaciones seguidas y se desactiva al estado original
	original := datosTOML(t, "con-modelo.toml")
	datos, ruta, h = herramientaTemporal(t, "codex", []byte(original), "config.toml")
	if _, err := Aplicar(datos, h, AlcanceGlobal, "", valoresCodex()); err != nil {
		t.Fatal(err)
	}
	v := valoresCodex()
	v.Modelo = "claude/haiku"
	a, err = Aplicar(datos, h, AlcanceGlobal, "", v)
	if err != nil {
		t.Fatal(err)
	}
	if want := (map[string]any{"model": "gpt-5", "model_provider": "openai"}); !reflect.DeepEqual(a.Previos, want) {
		t.Errorf("previos = %v", a.Previos)
	}
	if err := Quitar(datos, h, a); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(ruta); string(b) != original {
		t.Errorf("no volvió al original:\n%s", b)
	}
}

func TestRutaCodex(t *testing.T) {
	casa, codex := t.TempDir(), t.TempDir()
	t.Setenv("HOME", casa)
	t.Setenv("USERPROFILE", casa)
	t.Setenv("CODEX_HOME", "")
	if got, _ := rutaCodex(AlcanceGlobal, ""); got != filepath.Join(casa, ".codex", "config.toml") {
		t.Errorf("ruta = %s", got)
	}
	t.Setenv("CODEX_HOME", codex)
	if got, _ := rutaCodex(AlcanceGlobal, ""); got != filepath.Join(codex, "config.toml") {
		t.Errorf("ruta con CODEX_HOME = %s", got)
	}
}
