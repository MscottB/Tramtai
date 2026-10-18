package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

// registro es lo que el copilot falso apunta de cada ejecución.
type registro struct {
	Entrada string   `json:"entrada"`
	Args    []string `json:"args"`
	Entorno []string `json:"entorno"`
	Home    string   `json:"home"`
	Config  string   `json:"config"`
}

// TestMain hace de copilot falso cuando FALSO_COPILOT_REGISTRO está definido (Tramtai quita las variables COPILOT_*
// heredadas y las credenciales, no esta): apunta lo que recibe y responde con las muestras de testdata.
func TestMain(m *testing.M) {
	if archivo := os.Getenv("FALSO_COPILOT_REGISTRO"); archivo != "" {
		entrada, _ := io.ReadAll(os.Stdin)
		home := os.Getenv("COPILOT_HOME")
		config, _ := os.ReadFile(filepath.Join(home, "config.json"))
		datos, _ := json.Marshal(registro{Entrada: string(entrada), Args: os.Args[1:], Entorno: os.Environ(), Home: home, Config: string(config)})
		os.WriteFile(archivo, datos, 0o600)
		if os.Getenv("FALSO_COPILOT_FALLO") != "" {
			fmt.Fprintf(os.Stderr, "Error: Model \"%s\" from --model flag is not available. %s\n", os.Args[4], os.Getenv("COPILOT_GITHUB_TOKEN"))
			os.Exit(1)
		}
		muestras := os.Getenv("FALSO_COPILOT_DATOS")
		if i := slices.Index(os.Args, "--usage-output-file"); i > 0 {
			uso, _ := os.ReadFile(filepath.Join(muestras, "uso.json"))
			os.WriteFile(os.Args[i+1], uso, 0o600)
		}
		salida, _ := os.ReadFile(filepath.Join(muestras, "salida.jsonl"))
		os.Stdout.Write(salida)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// falso prepara el copilot falso y devuelve dónde deja su registro.
func falso(t *testing.T) string {
	t.Helper()
	muestras, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	archivo := filepath.Join(t.TempDir(), "registro.json")
	t.Setenv("FALSO_COPILOT_REGISTRO", archivo)
	t.Setenv("FALSO_COPILOT_DATOS", muestras)
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

// sesionDeUsuario deja un config.json como el de copilot login, con comentarios y otras claves.
func sesionDeUsuario(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	config := "// comentario de Copilot\n// otro\n{\n  \"loggedInUsers\": [{\"host\": \"https://github.com\", \"login\": \"usuario-prueba\", \"kind\": \"user\"}],\n" +
		"  \"lastLoggedInUser\": {\"host\": \"https://github.com\", \"login\": \"usuario-prueba\"},\n  \"theme\": \"oscuro\",\n  \"trustedFolders\": [\"C:/secreto\"]\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COPILOT_HOME", dir)
}

func suscripcion() Copilot {
	return Copilot{Programa: os.Args[0], Autenticacion: adaptadores.NuevoModo(adaptadores.AutenticacionSuscripcion)}
}

func solicitud(texto string) nucleo.SolicitudEjecucion {
	return nucleo.SolicitudEjecucion{Instrucciones: "Sé breve.", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}}
}

func TestCopilotConSesion(t *testing.T) {
	archivo := falso(t)
	sesionDeUsuario(t)
	t.Setenv("GH_TOKEN", "gh-token-de-prueba")
	t.Setenv("OPENAI_API_KEY", "clave-openai-de-prueba")
	t.Setenv("COPILOT_PROVIDER_API_KEY", "clave-byok-de-prueba")
	t.Setenv("COPILOT_PROVIDER_BASE_URL", "http://127.0.0.1:1")

	var textos, avisos []string
	r, err := suscripcion().Ejecutar(context.Background(), solicitud("Responde solo con la palabra: piña"), func(e adaptadores.Evento) error {
		switch e.Tipo {
		case "texto":
			textos = append(textos, e.Datos)
		case "aviso":
			avisos = append(avisos, e.Datos)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Salida != "piña" || strings.Join(textos, "|") != "pi|ña" || r.ModeloEfectivo != "mai-code-1.1-flash" {
		t.Fatalf("salida %q, en vivo %q, modelo %q", r.Salida, textos, r.ModeloEfectivo)
	}
	if r.Uso.TokensEntrada != 12160+40+100 || r.Uso.TokensSalida != 7 {
		t.Fatalf("uso %+v", r.Uso)
	}
	if len(avisos) != 1 || !strings.Contains(avisos[0], "1") {
		t.Fatalf("avisos %q", avisos)
	}

	reg := leerRegistro(t, archivo)
	if !strings.Contains(reg.Entrada, "Responde solo con la palabra: piña") || !strings.Contains(reg.Entrada, "Sé breve.") {
		t.Fatalf("el mensaje no llegó por stdin: %q", reg.Entrada)
	}
	for _, a := range []string{"--available-tools none", "--model auto", "--disallow-temp-dir", "--disable-builtin-mcps",
		"--no-ask-user", "--no-auto-update", "--no-custom-instructions", "--output-format json"} {
		if !strings.Contains(" "+strings.Join(reg.Args, " ")+" ", " "+a+" ") {
			t.Errorf("falta %s en %q", a, reg.Args)
		}
	}
	if slices.Contains(reg.Args, "--reasoning-effort") || slices.Contains(reg.Args, "-p") {
		t.Errorf("con auto no va el esfuerzo y el mensaje no va en la línea de órdenes: %q", reg.Args)
	}
	for _, par := range reg.Entorno {
		nombre, _, _ := strings.Cut(strings.ToUpper(par), "=")
		if nombre == "GH_TOKEN" || nombre == "OPENAI_API_KEY" || strings.HasPrefix(nombre, "COPILOT_PROVIDER") || nombre == "COPILOT_GITHUB_TOKEN" {
			t.Errorf("le llegó %s", nombre)
		}
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal([]byte(reg.Config), &config); err != nil {
		t.Fatalf("config.json privado: %v (%q)", err, reg.Config)
	}
	if len(config) != 2 || config["loggedInUsers"] == nil || config["lastLoggedInUser"] == nil {
		t.Fatalf("el config.json privado solo debe tener la cuenta: %s", reg.Config)
	}
	if reg.Home == os.Getenv("COPILOT_HOME") {
		t.Fatal("usó el COPILOT_HOME del usuario")
	}
	if _, err := os.Stat(reg.Home); !os.IsNotExist(err) {
		t.Fatalf("la carpeta privada quedó: %v", err)
	}
}

func TestCopilotConClave(t *testing.T) {
	archivo := falso(t)
	t.Setenv("COPILOT_GITHUB_TOKEN", "github_pat_de_prueba_123")
	c := Copilot{Programa: os.Args[0], Autenticacion: adaptadores.NuevoModo(adaptadores.AutenticacionClaveAPI)}
	s := solicitud("hola")
	s.Modelo, s.Esfuerzo = "gpt-6-luna", "alto"
	if _, err := c.Ejecutar(context.Background(), s, func(adaptadores.Evento) error { return nil }); err != nil {
		t.Fatal(err)
	}
	reg := leerRegistro(t, archivo)
	if !strings.Contains(strings.Join(reg.Args, " "), "--model gpt-6-luna --") || !strings.Contains(strings.Join(reg.Args, " "), "--reasoning-effort high") {
		t.Fatalf("argumentos %q", reg.Args)
	}
	if !slices.Contains(reg.Entorno, "COPILOT_GITHUB_TOKEN=github_pat_de_prueba_123") || reg.Config != "" {
		t.Fatalf("con clave llega el token y no hay sesión: config %q", reg.Config)
	}
}

func TestCopilotModeloNoDisponible(t *testing.T) {
	falso(t)
	t.Setenv("FALSO_COPILOT_FALLO", "1")
	t.Setenv("COPILOT_GITHUB_TOKEN", "github_pat_de_prueba_123")
	c := Copilot{Programa: os.Args[0], Autenticacion: adaptadores.NuevoModo(adaptadores.AutenticacionClaveAPI)}
	s := solicitud("hola")
	s.Modelo = "modelo-que-no-existe"
	_, err := c.Ejecutar(context.Background(), s, func(adaptadores.Evento) error { return nil })
	if nucleo.CodigoDe(err) != nucleo.CodigoModeloNoDisponible {
		t.Fatalf("error %v", err)
	}
	if strings.Contains(err.Error(), "github_pat_de_prueba_123") {
		t.Fatalf("el error muestra la clave: %v", err)
	}
}

func TestCopilotSinSesion(t *testing.T) {
	t.Setenv("COPILOT_HOME", t.TempDir())
	_, err := suscripcion().Ejecutar(context.Background(), solicitud("hola"), func(adaptadores.Evento) error { return nil })
	if nucleo.CodigoDe(err) != nucleo.CodigoAutenticacionAgente {
		t.Fatalf("sin sesión: %v", err)
	}
	if e := suscripcion().Detectar(context.Background()); e.ProblemaCodigo != adaptadores.ProblemaFaltaClave {
		t.Fatalf("Detectar sin sesión: %+v", e)
	}
	sesionDeUsuario(t)
	if e := suscripcion().Detectar(context.Background()); e.Problema != "" {
		t.Fatalf("Detectar con sesión: %+v", e)
	}
}

func TestProgramaNativoEnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("el envoltorio .cmd de npm solo existe en Windows")
	}
	dir := t.TempDir()
	plataforma := "copilot-win32-" + map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	exe := filepath.Join(dir, "node_modules", "@github", "copilot", "node_modules", "@github", plataforma, "copilot.exe")
	if err := os.MkdirAll(filepath.Dir(exe), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{exe, filepath.Join(dir, "copilot.cmd")} {
		if err := os.WriteFile(f, []byte("x"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	if ruta, err := programa("copilot"); err != nil || ruta != exe {
		t.Fatalf("programa = %q, %v", ruta, err)
	}
	os.Remove(exe)
	if _, err := programa("copilot"); err == nil {
		t.Fatal("sin el .exe nativo, el .cmd no se acepta")
	}
}
