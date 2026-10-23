package pagina

import (
	"context"
	"net/url"
	"syscall"
	"testing"

	"tramtai/internal/cliente"
	"tramtai/internal/nucleo"
)

func TestAvisoError(t *testing.T) {
	var e *Estilos
	l := e.AvisoError("es", nucleo.CodigoAgenteDesconectado, "el agente claude está desconectado")
	if len(l) != 3 || l[0] != "Ese agente está desconectado" || l[1] != "el agente claude está desconectado" {
		t.Fatalf("título y mensaje: %q", l)
	}
	if l[2] == "" || l[2] == "ayuda.problemas.agente_desconectado" {
		t.Errorf("falta qué hacer: %q", l[2])
	}
	if l := e.AvisoError("en", "codigo_nuevo", ""); len(l) != 1 || l[0] != "codigo_nuevo" {
		t.Errorf("un código sin texto se enseña tal cual, sin mensaje ni paso: %q", l)
	}
}

func TestNoResponde(t *testing.T) {
	casos := []struct {
		err      error
		esperado bool
	}{
		{cliente.ErrServicioInactivo, true},
		{&url.Error{Op: "Get", Err: syscall.ECONNREFUSED}, true},
		{context.DeadlineExceeded, true},
		{&url.Error{Op: "Get", Err: context.DeadlineExceeded}, true},
		{context.Canceled, false},
		{&url.Error{Op: "Get", Err: context.Canceled}, false},
		{nucleo.NuevoError(nucleo.CodigoEspacioDetenido, "x"), false},
	}
	for _, c := range casos {
		if NoResponde(c.err) != c.esperado {
			t.Errorf("NoResponde(%v) debía ser %v", c.err, c.esperado)
		}
	}
}
