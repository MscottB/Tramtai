package autoarranque

import "testing"

// ModoDe lee lo que escribe el usuario (--modo): app, servicio o su alias en inglés; sin nada, la app de siempre.
func TestModoDe(t *testing.T) {
	for texto, quiero := range map[string]Modo{"": ModoApp, "app": ModoApp, "servicio": ModoServicio, "service": ModoServicio} {
		if modo, err := ModoDe(texto); err != nil || modo != quiero {
			t.Errorf("ModoDe(%q) = %q %v; se esperaba %q", texto, modo, err, quiero)
		}
	}
	for _, texto := range []string{"otro", "App", "servicios"} {
		if modo, err := ModoDe(texto); err == nil || modo != "" {
			t.Errorf("ModoDe(%q) = %q %v; debía fallar", texto, modo, err)
		}
	}
}
