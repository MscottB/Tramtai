package credenciales

import (
	"strings"
	"testing"
)

func TestValorValido(t *testing.T) {
	for _, bien := range []string{"sk-ant-api03_ABC-123.xyz", "AIzaSyD0123456789", "sk-proj-abc_DEF"} {
		if err := valorValido(bien); err != nil {
			t.Errorf("%q: %v", bien, err)
		}
	}
	// Una clave con espacios, comillas o saltos de línea no es una clave de API y rompería el paso al Llavero.
	// Solo ASCII imprimible: el Llavero devuelve en hexadecimal lo que no lo es, y la comprobación al guardar fallaría.
	for _, mal := range []string{"", "con espacio", `comilla"`, "simple'", "a\nb", "barra\\", "clavé", strings.Repeat("x", 2049)} {
		if valorValido(mal) == nil {
			t.Errorf("%.20q (%d bytes) debía rechazarse", mal, len(mal))
		}
	}
}

func TestAlmacenTieneNombre(t *testing.T) {
	if Disponible() && Almacen() == "" {
		t.Fatal("un almacén disponible debe decir cuál es, para las instrucciones de la interfaz")
	}
}
