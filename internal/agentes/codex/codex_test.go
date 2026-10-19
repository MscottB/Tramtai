package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func conClave() Codex {
	return Codex{Programa: os.Args[0], Autenticacion: adaptadores.NuevoModo(adaptadores.AutenticacionClaveAPI)}
}

// Con clave de API, Codex va en solo lectura, sin shell ni búsqueda web, sin historial, sin las reglas del usuario
// (CODEX_HOME privado) y sin las credenciales de otros agentes.
func TestCodexConClave(t *testing.T) {
	archivo := falso(t, "salida.jsonl")
	t.Setenv("OPENAI_API_KEY", "sk-prueba-codex")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-de-otro-agente")
	s := solicitud("hola")
	s.Modelo, s.Esfuerzo = "gpt-5", "alto"
	s.Salida.Esquema = []byte(`{"type":"object"}`)
	var textos []string
	res, err := conClave().Ejecutar(context.Background(), s, func(e adaptadores.Evento) error {
		textos = append(textos, e.Datos)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Salida != "hola desde codex" || res.Uso.TokensEntrada != 120 || res.Uso.TokensSalida != 7 || res.ModeloEfectivo != "" {
		t.Fatalf("resultado: %+v", res)
	}
	if len(textos) != 1 || textos[0] != "hola desde codex" {
		t.Fatalf("solo el mensaje del agente llega como texto (no el razonamiento): %q", textos)
	}
	r := leerRegistro(t, archivo)
	args := strings.Join(r.Args, " ")
	for _, esperado := range []string{"exec --json --ephemeral", "-s read-only", `approval_policy="never"`, "features.shell_tool=false",
		`web_search="disabled"`, `history.persistence="none"`, "--ignore-rules", "-m gpt-5", `model_reasoning_effort="high"`, "--output-schema"} {
		if !strings.Contains(args, esperado) {
			t.Errorf("falta %q en los argumentos: %s", esperado, args)
		}
	}
	if r.Args[len(r.Args)-1] != "-" || !strings.Contains(r.Entrada, "hola") || !strings.Contains(r.Entrada, "Sé breve.") {
		t.Fatalf("el mensaje llega por stdin con las instrucciones: %v %q", r.Args, r.Entrada)
	}
	if v, _ := variable(r, "CODEX_API_KEY"); v != "sk-prueba-codex" {
		t.Fatalf("la clave llega como CODEX_API_KEY: %q", v)
	}
	if home, ok := variable(r, "CODEX_HOME"); !ok || home == "" {
		t.Fatal("con clave de API, CODEX_HOME es privado")
	}
	for _, nombre := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY"} {
		if _, ok := variable(r, nombre); ok {
			t.Errorf("%s no debe llegar a Codex", nombre)
		}
	}
}

func TestCodexSinClave(t *testing.T) {
	// Sin clave en el entorno, ClaveEntorno mira el almacén del sistema: con una guardada ahí, esta prueba no aplica.
	for _, nombre := range []string{"OPENAI_API_KEY", "CODEX_API_KEY"} {
		if valor, _ := credenciales.Leer(nombre); valor != "" {
			t.Skip("hay una clave guardada en el almacén del sistema: " + nombre)
		}
	}
	falso(t, "salida.jsonl")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("CODEX_API_KEY", "")
	if _, err := conClave().Ejecutar(context.Background(), solicitud("hola"), sinEventos); nucleo.CodigoDe(err) != nucleo.CodigoAutenticacionAgente {
		t.Fatalf("sin clave: %v", err)
	}
	if e := conClave().Detectar(context.Background()); e.ProblemaCodigo != adaptadores.ProblemaFaltaClave {
		t.Fatalf("Detectar avisa de la clave: %+v", e)
	}
}

// Con suscripción usa la sesión de codex login: ni CODEX_HOME privado ni clave, y avisa si hay un AGENTS.md del usuario.
func TestCodexConSuscripcion(t *testing.T) {
	archivo := falso(t, "salida.jsonl")
	inicio := t.TempDir()
	t.Setenv("USERPROFILE", inicio)
	t.Setenv("HOME", inicio)
	c := Codex{Programa: os.Args[0], Autenticacion: adaptadores.NuevoModo(adaptadores.AutenticacionSuscripcion)}
	if _, err := c.Ejecutar(context.Background(), solicitud("hola"), sinEventos); err != nil {
		t.Fatal(err)
	}
	r := leerRegistro(t, archivo)
	for _, nombre := range []string{"CODEX_HOME", "CODEX_API_KEY"} {
		if _, ok := variable(r, nombre); ok {
			t.Errorf("con suscripción no se fija %s", nombre)
		}
	}
	if c.ModoAutenticacion().Valor() != adaptadores.AutenticacionSuscripcion {
		t.Fatal("ModoAutenticacion devuelve el modo en marcha")
	}
	os.MkdirAll(filepath.Join(inicio, ".codex"), 0o700)
	os.WriteFile(filepath.Join(inicio, ".codex", "AGENTS.md"), []byte("reglas"), 0o600)
	if e := c.Detectar(context.Background()); e.ProblemaCodigo != adaptadores.ProblemaAviso {
		t.Fatalf("avisa de ~/.codex/AGENTS.md: %+v", e)
	}
}

// Un error del CLI llega con su mensaje y sin la clave; un JSONL cortado (sin turn.completed) es un error de protocolo.
func TestCodexErrores(t *testing.T) {
	falso(t, "fallo.jsonl")
	t.Setenv("OPENAI_API_KEY", "sk-prueba-codex")
	t.Setenv("FALSO_ERRORES", "usando la clave sk-prueba-codex")
	_, err := conClave().Ejecutar(context.Background(), solicitud("hola"), sinEventos)
	if err == nil || !strings.Contains(nucleo.MensajeDe(err), "401") || strings.Contains(nucleo.MensajeDe(err), "sk-prueba-codex") {
		t.Fatalf("error del CLI: %v", err)
	}
	falso(t, "cortado.jsonl")
	t.Setenv("FALSO_ERRORES", "")
	if _, err := conClave().Ejecutar(context.Background(), solicitud("hola"), sinEventos); nucleo.CodigoDe(err) != nucleo.CodigoErrorProtocoloAgente {
		t.Fatalf("sin turn.completed: %v", err)
	}
}
