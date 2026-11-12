//go:build ejemplos

// Los ejemplos de docs/ejemplos se ejecutan contra el servidor de la API con un agente simulado: si uno se rompe, el CI
// lo dice antes de publicar. Ejecutar: go test -tags ejemplos -run TestEjemplos -v ./internal/api/
// Python: TRAMTAI_PYTHON (por defecto python3 o python) con openai y requests; JavaScript: npm install en
// docs/ejemplos/javascript; curl: sh, curl y, para los que encadenan pasos, jq.
package api

import (
	"context"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

// agenteEjemplos responde lo que cada ejemplo espera: llamadas a clima (get_weather en inglés) y su respuesta final, un
// JSON con resumen (summary), lo que dice el archivo de muestra o un saludo en vivo. Contesta en inglés a los ejemplos
// de */en: sus preguntas no llevan ¿ ni tildes, y las de los ejemplos en español siempre llevan alguna.
type agenteEjemplos struct{}

func (agenteEjemplos) Ejecutar(_ context.Context, s nucleo.SolicitudEjecucion, emitir func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	var b strings.Builder
	for _, m := range s.Mensajes {
		b.WriteString(m.Contenido + "\n")
	}
	conversacion, esquema := b.String(), string(s.Salida.Esquema)
	ingles := !strings.ContainsAny(conversacion, "¿áéíóúñ")
	elegir := func(es, en string) string {
		if ingles {
			return en
		}
		return es
	}
	var salida string
	switch {
	case strings.Contains(esquema, "tool_calls") && strings.Contains(conversacion, "Tool result"):
		salida = elegir(`{"content":"En Santiago hay 18 °C y está despejado.","tool_calls":[]}`,
			`{"content":"It is 18 °C and clear in Santiago.","tool_calls":[]}`)
	case strings.Contains(esquema, "tool_calls"):
		salida = elegir(`{"content":"","tool_calls":[{"name":"clima","arguments":"{\"ciudad\":\"Santiago\"}"}]}`,
			`{"content":"","tool_calls":[{"name":"get_weather","arguments":"{\"city\":\"Santiago\"}"}]}`)
	case esquema != "":
		salida = elegir(`{"resumen":"Tramtai convierte agentes en una API local."}`, `{"summary":"Tramtai turns agents into a local API."}`)
	case strings.Contains(conversacion, "Tramtai muestra"):
		salida = "El archivo dice: Tramtai muestra."
	case strings.Contains(conversacion, "Tramtai sample"):
		salida = "The file says: Tramtai sample."
	default:
		salida = elegir("Hola desde Tramtai.", "Hello from Tramtai.")
	}
	if esquema == "" {
		_ = emitir(adaptadores.Evento{Tipo: "texto", Datos: salida})
	}
	return adaptadores.Resultado{Salida: salida}, nil
}

// programa busca el primero de los nombres; fuera del CI, si no hay ninguno, salta el ejemplo.
func programa(t *testing.T, nombres ...string) string {
	t.Helper()
	for _, nombre := range nombres {
		if nombre == "" {
			continue
		}
		if ruta, err := exec.LookPath(nombre); err == nil {
			return ruta
		}
	}
	if os.Getenv("CI") != "" {
		t.Fatalf("falta %v en el CI", nombres)
	}
	t.Skipf("falta %v", nombres)
	return ""
}

func TestEjemplos(t *testing.T) {
	ts := servidor(t, simulado(agenteEjemplos{}), map[string]any{"predeterminados.agente": "simulado"})
	raiz, err := filepath.Abs(filepath.Join("..", "..", "docs", "ejemplos"))
	if err != nil {
		t.Fatal(err)
	}
	// Los de */en son la versión inglesa de la guía (desarrolladores.html en inglés): se ejecutan igual, desde la carpeta
	// de su lenguaje para compartir node_modules y el módulo de Go.
	esperado := map[string]string{
		"primer_mensaje": "Hola desde Tramtai", "archivo": "Tramtai muestra", "respuesta_json": "Tramtai convierte",
		"herramientas": "18 °C", "herramientas_api_propia": "18 °C", "trabajo_largo": "Hola desde Tramtai",
		"limites": "extensiones_texto", "limits": "extensiones_texto",
		"first_message": "Hello from Tramtai", "file": "Tramtai sample", "json_response": "Tramtai turns",
		"tools": "18 °C", "tools_native_api": "18 °C", "long_job": "Hello from Tramtai",
	}
	type caso struct{ lenguaje, nombre string } // nombre va con / desde la carpeta del lenguaje: en/tools.py
	var casos []caso
	for _, l := range []string{"python", "javascript", "curl", "go"} {
		for _, sub := range []string{"", "en"} {
			entradas, _ := os.ReadDir(filepath.Join(raiz, l, sub))
			for _, e := range entradas {
				esGo := l == "go" && e.IsDir() && e.Name() != "en"
				if ext := filepath.Ext(e.Name()); ext == ".py" || ext == ".mjs" || ext == ".sh" || esGo {
					casos = append(casos, caso{l, path.Join(sub, e.Name())})
				}
			}
		}
	}
	if len(casos) < 42 {
		t.Fatalf("faltan ejemplos (21 en español y 21 en inglés): %v", casos)
	}
	for _, c := range casos {
		t.Run(c.lenguaje+"/"+c.nombre, func(t *testing.T) {
			quiere, ok := esperado[strings.TrimSuffix(path.Base(c.nombre), path.Ext(c.nombre))]
			if !ok {
				t.Fatalf("ejemplo sin salida esperada: %s", c.nombre)
			}
			dir := filepath.Join(raiz, c.lenguaje)
			archivo := filepath.Join(dir, filepath.FromSlash(c.nombre))
			var argumentos []string
			switch c.lenguaje {
			case "python":
				argumentos = []string{programa(t, os.Getenv("TRAMTAI_PYTHON"), "python3", "python"), archivo}
			case "javascript":
				argumentos = []string{programa(t, "node"), c.nombre}
			case "go":
				argumentos = []string{programa(t, "go"), "run", "./" + c.nombre}
			case "curl":
				programa(t, "curl")
				if guion, _ := os.ReadFile(archivo); strings.Contains(string(guion), "jq ") {
					programa(t, "jq")
				}
				argumentos = []string{programa(t, "sh"), archivo}
			}
			ctx, cancelar := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancelar()
			cmd := exec.CommandContext(ctx, argumentos[0], argumentos[1:]...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "TRAMTAI_URL="+ts.URL, "TRAMTAI_CLAVE=secreto", "TRAMTAI_MODELO=simulado",
				"TRAMTAI_KEY=secreto", "TRAMTAI_MODEL=simulado")
			cmd.WaitDelay = time.Second
			salida, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(salida), quiere) {
				t.Fatalf("%v; se esperaba %q en:\n%s", err, quiere, salida)
			}
		})
	}
}
