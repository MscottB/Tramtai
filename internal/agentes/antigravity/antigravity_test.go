package antigravity

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

// TestMain hace de agy falso cuando AGY_FALSO está definido: comprueba el aislamiento (HOME privado con modelProvider
// gemini, la clave y el hook que niega las herramientas), lee el mensaje de stdin y responde en stream-json.
func TestMain(m *testing.M) {
	if modo := os.Getenv("AGY_FALSO"); modo != "" {
		fmt.Println(`{"event":"init","init":{"tools":["view_file","run_command"]}}`)
		var entrada struct {
			Event   string `json:"event"`
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		linea, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		ajustes, err := os.ReadFile(filepath.Join(os.Getenv("USERPROFILE"), ".gemini", "antigravity-cli", "settings.json"))
		switch {
		case json.Unmarshal([]byte(linea), &entrada) != nil || entrada.Event != "user":
			resultado("ERROR", "", "stream input message is missing the event field")
		case err != nil || !strings.Contains(string(ajustes), `"modelProvider": "gemini"`):
			resultado("ERROR", "", "sin modelProvider gemini")
		case os.Getenv("GEMINI_API_KEY") != "clave-falsa":
			resultado("ERROR", "", "API key not valid")
		case !hookNiega():
			resultado("ERROR", "", "sin hook que niegue las herramientas")
		case modo == "entorno":
			resultado("SUCCESS", os.Getenv("USERPROFILE")+"|"+os.Getenv("APPDATA")+"|"+os.Getenv("LOCALAPPDATA")+"|"+os.Getenv("XDG_CONFIG_HOME")+"|"+strings.Join(os.Args[1:], " "), "")
		case modo == "herramienta":
			fmt.Println(`{"event":"step_update","step_update":{"step_type":"tool","state":"DONE","tool_name":"view_file"}}`)
			fmt.Println(`{"event":"step_update","step_update":{"step_type":"agent_response","state":"ACTIVE","text_delta":"SECRETO"}}`)
			resultado("SUCCESS", "SECRETO", "")
		case modo == "json":
			resultado("SUCCESS", "```json\n{\"a\": 1}\n```", "")
		default:
			for _, trozo := range []string{"eco: ", entrada.Message.Content} {
				d, _ := json.Marshal(map[string]any{"event": "step_update", "step_update": map[string]any{"step_type": "agent_response", "state": "ACTIVE", "text_delta": trozo}})
				fmt.Println(string(d))
			}
			resultado("SUCCESS", "eco: "+entrada.Message.Content, "")
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func resultado(estado, respuesta, error string) {
	d, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]any{"status": estado, "response": respuesta, "error": error,
		"usage": map[string]int{"input_tokens": 100, "output_tokens": 7, "cache_read_tokens": 5}}})
	fmt.Println(string(d))
}

// hookNiega comprueba que .agents/hooks.json de la carpeta de trabajo niega todas las herramientas con su orden.
func hookNiega() bool {
	datos, err := os.ReadFile(filepath.Join(".agents", "hooks.json"))
	if err != nil {
		return false
	}
	var hooks map[string]struct {
		PreToolUse []struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"PreToolUse"`
	}
	if json.Unmarshal(datos, &hooks) != nil {
		return false
	}
	h := hooks["tramtai-sin-herramientas"].PreToolUse
	if len(h) != 1 || h[0].Matcher != "*" || len(h[0].Hooks) != 1 {
		return false
	}
	orden := exec.Command(h[0].Hooks[0].Command)
	orden.Env = os.Environ()
	salida, err := orden.Output()
	var decision struct{ Decision string }
	return err == nil && json.Unmarshal(salida, &decision) == nil && decision.Decision == "deny"
}

func solicitudSimple(texto string) nucleo.SolicitudEjecucion {
	return nucleo.SolicitudEjecucion{Instrucciones: "Sé breve.", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}}
}

func TestAntigravityAisladoYSoloConClave(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	t.Setenv("AGY_FALSO", "eco")
	a := Antigravity{Programa: os.Args[0]}
	var trozos []string
	r, err := a.Ejecutar(context.Background(), solicitudSimple("--yolo piña"), func(e adaptadores.Evento) error {
		trozos = append(trozos, e.Datos)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Salida, "Sé breve.") || !strings.Contains(r.Salida, "--yolo piña") || r.Uso.TokensEntrada != 105 || r.Uso.TokensSalida != 7 {
		t.Fatalf("resultado: %+v", r)
	}
	if len(trozos) != 2 || trozos[0] != "eco: " {
		t.Fatalf("el texto llega en vivo: %q", trozos)
	}
	if _, err := (Antigravity{Programa: os.Args[0], Autenticacion: adaptadores.AutenticacionSuscripcion}).Ejecutar(context.Background(), solicitudSimple("x"),
		func(adaptadores.Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoConfiguracionInvalida {
		t.Fatalf("la sesión personal no se admite: %v", err)
	}
	t.Setenv("GEMINI_API_KEY", "")
	if _, err := a.Ejecutar(context.Background(), solicitudSimple("x"), func(adaptadores.Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoAutenticacionAgente {
		t.Fatalf("sin clave: %v", err)
	}
}

func TestAntigravityEntornoYArgumentos(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	t.Setenv("AGY_FALSO", "entorno")
	s := solicitudSimple(strings.Repeat("x", 40000)) // por stdin no hay tope de línea de órdenes
	s.Modelo, s.Esfuerzo = "gemini-flash-latest", "alto"
	r, err := Antigravity{Programa: os.Args[0]}.Ejecutar(context.Background(), s, func(adaptadores.Evento) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	partes := strings.Split(r.Salida, "|")
	if len(partes) != 5 || partes[0] == "" {
		t.Fatalf("entorno %q", r.Salida)
	}
	for _, p := range partes[1:4] {
		if !strings.HasPrefix(p, partes[0]) {
			t.Fatalf("APPDATA, LOCALAPPDATA y XDG_CONFIG_HOME dentro de la carpeta privada %q: %q", partes[0], r.Salida)
		}
	}
	args := partes[4]
	for _, a := range []string{"--input-format stream-json", "--output-format stream-json", "--model gemini-flash-latest", "--effort high"} {
		if !strings.Contains(args, a) {
			t.Errorf("falta %q en %q", a, args)
		}
	}
	if !strings.HasSuffix(args, "-p=") || strings.Contains(args, "xxxx") || strings.Contains(args, "--json-schema") {
		t.Fatalf("el mensaje no va en la línea de órdenes ni se usa --json-schema: %q", args)
	}
}

func TestAntigravityCortaSiUsaHerramienta(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	t.Setenv("AGY_FALSO", "herramienta")
	var emitidos []string
	_, err := Antigravity{Programa: os.Args[0]}.Ejecutar(context.Background(), solicitudSimple("lee el archivo"), func(e adaptadores.Evento) error {
		emitidos = append(emitidos, e.Datos)
		return nil
	})
	if nucleo.CodigoDe(err) != nucleo.CodigoErrorAgente {
		t.Fatalf("una herramienta ejecutada corta la ejecución: %v", err)
	}
	if strings.Contains(strings.Join(emitidos, "|"), "SECRETO") {
		t.Fatalf("tras la herramienta no se emite nada: %q", emitidos)
	}
}

func TestAntigravityJSONEnInstrucciones(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	t.Setenv("AGY_FALSO", "json")
	s := solicitudSimple("x")
	s.Salida.Esquema = json.RawMessage(`{"type":"object"}`)
	r, err := Antigravity{Programa: os.Args[0]}.Ejecutar(context.Background(), s, func(adaptadores.Evento) error { return nil })
	if err != nil || r.Salida != `{"a": 1}` {
		t.Fatalf("sin el bloque de código: %+v %v", r, err)
	}
}

func TestAntigravityRechazaConsciente(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	s := solicitudSimple("x")
	s.ModoContexto, s.RaizProyecto = "consciente_proyecto", t.TempDir()
	if _, err := (Antigravity{Programa: os.Args[0]}).Ejecutar(context.Background(), s, func(adaptadores.Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoContextoNoPermitido {
		t.Fatalf("consciente_proyecto no se admite: %v", err)
	}
}
