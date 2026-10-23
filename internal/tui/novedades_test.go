package tui

import (
	"strings"
	"testing"

	"tramtai/internal/actualizacion"
	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
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

// TestUEnAjustesEsDeLaPagina: con Ajustes abierto, u es suya aunque no la enseñe en el pie (en el avance de una
// actualización, por ejemplo): la app no la vuelve a iniciar, y así no se pierde la cuenta atrás.
func TestUEnAjustesEsDeLaPagina(t *testing.T) {
	m := nuevoEn(Opciones{}, "es")
	falsa := &paginaFalsa{}
	m.paginas[pagina.PaginaConfiguracion] = falsa
	m.Update(tecla("7"))
	m.novedad = contrato.EstadoActualizacion{Disponible: true, Version: "v99.0.0"}
	iniciada := falsa.iniciada
	if _, cmd := m.Update(tecla("u")); cmd != nil || falsa.iniciada != iniciada || falsa.texto != "u" {
		t.Fatalf("u llega a Ajustes sin iniciarla de nuevo: iniciada %d→%d, texto %q", iniciada, falsa.iniciada, falsa.texto)
	}
}

// TestCabeceraSigueAAjustes: lo que Ajustes sabe de las actualizaciones (tras «Buscar ahora» o al activar o desactivar la
// búsqueda) llega a la cabecera al momento, como el estado compartido de la Web UI.
func TestCabeceraSigueAAjustes(t *testing.T) {
	m := nuevoEn(Opciones{}, "es")
	m.ancho = 120
	boton := datos.Texto("es", "actualizacion.boton")
	m.Update(pagina.EstadoActualizacion{Estado: contrato.EstadoActualizacion{Disponible: true, Version: "v99.0.0"}})
	if !strings.Contains(m.View().Content, boton) {
		t.Fatalf("hay versión nueva:\n%s", m.View().Content)
	}
	m.Update(pagina.EstadoActualizacion{Estado: contrato.EstadoActualizacion{Version: "v99.0.0"}})
	if strings.Contains(m.View().Content, boton) {
		t.Fatalf("sin búsqueda, el aviso se va:\n%s", m.View().Content)
	}
}

// TestNovedadesAlReconectar: cuando el servicio vuelve (o es otro, tras reiniciarlo o actualizar), la terminal vuelve a
// preguntar por las actualizaciones; una comprobación normal no.
func TestNovedadesAlReconectar(t *testing.T) {
	cli, _ := prueba.ConVersion(t, actualizacion.Version{Etiqueta: "v99.0.0"})
	boton := datos.Texto("es", "actualizacion.boton")
	m := nuevoEn(Opciones{}, "es")
	m.cli, m.ancho, m.preguntada = cli, 120, true // ya preguntó, al servicio de antes
	a := conducir(t, m)
	a.Correr(m.conectar(false, false, false))
	if strings.Contains(m.View().Content, boton) {
		t.Fatal("con el mismo servicio no se vuelve a preguntar")
	}
	m.caido = cliente.ErrServicioInactivo // se cayó y vuelve
	a.Correr(m.conectar(false, false, false))
	a.Hasta(func() bool { return strings.Contains(m.View().Content, boton) })
}
