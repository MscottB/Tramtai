package adaptadores

import (
	"context"
	"strings"
	"testing"

	"tramtai/internal/nucleo"
)

func TestSepararUnMensajeSinEnmarcar(t *testing.T) {
	s := nucleo.SolicitudEjecucion{Instrucciones: "Sé breve.", Mensajes: []nucleo.Mensaje{
		{Rol: "sistema", Contenido: "Responde en español."}, {Rol: "usuario", Contenido: "hola"}}}
	sistema, prompt := separar(s)
	if sistema != "Sé breve.\n\nResponde en español." || prompt != "hola" {
		t.Fatalf("sistema %q prompt %q", sistema, prompt)
	}
}

func TestSepararConversacionConservaContenido(t *testing.T) {
	s := nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{
		{Rol: "usuario", Contenido: "a"}, {Rol: "asistente", Contenido: "b"}, {Rol: "usuario", Contenido: "c\nd"}}}
	_, prompt := separar(s)
	esperado := "<<<turno rol=usuario>>>\na\n<<<fin turno>>>\n<<<turno rol=asistente>>>\nb\n<<<fin turno>>>\n<<<turno rol=usuario>>>\nc\nd\n<<<fin turno>>>\n"
	if prompt != esperado {
		t.Fatalf("prompt %q", prompt)
	}
}

func TestClasificar(t *testing.T) {
	casos := map[string]nucleo.CodigoError{
		"Rate limit reached":         nucleo.CodigoLimiteUsoProveedor,
		"401 Unauthorized":           nucleo.CodigoAutenticacionAgente,
		"Invalid API key":            nucleo.CodigoAutenticacionAgente,
		"model gpt-x does not exist": nucleo.CodigoModeloNoDisponible,
		"algo inesperado":            nucleo.CodigoErrorAgente,
	}
	for texto, esperado := range casos {
		if c := clasificar(texto, 0); c != esperado {
			t.Errorf("%q: %s, se esperaba %s", texto, c, esperado)
		}
	}
	if clasificar("x", 429) != nucleo.CodigoLimiteUsoProveedor {
		t.Error("HTTP 429 debe ser limite_uso_proveedor")
	}
}

func TestEntornoLimpioQuitaCredenciales(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "secreto")
	for _, nombre := range []string{"TRAMTAI_TOKEN_GLOBAL", "TRAMTAI_OTRA", "GITHUB_TOKEN", "AWS_SECRET_ACCESS_KEY", "AZURE_CLIENT_SECRET", "DB_PASSWORD", "MI_CREDENTIAL"} {
		t.Setenv(nombre, "secreto")
	}
	t.Setenv("CLAUDE_CODE_TOKEN_EXTRA", "propia")
	t.Setenv("PATH_PRUEBA", "conservada")
	entorno := strings.Join(entornoLimpio("CODEX_API_KEY=propia", "MI_TOKEN=explicita"), "\n")
	if strings.Contains(entorno, "=secreto") {
		t.Fatalf("el entorno heredó una credencial ajena:\n%s", entorno)
	}
	for _, par := range []string{"CLAUDE_CODE_TOKEN_EXTRA=propia", "PATH_PRUEBA=conservada", "CODEX_API_KEY=propia", "MI_TOKEN=explicita"} {
		if !strings.Contains(entorno, par) {
			t.Errorf("falta %s: la configuración propia de las CLI y las variables indicadas se conservan", par)
		}
	}
}

func TestResolverProgramaRechazaInexistente(t *testing.T) {
	if _, err := resolverPrograma("tramtai-no-existe-xyz"); nucleo.CodigoDe(err) != nucleo.CodigoAgenteNoDisponible {
		t.Fatalf("se esperaba agente_no_disponible: %v", err)
	}
}

// La Web UI decide la ayuda por el código del problema, no por su texto (que cambia con el idioma).
func TestDetectarClasificaProblema(t *testing.T) {
	sin := Claude{Programa: "tramtai-no-existe-xyz", Autenticacion: NuevoModo(AutenticacionSuscripcion)}
	if e := sin.Detectar(context.Background()); e.ProblemaCodigo != ProblemaNoInstalado {
		t.Fatalf("se esperaba no_instalado: %+v", e)
	}
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	if claveEntorno("GEMINI_API_KEY", "GOOGLE_API_KEY") != "" {
		t.Skip("hay una clave de Gemini en el almacén de credenciales")
	}
	if e := (Gemini{}).Detectar(context.Background()); e.ProblemaCodigo != ProblemaFaltaClave {
		t.Fatalf("se esperaba falta_clave: %+v", e)
	}
}

func TestModeloEfectivoEligeElDeMasSalida(t *testing.T) {
	var r resultadoClaude
	r.ModelUsage = map[string]struct {
		OutputTokens int `json:"outputTokens"`
	}{"auxiliar": {OutputTokens: 3}, "principal": {OutputTokens: 40}}
	if r.modeloEfectivo() != "principal" {
		t.Fatalf("modelo efectivo %q", r.modeloEfectivo())
	}
}

func TestCambiarAutenticacion(t *testing.T) {
	c := Claude{Autenticacion: NuevoModo(AutenticacionClaveAPI)}
	registro := map[string]Adaptador{"claude": c, "gemini": Gemini{}}
	if err := CambiarAutenticacion(registro, "claude", AutenticacionSuscripcion); err != nil || c.Autenticacion.Valor() != AutenticacionSuscripcion {
		t.Fatalf("%v %s", err, c.Autenticacion.Valor())
	}
	if CambiarAutenticacion(registro, "gemini", AutenticacionSuscripcion) == nil || CambiarAutenticacion(registro, "claude", "otra") == nil {
		t.Fatal("gemini no admite suscripción y un modo desconocido se rechaza")
	}
	if (Claude{}).Autenticacion.Valor() != AutenticacionClaveAPI {
		t.Fatal("sin modo, clave de API")
	}
}
