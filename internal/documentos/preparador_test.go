package documentos

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"tramtai/internal/nucleo"
)

func TestNecesitaConversor(t *testing.T) {
	casos := []struct {
		nombre string
		quiere bool
	}{
		{"informe.PDF", true}, {"tabla.xlsx", true}, {"pagina.htm", true},
		{"notas.md", false}, {"datos.csv", false}, {"informes.zip", false},
	}
	for _, c := range casos {
		if got := NecesitaConversor(c.nombre); got != c.quiere {
			t.Errorf("%s: %v, se esperaba %v", c.nombre, got, c.quiere)
		}
	}
}

// preparadorCon guarda los archivos dados como cargas y devuelve sus identificadores, con el conversor buscado en una
// raíz vacía y sin markitdown en el PATH.
func preparadorCon(t *testing.T, nombres ...string) (Preparador, []string) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	cargas, err := AbrirCargas(t.TempDir(), time.Hour, 1<<20, 0)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, n := range nombres {
		c, err := cargas.Guardar(n, "", strings.NewReader("contenido de "+n))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, c.ID)
	}
	return Preparador{Cargas: cargas, Conversor: &Conversor{Comando: "markitdown", Raiz: t.TempDir()}}, ids
}

func TestConversorListoSinConversor(t *testing.T) {
	p, ids := preparadorCon(t, "notas.txt", "informe.pdf")
	if err := p.ConversorListo(ids); nucleo.CodigoDe(err) != nucleo.CodigoConversorNoDisponible {
		t.Fatalf("un PDF sin conversor: %v", err)
	}
}

func TestConversorListoSoloTexto(t *testing.T) {
	p, ids := preparadorCon(t, "notas.txt", "datos.json")
	if err := p.ConversorListo(ids); err != nil {
		t.Fatalf("el texto no necesita conversor: %v", err)
	}
}

func TestConversorListoPreparando(t *testing.T) {
	p, ids := preparadorCon(t, "informe.pdf")
	t.Setenv("UV_FALSO", "colgar")
	prep := NuevaPreparacion(p.Conversor.Raiz, "markitdown", os.Args[0])
	prep.plazo = 2 * time.Second
	p.Preparacion = prep
	prep.Preparar()
	t.Cleanup(func() { esperar(t, prep) })
	var preparando *nucleo.ConversorPreparando
	if err := p.ConversorListo(ids); !errors.As(err, &preparando) || nucleo.CodigoDe(err) != nucleo.CodigoConversorNoDisponible {
		t.Fatalf("mientras se prepara: %v", err)
	}
}
