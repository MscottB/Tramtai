package antigravity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

// TestMain hace de agy falso cuando AGY_FALSO está definido: comprueba el aislamiento y responde en JSON.
func TestMain(m *testing.M) {
	if modo := os.Getenv("AGY_FALSO"); modo != "" {
		ajustes, err := os.ReadFile(filepath.Join(os.Getenv("USERPROFILE"), ".gemini", "antigravity-cli", "settings.json"))
		switch {
		case err != nil || !strings.Contains(string(ajustes), `"modelProvider": "gemini"`):
			fmt.Println(`{"status":"ERROR","response":"sin modelProvider gemini"}`)
		case os.Getenv("GEMINI_API_KEY") != "clave-falsa":
			fmt.Println(`{"status":"ERROR","response":"API key not valid"}`)
		case modo == "entorno":
			r, _ := json.Marshal(map[string]any{"status": "SUCCESS", "response": os.Getenv("USERPROFILE") + "|" + os.Getenv("APPDATA") + "|" +
				os.Getenv("LOCALAPPDATA") + "|" + os.Getenv("XDG_CONFIG_HOME")})
			fmt.Println(string(r))
		case modo == "esquema":
			fmt.Println(`{"status":"SUCCESS","response":"{\"a\":1,\"extra\":2}","structured_output":{"a":1},"usage":{"input_tokens":10,"output_tokens":3}}`)
		default:
			prompt := ""
			for i, a := range os.Args {
				if a == "-p" && i+1 < len(os.Args) {
					prompt = os.Args[i+1]
				}
			}
			r, _ := json.Marshal(map[string]any{"status": "SUCCESS", "response": "eco: " + prompt + "\n",
				"usage": map[string]int{"input_tokens": 100, "output_tokens": 7, "cache_read_tokens": 5}})
			fmt.Println(string(r))
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func solicitudSimple(texto string) nucleo.SolicitudEjecucion {
	return nucleo.SolicitudEjecucion{Instrucciones: "Sé breve.", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}}
}

func TestAntigravityAisladoYSoloConClave(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	t.Setenv("AGY_FALSO", "eco")
	a := Antigravity{Programa: os.Args[0]}
	r, err := a.Ejecutar(context.Background(), solicitudSimple("hola"), func(adaptadores.Evento) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Salida, "Sé breve.") || !strings.Contains(r.Salida, "hola") || r.Uso.TokensEntrada != 105 || r.Uso.TokensSalida != 7 {
		t.Fatalf("resultado: %+v", r)
	}
	t.Setenv("AGY_FALSO", "esquema")
	s := solicitudSimple("x")
	s.Salida.Esquema = json.RawMessage(`{"type":"object"}`)
	if r, err := a.Ejecutar(context.Background(), s, func(adaptadores.Evento) error { return nil }); err != nil || r.Salida != `{"a":1}` {
		t.Fatalf("structured_output prevalece: %+v %v", r, err)
	}
	if _, err := (Antigravity{Programa: os.Args[0], Autenticacion: adaptadores.AutenticacionSuscripcion}).Ejecutar(context.Background(), s,
		func(adaptadores.Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoConfiguracionInvalida {
		t.Fatalf("la sesión personal no se admite: %v", err)
	}
	t.Setenv("GEMINI_API_KEY", "")
	if _, err := a.Ejecutar(context.Background(), s, func(adaptadores.Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoAutenticacionAgente {
		t.Fatalf("sin clave: %v", err)
	}
}

func TestAntigravityNoEmpiezaPorGuion(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	t.Setenv("AGY_FALSO", "eco")
	s := nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "--yolo"}}}
	r, err := Antigravity{Programa: os.Args[0]}.Ejecutar(context.Background(), s, func(adaptadores.Evento) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	// El argumento tras -p no empieza por «-»: un CLI no lo puede leer como una opción.
	if !strings.Contains(r.Salida, "--yolo") || strings.HasPrefix(strings.TrimPrefix(r.Salida, "eco: "), "-") {
		t.Fatalf("el prompt llegó como %q", r.Salida)
	}
}

func TestAntigravityEntornoPrivado(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	t.Setenv("AGY_FALSO", "entorno")
	r, err := Antigravity{Programa: os.Args[0]}.Ejecutar(context.Background(), solicitudSimple("x"), func(adaptadores.Evento) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	partes := strings.Split(r.Salida, "|")
	if len(partes) != 4 || partes[0] == "" {
		t.Fatalf("entorno %q", r.Salida)
	}
	for _, p := range partes[1:] {
		if !strings.HasPrefix(p, partes[0]) {
			t.Fatalf("APPDATA, LOCALAPPDATA y XDG_CONFIG_HOME dentro de la carpeta privada %q: %q", partes[0], r.Salida)
		}
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

func TestAntigravityLineaOrdenes(t *testing.T) {
	for _, c := range []struct {
		arg      string
		unidades int
	}{{"abc", 3}, {"a b", 5}, {`a"b`, 4}, {`a\"b`, 6}, {`a b\`, 7}, {"😀", 2}} {
		if n := unidadesLineaOrdenes(c.arg); n != c.unidades {
			t.Fatalf("%q: %d unidades, se esperaban %d", c.arg, n, c.unidades)
		}
	}
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	t.Setenv("AGY_FALSO", "eco")
	s := nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: strings.Repeat("x", 28001)}}}
	if _, err := (Antigravity{Programa: os.Args[0]}).Ejecutar(context.Background(), s, func(adaptadores.Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoEntradaDemasiadoGrande {
		t.Fatalf("más de 28.000 unidades: %v", err)
	}
}
