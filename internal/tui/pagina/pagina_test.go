package pagina

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"slices"
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

// TestEnvolver: por palabras y cada párrafo por su lado; con un máximo de líneas, la última lleva lo que no cabe, cortado.
func TestEnvolver(t *testing.T) {
	e := &Estilos{Iconos: IconosUnicode}
	if l := e.Envolver("uno dos tres\ncuatro", 7, 0); !slices.Equal(l, []string{"uno dos", "tres", "cuatro"}) {
		t.Fatalf("por palabras y por párrafos: %q", l)
	}
	if l := e.Envolver("uno dos tres cuatro", 7, 2); !slices.Equal(l, []string{"uno dos", "tres c…"}) {
		t.Fatalf("en dos líneas, la última cortada: %q", l)
	}
}

// TestCompletarSoloCarpetas: para elegir una carpeta, los archivos no cuentan.
func TestCompletarSoloCarpetas(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "notas.txt"), nil, 0o600)
	os.Mkdir(filepath.Join(dir, "notas"), 0o700)
	base := dir + string(filepath.Separator)
	if r, c := CompletarRuta(base+"no", true); r != base+"notas"+string(filepath.Separator) || len(c) != 1 {
		t.Fatalf("solo la carpeta: %q %v", r, c)
	}
	if r, c := CompletarRuta(base+"no", false); r != base+"notas" || len(c) != 2 {
		t.Fatalf("con los archivos, lo común: %q %v", r, c)
	}
}

func TestNumero(t *testing.T) {
	for n, quiere := range map[int][2]string{999: {"999", "999"}, 1234: {"1234", "1,234"}, 12345: {"12.345", "12,345"}, -1234567: {"-1.234.567", "-1,234,567"}} {
		if es, en := Numero("es", n), Numero("en", n); es != quiere[0] || en != quiere[1] {
			t.Errorf("Numero(%d) = %q y %q", n, es, en)
		}
	}
}

// TestDecimal: como toLocaleString con esos decimales: coma en español, punto en inglés y los miles de Numero.
func TestDecimal(t *testing.T) {
	casos := []struct {
		x      float64
		d      int
		es, en string
	}{{0.0069, 4, "0,0069", "0.0069"}, {13.25, 1, "13,3", "13.3"}, {2, 2, "2,00", "2.00"}, {12345.5, 1, "12.345,5", "12,345.5"}}
	for _, c := range casos {
		if es, en := Decimal("es", c.x, c.d), Decimal("en", c.x, c.d); es != c.es || en != c.en {
			t.Errorf("Decimal(%v, %d) = %q y %q", c.x, c.d, es, en)
		}
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
