package pagina

import (
	"context"
	"net/url"
	"syscall"
	"testing"

	"tramtai/internal/cliente"
	"tramtai/internal/nucleo"
)

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
