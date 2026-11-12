package agentes

import (
	"testing"

	"tramtai/internal/preparar"
)

// TestRecetasBorranSoloLoSuyo: lo que una receta borra al desinstalar empieza por «~/», no lleva «..» y no es una
// carpeta que compartan otros programas (~/.local, ~/AppData/Local…).
func TestRecetasBorranSoloLoSuyo(t *testing.T) {
	for _, f := range Todos() {
		if f.Receta == nil {
			continue
		}
		rutas := f.Receta.DatosCLI
		for _, lista := range f.Receta.Borrar {
			rutas = append(rutas, lista...)
		}
		for _, r := range rutas {
			if err := preparar.RutaBorrable(r); err != nil {
				t.Errorf("%s: %v", f.Nombre, err)
			}
		}
	}
	for _, mala := range []string{"~", "~/", "~/.", "~/..", "~/../x", "~/a/../b", "/usr/local/bin/x", "x", "~/.local", "~/AppData/local"} {
		if preparar.RutaBorrable(mala) == nil {
			t.Errorf("%q no se puede borrar", mala)
		}
	}
}
