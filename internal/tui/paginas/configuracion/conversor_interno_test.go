package configuracion

import (
	"strings"
	"testing"

	"tramtai/internal/contrato"
	"tramtai/internal/tui/pagina"
)

// TestRedEmpresaSoloConPaquete: tras un error, el paquete del conversor solo se ofrece si lo hay para el sistema del
// servicio (lo dice el servicio: solo se publica para Windows x64).
func TestRedEmpresaSoloConPaquete(t *testing.T) {
	p := Nueva("es").(*Pagina)
	p.estilos = &pagina.Estilos{Iconos: pagina.IconosUnicode}
	for _, paquete := range []bool{true, false} {
		p.conversor = &contrato.EstadoConversor{Estado: "error", Mensaje: "sin red", Paquete: paquete}
		texto := strings.Join(p.bloqueDe(secConversor).lineas, "\n")
		if strings.Contains(texto, "Si la red de tu empresa") != paquete {
			t.Errorf("paquete %v:\n%s", paquete, texto)
		}
	}
}
