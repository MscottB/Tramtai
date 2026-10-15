package instalacion

import "testing"

func TestEditarRuta(t *testing.T) {
	const dir = `C:\Users\yo\AppData\Local\Programs\Tramtai`
	casos := []struct {
		nombre, ruta string
		agregar      bool
		esperada     string
	}{
		{"añade a un PATH vacío", "", true, dir},
		{"añade al final y conserva las variables", `C:\a;%USERPROFILE%\b`, true, `C:\a;%USERPROFILE%\b;` + dir},
		{"añade antes del ; final", `C:\a;C:\b;`, true, `C:\a;C:\b;` + dir + `;`},
		{"no duplica aunque cambien mayúsculas o la barra final", `C:\a;c:\users\YO\appdata\local\programs\tramtai\`, true, `C:\a;c:\users\YO\appdata\local\programs\tramtai\`},
		{"quita solo su entrada", `C:\a;;` + dir + `\;C:\b`, false, `C:\a;;C:\b`},
		{"quitar algo que no está no cambia nada", `C:\a;;C:\b`, false, `C:\a;;C:\b`},
	}
	for _, c := range casos {
		if r := editarRuta(c.ruta, dir, c.agregar); r != c.esperada {
			t.Errorf("%s: %q, se esperaba %q", c.nombre, r, c.esperada)
		}
	}
	// Instalar y desinstalar deja cualquier PATH exactamente como estaba.
	for _, ruta := range []string{"", `C:\a`, `C:\a;`, `C:\a;;C:\b;`, `;C:\a`, `;`, `%USERPROFILE%\x;C:\b;;`} {
		if r := editarRuta(editarRuta(ruta, dir, true), dir, false); r != ruta {
			t.Errorf("ida y vuelta de %q dio %q", ruta, r)
		}
	}
	if r := editarRuta(`C:\a`, "  ", true); r != `C:\a` {
		t.Errorf("una carpeta vacía no debe tocar el PATH: %q", r)
	}
}
