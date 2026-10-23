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

// El paquete, y no solo la CLI, rechaza un nombre que no es una de las claves conocidas: nadie escribe ni borra
// tramtai/<cualquier cosa> en el almacén.
func TestSoloNombresAdmitidos(t *testing.T) {
	if err := Borrar("TRAMTAI_PRUEBA_NO_ADMITIDA"); err == nil {
		t.Error("Borrar aceptó un nombre que no está en Nombres")
	}
	if err := Guardar("TRAMTAI_PRUEBA_NO_ADMITIDA", "sk-prueba-123"); err == nil {
		Borrar("TRAMTAI_PRUEBA_NO_ADMITIDA")
		t.Error("Guardar aceptó un nombre que no está en Nombres")
	}
}

// secret-tool sale con 1 tanto si la clave no existe como si el llavero no responde: solo en el segundo caso explica
// algo por stderr, y entonces es un error y no «no configurada».
func TestClaveAusente(t *testing.T) {
	casos := []struct {
		codigo          int
		salida, errores string
		ausente         bool
	}{
		{1, "", "", true},
		{1, "", "Cannot autolaunch D-Bus without X11 $DISPLAY", false},
		{0, "sk-123", "", false},
		{2, "", "", false},
	}
	for _, c := range casos {
		if claveAusente(c.codigo, 1, c.salida, c.errores) != c.ausente {
			t.Errorf("%+v: se esperaba ausente=%v", c, c.ausente)
		}
	}
}

func TestAlmacenTieneNombre(t *testing.T) {
	if Disponible() && Almacen() == "" {
		t.Fatal("un almacén disponible debe decir cuál es, para las instrucciones de la interfaz")
	}
}
