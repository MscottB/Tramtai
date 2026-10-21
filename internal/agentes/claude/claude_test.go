package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/credenciales"
	"tramtai/internal/nucleo"
)

// registro es lo que recibió el CLI falso.
type registro struct {
	Entrada string   `json:"entrada"`
	Args    []string `json:"args"`
	Entorno []string `json:"entorno"`
}

// TestMain hace de CLI falso cuando FALSO_REGISTRO está definido (Tramtai no quita esa variable al limpiar el entorno):
// apunta lo que recibe y responde con la muestra de testdata indicada en FALSO_SALIDA.
func TestMain(m *testing.M) {
	if archivo := os.Getenv("FALSO_REGISTRO"); archivo != "" {
		entrada, _ := io.ReadAll(os.Stdin)
		datos, _ := json.Marshal(registro{Entrada: string(entrada), Args: os.Args[1:], Entorno: os.Environ()})
		os.WriteFile(archivo, datos, 0o600)
		salida, _ := os.ReadFile(filepath.Join(os.Getenv("FALSO_DATOS"), os.Getenv("FALSO_SALIDA")))
		os.Stdout.Write(salida)
		fmt.Fprint(os.Stderr, os.Getenv("FALSO_ERRORES"))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// falso prepara el CLI falso con la muestra salida y devuelve dónde deja su registro.
func falso(t *testing.T, salida string) string {
	t.Helper()
	muestras, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	archivo := filepath.Join(t.TempDir(), "registro.json")
	t.Setenv("FALSO_REGISTRO", archivo)
	t.Setenv("FALSO_DATOS", muestras)
	t.Setenv("FALSO_SALIDA", salida)
	return archivo
}

func leerRegistro(t *testing.T, archivo string) registro {
	t.Helper()
	datos, err := os.ReadFile(archivo)
	if err != nil {
		t.Fatal(err)
	}
	var r registro
	if err := json.Unmarshal(datos, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// variable devuelve el valor de una variable del entorno registrado y si estaba.
func variable(r registro, nombre string) (string, bool) {
	for _, par := range r.Entorno {
		if n, v, _ := strings.Cut(par, "="); strings.EqualFold(n, nombre) {
			return v, true
		}
	}
	return "", false
}

func solicitud(texto string) nucleo.SolicitudEjecucion {
	return nucleo.SolicitudEjecucion{Instrucciones: "Sé breve.", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}}
}

func sinEventos(adaptadores.Evento) error { return nil }

func conClave() Claude {
	return Claude{Programa: os.Args[0], Autenticacion: adaptadores.NuevoModo(adaptadores.AutenticacionClaveAPI)}
}

// Con clave de API, Claude Code va sin herramientas, sin MCP, sin sesiones guardadas, con --bare y con el sistema
// propio; el texto llega en vivo y el resultado trae uso, costo, modelo efectivo y contexto.
func TestClaudeConClave(t *testing.T) {
	archivo := falso(t, "salida.jsonl")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-prueba")
	t.Setenv("OPENAI_API_KEY", "sk-de-otro-agente")
	s := solicitud("hola")
	s.Modelo, s.Esfuerzo = "haiku", "maximo"
	var textos []string
	res, err := conClave().Ejecutar(context.Background(), s, func(e adaptadores.Evento) error {
		textos = append(textos, e.Datos)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Salida != "Hola mundo" || res.ModeloEfectivo != "claude-haiku-4-5-20251001" || res.ContextoTokens != 200000 ||
		res.Uso.TokensEntrada != 115 || res.Uso.TokensSalida != 3 || res.Uso.CostoUSD != 0.0012 {
		t.Fatalf("resultado: %+v", res)
	}
	if strings.Join(textos, "") != "Hola mundo" {
		t.Fatalf("texto en vivo: %q", textos)
	}
	r := leerRegistro(t, archivo)
	args := strings.Join(r.Args, " ")
	for _, esperado := range []string{"-p --output-format stream-json", "--strict-mcp-config", "--tools ", "--no-session-persistence",
		"--disable-slash-commands", "--bare", "--system-prompt-file", "--model haiku", "--effort max"} {
		if !strings.Contains(args, esperado) {
			t.Errorf("falta %q en los argumentos: %s", esperado, args)
		}
	}
	if i := slices.Index(r.Args, "--tools"); i < 0 || r.Args[i+1] != "" {
		t.Fatalf("sin herramientas: %v", r.Args)
	}
	if strings.Contains(args, "--json-schema") || !strings.Contains(r.Entrada, "hola") {
		t.Fatalf("sin esquema no hay --json-schema y el mensaje llega por stdin: %s %q", args, r.Entrada)
	}
	if v, _ := variable(r, "ANTHROPIC_API_KEY"); v != "sk-ant-prueba" {
		t.Fatalf("la clave llega a Claude: %q", v)
	}
	if _, ok := variable(r, "OPENAI_API_KEY"); ok {
		t.Fatal("OPENAI_API_KEY no debe llegar a Claude")
	}
}

// Con esquema, la salida estructurada nativa prevalece sobre el texto libre.
func TestClaudeSalidaEstructurada(t *testing.T) {
	archivo := falso(t, "estructurada.jsonl")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-prueba")
	s := solicitud("cuenta")
	s.Salida.Esquema = []byte(`{ "type": "object" }`)
	res, err := conClave().Ejecutar(context.Background(), s, sinEventos)
	if err != nil {
		t.Fatal(err)
	}
	if res.Salida != `{"total":3}` {
		t.Fatalf("salida estructurada: %q", res.Salida)
	}
	if r := leerRegistro(t, archivo); !strings.Contains(strings.Join(r.Args, " "), `--json-schema {"type":"object"}`) {
		t.Fatalf("el esquema va compacto en --json-schema: %v", r.Args)
	}
}

// Un error de la API llega clasificado y sin la clave; sin evento result es un error de protocolo.
func TestClaudeErrores(t *testing.T) {
	falso(t, "error.jsonl")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-prueba")
	_, err := conClave().Ejecutar(context.Background(), solicitud("hola"), sinEventos)
	if nucleo.CodigoDe(err) != nucleo.CodigoAutenticacionAgente || strings.Contains(nucleo.MensajeDe(err), "sk-ant-prueba") {
		t.Fatalf("error 401: %v", err)
	}
	falso(t, "sin-resultado.jsonl")
	if _, err := conClave().Ejecutar(context.Background(), solicitud("hola"), sinEventos); nucleo.CodigoDe(err) != nucleo.CodigoErrorProtocoloAgente {
		t.Fatalf("sin result: %v", err)
	}
	// Sin clave en el entorno, ClaveEntorno mira el almacén del sistema: con una guardada ahí, esta prueba no aplica.
	for _, nombre := range []string{"ANTHROPIC_API_KEY"} {
		if valor, _ := credenciales.Leer(nombre); valor != "" {
			t.Skip("hay una clave guardada en el almacén del sistema: " + nombre)
		}
	}
	t.Setenv("ANTHROPIC_API_KEY", "")
	if _, err := conClave().Ejecutar(context.Background(), solicitud("hola"), sinEventos); nucleo.CodigoDe(err) != nucleo.CodigoAutenticacionAgente {
		t.Fatalf("sin clave: %v", err)
	}
}

func TestContextoClaude(t *testing.T) {
	var r resultadoClaude
	datos := `{"result":"ok","modelUsage":{"claude-haiku-4-5":{"outputTokens":5,"contextWindow":200000},"aux":{"outputTokens":1,"contextWindow":100}}}`
	if err := json.Unmarshal([]byte(datos), &r); err != nil {
		t.Fatal(err)
	}
	if r.contexto() != 200000 {
		t.Fatalf("el contexto del modelo efectivo: %d", r.contexto())
	}
}

func TestModeloEfectivoEligeElDeMasSalida(t *testing.T) {
	var r resultadoClaude
	if err := json.Unmarshal([]byte(`{"modelUsage":{"auxiliar":{"outputTokens":3},"principal":{"outputTokens":40}}}`), &r); err != nil {
		t.Fatal(err)
	}
	if r.modeloEfectivo() != "principal" {
		t.Fatalf("modelo efectivo %q", r.modeloEfectivo())
	}
}

// La Web UI decide la ayuda por el código del problema, no por su texto (que cambia con el idioma).
func TestDetectarNoInstalado(t *testing.T) {
	sin := Claude{Programa: "tramtai-no-existe-xyz", Autenticacion: adaptadores.NuevoModo(adaptadores.AutenticacionSuscripcion)}
	if e := sin.Detectar(context.Background()); e.ProblemaCodigo != adaptadores.ProblemaNoInstalado {
		t.Fatalf("se esperaba no_instalado: %+v", e)
	}
}

func enSuscripcion() Claude {
	return Claude{Programa: os.Args[0], Autenticacion: adaptadores.NuevoModo(adaptadores.AutenticacionSuscripcion)}
}

func TestComprobarSesionOk(t *testing.T) {
	falso(t, "auth-ok.json")
	if s := enSuscripcion().ComprobarSesion(context.Background()); s.Estado != adaptadores.SesionOK || s.Cuenta != "persona@ejemplo.com" {
		t.Fatalf("sesión de la suscripción: %+v", s)
	}
}

func TestComprobarSesionSinSesion(t *testing.T) {
	falso(t, "auth-sin.json")
	if s := enSuscripcion().ComprobarSesion(context.Background()); s.Estado != adaptadores.SesionSin {
		t.Fatalf("sin sesión: %+v", s)
	}
}

func TestComprobarSesionUsaClave(t *testing.T) {
	falso(t, "auth-clave.json")
	if s := enSuscripcion().ComprobarSesion(context.Background()); s.Estado != adaptadores.SesionUsaClave {
		t.Fatalf("con clave de API: %+v", s)
	}
}

func TestComprobarSesionNoGastaMensajes(t *testing.T) {
	archivo := falso(t, "auth-ok.json")
	enSuscripcion().ComprobarSesion(context.Background())
	if r := leerRegistro(t, archivo); strings.Join(r.Args, " ") != "auth status --json" || r.Entrada != "" {
		t.Fatalf("solo se pregunta el estado: %q %q", r.Args, r.Entrada)
	}
}

func TestDetectarSuscripcionSinSesion(t *testing.T) {
	falso(t, "auth-sin.json")
	if e := enSuscripcion().Detectar(context.Background()); e.ProblemaCodigo != adaptadores.ProblemaFaltaClave {
		t.Fatalf("sin sesión, la tarjeta dice cómo iniciarla: %+v", e)
	}
	falso(t, "auth-ok.json")
	if e := enSuscripcion().Detectar(context.Background()); e.Cuenta != "persona@ejemplo.com" {
		t.Fatalf("con sesión, la cuenta: %+v", e)
	}
}
