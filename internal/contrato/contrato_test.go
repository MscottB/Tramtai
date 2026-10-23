package contrato

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDependencias comprueba que los clientes del servicio no enlazan el servicio: si uno vuelve a importar sus tipos
// desde servicio, la app de escritorio lleva otra vez dentro el planificador, la base y los adaptadores.
func TestDependencias(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("sin go en el PATH")
	}
	prohibidos := []string{"tramtai/internal/servicio", "tramtai/internal/planificador", "tramtai/internal/persistencia",
		"tramtai/internal/api", "tramtai/internal/actualizacion", "modernc.org/sqlite"}
	for _, c := range []struct{ dir, paquete string }{
		{filepath.Join("..", ".."), "./internal/cliente"},
		{filepath.Join("..", "..", "escritorio"), "."},
	} {
		orden := exec.Command("go", "list", "-deps", c.paquete)
		orden.Dir = c.dir
		salida, err := orden.Output()
		if err != nil {
			var fallo *exec.ExitError
			detalle := ""
			if errors.As(err, &fallo) {
				detalle = string(fallo.Stderr) // por ejemplo, un go.sum de la app de escritorio sin actualizar
			}
			t.Fatalf("go list -deps %s en %s: %v\n%s", c.paquete, c.dir, err, detalle)
		}
		for _, dep := range strings.Fields(string(salida)) {
			for _, p := range prohibidos {
				if dep == p {
					t.Errorf("%s (en %s) enlaza %s", c.paquete, c.dir, dep)
				}
			}
		}
	}
}
