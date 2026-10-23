package tui

import (
	"strings"
	"testing"

	"tramtai/internal/actualizacion"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/prueba"
)

// TestNovedadesAviso: al conectar, la terminal pregunta al servicio, en segundo plano, si hay una versión nueva (su
// Comprobador, aquí uno falso). Con una, la cabecera lo avisa y u lleva a sus novedades en Ajustes; sin ella, no hay aviso.
func TestNovedadesAviso(t *testing.T) {
	boton := datos.Texto("es", "actualizacion.boton")
	for _, c := range []struct {
		version string
		nueva   bool
	}{{"v99.0.0", true}, {"v0.0.1", false}} {
		cli, _ := prueba.ConVersion(t, actualizacion.Version{Etiqueta: c.version, Notas: "Novedades:\n- Algo nuevo"})
		m := nuevoEn(Opciones{}, "es")
		m.cli, m.ancho = cli, 120
		a := conducir(t, m)
		a.Correr(m.conectar(false, true, false))
		if !c.nueva {
			if strings.Contains(m.View().Content, boton) {
				t.Fatalf("sin versión nueva no hay aviso:\n%s", m.View().Content)
			}
			continue
		}
		a.Hasta(func() bool { return strings.Contains(strings.Split(m.View().Content, "\n")[0], boton+" · u") })
		a.Pulsar("u")
		a.HastaVer(datos.Textof("es", "actualizacion.titulo", "version", c.version), "Algo nuevo")
		if m.actual != pagina.PaginaConfiguracion {
			t.Fatalf("u lleva a Ajustes: %s", m.actual)
		}
	}
}
