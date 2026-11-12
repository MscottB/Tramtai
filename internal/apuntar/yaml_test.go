package apuntar

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func valoresContinue() Valores {
	return Valores{URLOpenAI: "http://127.0.0.1:17373/p/continue/openai/v1", Modelo: "claude/sonnet", Clave: "tk_secreta_123", Puerto: 17373}
}

// datosYAML lee un archivo de testdata/yaml con saltos de línea «\n», lo haya dejado así o no git.
func datosYAML(t *testing.T, nombre string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "yaml", nombre))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func aplicarContinue(t *testing.T, original string) (string, Anotacion) {
	t.Helper()
	nuevo, escrito, previos, err := Continue().Aplicar([]byte(original), valoresContinue())
	if err != nil {
		t.Fatal(err)
	}
	return string(nuevo), Anotacion{ID: "continue", Escrito: escrito, Previos: previos}
}

func leerYAML(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := yaml.Unmarshal([]byte(strings.TrimPrefix(s, string(bom))), &m); err != nil {
		t.Fatalf("no es YAML válido: %v\n%s", err, s)
	}
	return m
}

func modelosContinue(t *testing.T, s string) []map[string]any {
	t.Helper()
	var salida []map[string]any
	lista, _ := leerYAML(t, s)["models"].([]any)
	for _, e := range lista {
		salida = append(salida, e.(map[string]any))
	}
	return salida
}

func nombresContinue(t *testing.T, s string) []any {
	t.Helper()
	var nombres []any
	for _, m := range modelosContinue(t, s) {
		nombres = append(nombres, m["name"])
	}
	return nombres
}

func TestContinueAnadeModelo(t *testing.T) {
	original := datosYAML(t, "config.yaml")
	nuevo, a := aplicarContinue(t, original)
	if got, want := nombresContinue(t, nuevo), []any{"Claude", "Llama local", "Tramtai"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("modelos = %v, quería %v\n%s", got, want, nuevo)
	}
	ultimo := modelosContinue(t, nuevo)[2]
	want := map[string]any{"name": "Tramtai", "provider": "openai", "model": "claude/sonnet", "apiBase": valoresContinue().URLOpenAI,
		"apiKey": "tk_secreta_123", "roles": []any{"chat", "edit", "apply"}, "capabilities": []any{"tool_use"}}
	if !reflect.DeepEqual(ultimo, want) {
		t.Errorf("modelo de Tramtai = %v", ultimo)
	}
	if m := leerYAML(t, nuevo); m["name"] != "Mi asistente" || m["schema"] != "v1" || m["context"] == nil {
		t.Errorf("se perdió algo del archivo:\n%s", nuevo)
	}
	// el nuevo va tras el último modelo y antes de lo que sigue, con el estilo de la lista
	if !strings.Contains(nuevo, "    roles: [chat, autocomplete]\n  - name: Tramtai\n    provider: openai\n") {
		t.Errorf("no lo puso tras el último modelo:\n%s", nuevo)
	}
	if !Continue().Contiene([]byte(nuevo), a) {
		t.Error("Contiene debería dar true")
	}
	if quitado, vacio, err := Continue().Quitar([]byte(nuevo), a); err != nil || vacio || string(quitado) != original {
		t.Errorf("Quitar = %q, %v, %v; quería el original", quitado, vacio, err)
	}
}

func TestContinueConservaComentarios(t *testing.T) {
	original := datosYAML(t, "config.yaml")
	sangria4 := "name: A\nmodels:\n    -   name: Uno # primero\n        provider: ollama\n        model: x\n\n    # ojo\n    -   name: Dos\n        provider: ollama\n        model: y\n# cola\n"
	for nombre, variante := range map[string]string{
		"LF":                        original,
		"sin salto":                 strings.TrimSuffix(original, "\n"),
		"CRLF":                      strings.ReplaceAll(original, "\n", "\r\n"),
		"CRLF sin salto":            strings.TrimSuffix(strings.ReplaceAll(original, "\n", "\r\n"), "\r\n"),
		"BOM":                       string(bom) + original,
		"sangrado de 4":             sangria4,
		"guiones al borde":          "models:\n- name: A\n  provider: ollama\n  model: x\n# cola\n",
		"models al final":           "name: A\nmodels:\n  - name: A\n    model: \"con: dos puntos\" # raro\n    provider: ollama",
		"models sin más":            "models:\n  - {name: A, provider: ollama, model: x}\n",
		"otro modelo ajeno primero": "models:\n  - name: Tramtai2\n    provider: ollama\n    model: x\n",
		"sin models":                "name: A\nversion: 1.0.0\nschema: v1\n# nada más\n",
		"sin models ni salto":       "name: A\nversion: 1.0.0\nschema: v1",
		"solo comentario":           "# hola\n",
	} {
		t.Run(nombre, func(t *testing.T) {
			nuevo, a := aplicarContinue(t, variante)
			cuerpo := strings.TrimPrefix(variante, string(bom))
			for _, l := range strings.Split(cuerpo, "\n") {
				if i := strings.Index(l, "#"); i >= 0 && !strings.Contains(l, `"`) && !strings.Contains(nuevo, strings.TrimSpace(l[i:])) {
					t.Errorf("se perdió el comentario %q:\n%s", l[i:], nuevo)
				}
			}
			if strings.Contains(variante, "\r\n") && strings.Count(nuevo, "\n") != strings.Count(nuevo, "\r\n") {
				t.Errorf("mezcló saltos de línea:\n%q", nuevo)
			}
			if strings.HasPrefix(variante, string(bom)) != strings.HasPrefix(nuevo, string(bom)) {
				t.Error("cambió la marca de orden de bytes")
			}
			if !Continue().Contiene([]byte(nuevo), a) {
				t.Error("Contiene debería dar true")
			}
			quitado, vacio, err := Continue().Quitar([]byte(nuevo), a)
			if err != nil || vacio || string(quitado) != variante {
				t.Errorf("Quitar = %q, %v, %v;\nquería %q", quitado, vacio, err, variante)
			}
		})
	}
}

func TestContinueReemplazaElSuyo(t *testing.T) {
	for nombre, original := range map[string]string{
		"con otros": datosYAML(t, "config.yaml"),
		"solo":      "models:\n  - name: A\n    provider: ollama\n    model: x\n",
		"en medio":  "models:\n  - name: A\n    provider: ollama\n    model: x\ncontext:\n  - provider: code\n",
		"sin salto": "models:\n  - name: A\n    provider: ollama\n    model: x",
	} {
		t.Run(nombre, func(t *testing.T) {
			uno, a1 := aplicarContinue(t, original)
			v := valoresContinue()
			v.Modelo, v.Clave, v.URLOpenAI = "claude/haiku", "tk_otra_456", "http://127.0.0.1:17400/p/continue/openai/v1"
			dos, escrito, previos, err := Continue().Aplicar([]byte(uno), v)
			if err != nil {
				t.Fatal(err)
			}
			if n := strings.Count(string(dos), "name: Tramtai"); n != 1 {
				t.Fatalf("hay %d modelos Tramtai:\n%s", n, dos)
			}
			ms := modelosContinue(t, string(dos))
			if ult := ms[len(ms)-1]; ult["model"] != "claude/haiku" || ult["apiKey"] != "tk_otra_456" || ult["apiBase"] != v.URLOpenAI {
				t.Errorf("no puso los valores nuevos: %v", ult)
			}
			if len(modelosContinue(t, uno)) != len(ms) || len(previos) != 0 {
				t.Errorf("reaplicar cambió el número de modelos o dio previos %v", previos)
			}
			if Continue().Contiene([]byte(dos), a1) {
				t.Error("Contiene con la anotación vieja debería dar false")
			}
			a2 := Anotacion{ID: "continue", Escrito: escrito, Previos: previos}
			if quitado, _, err := Continue().Quitar(dos, a2); err != nil || string(quitado) != original {
				t.Errorf("Quitar tras reaplicar = %q, %v", quitado, err)
			}
		})
	}
}

func TestContinueYAMLRoto(t *testing.T) {
	for nombre, c := range map[string]string{
		"roto":             datosYAML(t, "roto.yaml"),
		"raíz lista":       "- a\n- b\n",
		"raíz texto":       "hola\n",
		"models número":    "models: 5\n",
		"models mapa":      "models:\n  a: 1\n",
		"models con ~":     "models: ~\n",
		"models alias":     "x: &a [1]\nmodels: *a\n",
		"claves repetidas": "a: 1\na: 2\n",
		"dos documentos":   "a: 1\n---\nb: 2\n",
		"tabulador":        "models:\n\t- name: x\n",
	} {
		if _, _, _, err := Continue().Aplicar([]byte(c), valoresContinue()); !errors.Is(err, ErrArchivoIlegible) {
			t.Errorf("%s: Aplicar err = %v, quería ErrArchivoIlegible", nombre, err)
		}
		if nombre == "models número" || nombre == "models mapa" || nombre == "models con ~" || nombre == "models alias" {
			continue // Quitar no tiene nada que quitar de un models que no es una lista
		}
		if _, _, err := Continue().Quitar([]byte(c), Anotacion{Escrito: map[string]any{claveEscritoYAML: map[string]any{}}}); !errors.Is(err, ErrArchivoIlegible) {
			t.Errorf("%s: Quitar err = %v, quería ErrArchivoIlegible", nombre, err)
		}
		if Continue().Contiene([]byte(c), Anotacion{Escrito: map[string]any{claveEscritoYAML: map[string]any{}}}) {
			t.Errorf("%s: Contiene dio true", nombre)
		}
	}
	if _, _, _, err := Continue().Aplicar(nil, Valores{Modelo: "x"}); !errors.Is(err, errSinClave) {
		t.Errorf("sin clave: %v", err)
	}
}

func TestContinueNoGuardaLaClave(t *testing.T) {
	original := datosYAML(t, "config.yaml")
	nuevo, a := aplicarContinue(t, original)
	crudo, _ := json.Marshal(a)
	if strings.Contains(string(crudo), "tk_secreta_123") {
		t.Errorf("la anotación lleva la clave: %s", crudo)
	}
	entrada, _ := a.Escrito[claveEscritoYAML].(map[string]any)
	if entrada["apiKey"] != marca("tk_secreta_123") {
		t.Errorf("apiKey anotada = %v", entrada["apiKey"])
	}
	for k := range a.Previos {
		if _, ok := a.Escrito[k]; !ok {
			t.Errorf("previos tiene %s, que no está en escrito", k)
		}
	}
	// la anotación se guarda en JSON y vuelve igual
	var vuelta Anotacion
	if err := json.Unmarshal(crudo, &vuelta); err != nil || !Continue().Contiene([]byte(nuevo), vuelta) {
		t.Errorf("Contiene tras guardar y leer la anotación: %v", err)
	}
	cambiada := strings.Replace(nuevo, "tk_secreta_123", "tk_otra", 1)
	if Continue().Contiene([]byte(cambiada), a) {
		t.Error("con otra clave, Contiene debería dar false")
	}
	if quitado, _, err := Continue().Quitar([]byte(cambiada), a); err != nil || string(quitado) != cambiada {
		t.Errorf("con otra clave, Quitar no debe tocar nada: %q, %v", quitado, err)
	}
}

func TestContinueQuitarRespetaAlUsuario(t *testing.T) {
	original := datosYAML(t, "config.yaml")
	nuevo, a := aplicarContinue(t, original)
	for nombre, cambio := range map[string][2]string{
		"cambió el modelo": {"model: claude/sonnet", "model: gpt-5"},
		"cambió la URL":    {"apiBase: http://127.0.0.1:17373", "apiBase: http://127.0.0.1:9999"},
		"cambió un rol":    {"      - apply\n", "      - rerank\n"},
		"campo añadido":    {"    capabilities:\n", "    contextLength: 8000\n    capabilities:\n"},
		"lo renombró":      {"name: Tramtai", "name: Mío"},
	} {
		t.Run(nombre, func(t *testing.T) {
			tocado := strings.Replace(nuevo, cambio[0], cambio[1], 1)
			if tocado == nuevo {
				t.Fatal("la prueba no cambió nada")
			}
			if Continue().Contiene([]byte(tocado), a) {
				t.Error("Contiene debería dar false")
			}
			quitado, vacio, err := Continue().Quitar([]byte(tocado), a)
			if err != nil || vacio || string(quitado) != tocado {
				t.Errorf("Quitar debe dejar el archivo como está: %q, %v, %v", quitado, vacio, err)
			}
		})
	}

	t.Run("lo borró", func(t *testing.T) {
		quitado, vacio, err := Continue().Quitar([]byte(original), a)
		if err != nil || vacio || string(quitado) != original {
			t.Errorf("Quitar = %q, %v, %v", quitado, vacio, err)
		}
	})

	t.Run("añadió un modelo detrás", func(t *testing.T) {
		con := strings.Replace(nuevo, "\ncontext:\n", "  - name: Otro\n    provider: ollama\n    model: z\n\ncontext:\n", 1)
		want := strings.Replace(original, "\ncontext:\n", "  - name: Otro\n    provider: ollama\n    model: z\n\ncontext:\n", 1)
		quitado, _, err := Continue().Quitar([]byte(con), a)
		if err != nil || string(quitado) != want {
			t.Errorf("Quitar = %q, %v;\nquería %q", quitado, err, want)
		}
	})
}

func TestContinueModeloAjeno(t *testing.T) {
	nuevo, _ := aplicarContinue(t, "models: []\n")
	for nombre, c := range map[string]string{
		"de otro proveedor":   datosYAML(t, "ajeno.yaml"),
		"con un campo de más": strings.Replace(nuevo, "    capabilities:\n", "    contextLength: 8000\n    capabilities:\n", 1),
		"repetido":            nuevo + "  - name: Tramtai\n    provider: openai\n    model: x\n",
	} {
		_, _, _, err := Continue().Aplicar([]byte(c), valoresContinue())
		if !errors.Is(err, errModeloAjeno) || errors.Is(err, ErrArchivoIlegible) {
			t.Errorf("%s: err = %v, quería errModeloAjeno", nombre, err)
		}
	}
}

func TestContinueArchivoNuevoOVacio(t *testing.T) {
	for _, original := range []string{"", "\n", "# solo un comentario\n"} {
		nuevo, a := aplicarContinue(t, original)
		m := leerYAML(t, nuevo)
		if m["name"] != "Local" || m["version"] != "1.0.0" || m["schema"] != "v1" || len(modelosContinue(t, nuevo)) != 1 {
			t.Errorf("%q: archivo nuevo mal formado:\n%s", original, nuevo)
		}
		quitado, vacio, err := Continue().Quitar([]byte(nuevo), a)
		if err != nil || string(quitado) != original || vacio != (strings.TrimSpace(original) == "") {
			t.Errorf("%q: Quitar = %q, %v, %v", original, quitado, vacio, err)
		}
	}
}

func TestContinueModelsVacio(t *testing.T) {
	for nombre, original := range map[string]string{
		"corchetes":                    "name: A\nmodels: []\nschema: v1\n",
		"corchetes y nota":             "name: A\nmodels: [ ]   # luego\nschema: v1\n",
		"nulo":                         "name: A\nmodels:\nschema: v1\n",
		"nulo con nota":                "name: A\nmodels: # luego\nschema: v1\n",
		"corchetes al final sin salto": "name: A\nmodels: []",
	} {
		t.Run(nombre, func(t *testing.T) {
			nuevo, a := aplicarContinue(t, original)
			if got := nombresContinue(t, nuevo); !reflect.DeepEqual(got, []any{"Tramtai"}) {
				t.Fatalf("modelos = %v\n%s", got, nuevo)
			}
			if m := leerYAML(t, nuevo); m["name"] != "A" || (strings.Contains(original, "schema") && m["schema"] != "v1") {
				t.Errorf("se perdió algo:\n%s", nuevo)
			}
			quitado, vacio, err := Continue().Quitar([]byte(nuevo), a)
			if err != nil || vacio {
				t.Fatalf("Quitar: %v, %v", vacio, err)
			}
			// el models vacío que había vuelve tal cual, con su comentario
			if string(quitado) != original {
				t.Errorf("tras Quitar:\n%q\nquería\n%q", quitado, original)
			}
			// y también tras activar dos veces con el núcleo, que arrastra los previos de la primera
			datos, ruta, h := herramientaTemporal(t, "continue", []byte(original), "config.yaml")
			var an Anotacion
			for i := 0; i < 2; i++ {
				if an, err = Aplicar(datos, h, AlcanceGlobal, "", valoresContinue()); err != nil {
					t.Fatal(err)
				}
			}
			if err := Quitar(datos, h, an); err != nil {
				t.Fatal(err)
			}
			if got := leerTexto(t, ruta); got != original {
				t.Errorf("tras activar dos veces y desactivar:\n%q", got)
			}
		})
	}
	if _, _, _, err := Continue().Aplicar([]byte("models: [{name: A}]\n"), valoresContinue()); !errors.Is(err, errModelosEnLinea) {
		t.Errorf("models entre corchetes con elementos: %v", err)
	}
	if _, _, _, err := Continue().Aplicar([]byte("models: [\n]\n"), valoresContinue()); !errors.Is(err, errModelosEnLinea) {
		t.Errorf("models con corchetes en varias líneas: %v", err)
	}
}

func TestContinueConElNucleo(t *testing.T) {
	datos, ruta, h := herramientaTemporal(t, "continue", nil, "config.yaml")
	a, err := Aplicar(datos, h, AlcanceGlobal, "", valoresContinue())
	if err != nil || !a.NoExistia {
		t.Fatalf("Aplicar: %+v, %v", a, err)
	}
	if got := nombresContinue(t, leerTexto(t, ruta)); !reflect.DeepEqual(got, []any{"Tramtai"}) {
		t.Errorf("modelos = %v", got)
	}
	comprobarPermisos(t, ruta)
	if err := Quitar(datos, h, a); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Errorf("el archivo que creó Tramtai debería haberse borrado: %v", err)
	}

	original := datosYAML(t, "config.yaml")
	datos, ruta, h = herramientaTemporal(t, "continue", []byte(original), "config.yaml")
	for i := 0; i < 2; i++ { // activar dos veces
		if a, err = Aplicar(datos, h, AlcanceGlobal, "", valoresContinue()); err != nil {
			t.Fatal(err)
		}
	}
	if err := Quitar(datos, h, a); err != nil {
		t.Fatal(err)
	}
	if got := leerTexto(t, ruta); got != original {
		t.Errorf("tras activar y desactivar:\n%s", got)
	}
}

func TestContinueDetectada(t *testing.T) {
	casa := t.TempDir()
	t.Setenv("HOME", casa)
	t.Setenv("USERPROFILE", casa)
	t.Setenv("CONTINUE_GLOBAL_DIR", "")
	t.Setenv("PATH", t.TempDir())
	var h Herramienta
	for _, l := range Lista() {
		if l.ID == "continue" {
			h = l
		}
	}
	if Detectada(h) {
		t.Fatal("sin ~/.continue ni cn en el PATH no debería detectarse")
	}
	if err := os.MkdirAll(filepath.Join(casa, ".continue"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !Detectada(h) {
		t.Error("con ~/.continue debería detectarse (Continue suele ser una extensión del editor, sin programa en el PATH)")
	}
}

func TestContinueNoUsaConfigJSON(t *testing.T) {
	datos, ruta, h := herramientaTemporal(t, "continue", nil, "config.yaml")
	json := filepath.Join(filepath.Dir(ruta), "config.json")
	if err := os.WriteFile(json, []byte(`{"models":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Aplicar(datos, h, AlcanceGlobal, "", valoresContinue()); !errors.Is(err, errContinueJSON) {
		t.Fatalf("solo con config.json: err = %v, quería errContinueJSON", err)
	}
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Errorf("se creó config.yaml: %v", err)
	}
	if _, hay, _ := Leer(datos, "continue"); hay {
		t.Error("quedó una anotación")
	}
	if _, err := os.Stat(filepath.Join(datos, "herramientas", "copias")); !os.IsNotExist(err) {
		t.Errorf("se creó una copia: %v", err)
	}
	// con los dos archivos Continue ya lee el YAML: se puede activar
	if err := os.WriteFile(ruta, []byte("name: A\nmodels: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Aplicar(datos, h, AlcanceGlobal, "", valoresContinue()); err != nil {
		t.Errorf("con config.yaml y config.json: %v", err)
	}
}

func TestContinueComentarioTrasSuModelo(t *testing.T) {
	original := datosYAML(t, "config.yaml")
	nuevo, a := aplicarContinue(t, original)
	// una nota del usuario, más sangrada que el guion, justo tras el modelo de Tramtai
	con := strings.Replace(nuevo, "      - tool_use\n", "      - tool_use\n    # nota mía\n\n    # otra\n", 1)
	want := strings.Replace(original, "roles: [chat, autocomplete]\n", "roles: [chat, autocomplete]\n    # nota mía\n\n    # otra\n", 1)
	if con == nuevo || want == original {
		t.Fatal("la prueba no cambió nada")
	}
	quitado, _, err := Continue().Quitar([]byte(con), a)
	if err != nil || string(quitado) != want {
		t.Errorf("Quitar = %q, %v;\nquería %q", quitado, err, want)
	}
	v := valoresContinue()
	v.Modelo = "claude/haiku"
	otro, _, _, err := Continue().Aplicar([]byte(con), v)
	if err != nil || !strings.Contains(string(otro), "      - tool_use\n    # nota mía\n\n    # otra\n") {
		t.Errorf("reaplicar perdió la nota: %v\n%s", err, otro)
	}
}
