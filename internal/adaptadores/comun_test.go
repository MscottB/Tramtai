package adaptadores

import (
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
	for _, par := range entornoLimpio("CODEX_API_KEY=propia") {
		if par == "OPENAI_API_KEY=secreto" {
			t.Fatal("el entorno heredó una credencial ajena")
		}
	}
}

func TestResolverProgramaRechazaInexistente(t *testing.T) {
	if _, err := resolverPrograma("tramtai-no-existe-xyz"); nucleo.CodigoDe(err) != nucleo.CodigoAgenteNoDisponible {
		t.Fatalf("se esperaba agente_no_disponible: %v", err)
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
