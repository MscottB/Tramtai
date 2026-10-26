package main

import (
	"errors"
	"slices"
	"testing"
	"time"

	"tramtai/internal/contrato"
)

func TestResultadoBusqueda(t *testing.T) {
	ahora := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	if a := resultadoBusqueda(contrato.EstadoActualizacion{}, errors.New("sin servicio"), ahora); a != (actualizacionMenu{fallo: true}) {
		t.Errorf("sin respuesta: %+v", a)
	}
	if a := resultadoBusqueda(contrato.EstadoActualizacion{Actual: "v0.56.1", Fallo: true}, nil, ahora); a != (actualizacionMenu{fallo: true}) {
		t.Errorf("GitHub no respondió: %+v", a)
	}
	if a := resultadoBusqueda(contrato.EstadoActualizacion{Actual: "v0.56.1", Disponible: true, Version: "v0.57.0", Instalable: true}, nil, ahora); a != (actualizacionMenu{nueva: "v0.57.0", instalable: true}) {
		t.Errorf("con versión nueva: %+v", a)
	}
	// GitHub falló ahora, pero el servicio conserva la versión nueva de la consulta anterior: se ofrece, como en la Web UI.
	if a := resultadoBusqueda(contrato.EstadoActualizacion{Actual: "v0.56.1", Disponible: true, Version: "v0.57.0", Fallo: true}, nil, ahora); a != (actualizacionMenu{nueva: "v0.57.0"}) {
		t.Errorf("versión nueva con fallo: %+v", a)
	}
	a := resultadoBusqueda(contrato.EstadoActualizacion{Actual: "v0.56.1"}, nil, ahora)
	if a.alDia != "v0.56.1" {
		t.Fatalf("al día: %+v", a)
	}
	// «Al día» se enseña plazoAlDia; después vuelve «Buscar actualizaciones».
	if a.vigente(ahora.Add(plazoAlDia-time.Second)).alDia != "v0.56.1" {
		t.Error("«al día» se quitó antes de tiempo")
	}
	if v := a.vigente(ahora.Add(plazoAlDia)); v.alDia != "" {
		t.Errorf("«al día» sigue tras el plazo: %+v", v)
	}
}

// avance simula las respuestas del progreso: una por consulta, en orden; un error sin fase es que el servicio no respondió.
type avance struct {
	fase string
	err  error
}

// seguir pasa las respuestas por seguirAvance, sin esperas: cada espera adelanta paso un reloj falso. Devuelve las fases
// que recibió el menú y el resultado.
func seguir(t *testing.T, paso time.Duration, respuestas ...avance) ([]string, error) {
	t.Helper()
	var fases []string
	reloj := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	err := seguirAvance(func() (contrato.ProgresoActualizacion, error) {
		if len(respuestas) == 0 {
			t.Fatal("siguió preguntando cuando ya no había nada que esperar")
		}
		r := respuestas[0]
		respuestas = respuestas[1:]
		p := contrato.ProgresoActualizacion{Fase: r.fase}
		if r.fase == "error" {
			p.Error = "firma no válida"
		}
		return p, r.err
	}, func() { reloj = reloj.Add(paso) }, func(f string) { fases = append(fases, f) }, func() time.Time { return reloj })
	return fases, err
}

func TestSeguirAvance(t *testing.T) {
	caido := errors.New("connection refused")
	t.Run("el instalador detiene el servicio y vuelve sin actualización en curso", func(t *testing.T) {
		fases, err := seguir(t, cadaAvance, avance{fase: "descargando"}, avance{err: caido}, avance{fase: "descargando"}, avance{fase: "verificando"},
			avance{fase: "listo"}, avance{fase: "instalando"}, avance{err: caido}, avance{err: caido}, avance{})
		// Mientras el servicio no responde queda la última fase; el instalador suele cerrar la app antes.
		if err != nil || !slices.Equal(fases, []string{"descargando", "descargando", "verificando", "listo", "instalando", ""}) {
			t.Errorf("fases %v, error %v", fases, err)
		}
	})
	t.Run("en la cuenta atrás reinician el servicio", func(t *testing.T) {
		fases, err := seguir(t, cadaAvance, avance{fase: "listo"}, avance{err: caido}, avance{err: caido}, avance{})
		if err != nil || !slices.Equal(fases, []string{"listo", ""}) {
			t.Errorf("fases %v, error %v", fases, err)
		}
	})
	t.Run("el servicio no vuelve a responder", func(t *testing.T) {
		respuestas := []avance{{fase: "instalando"}}
		for range 5 {
			respuestas = append(respuestas, avance{err: caido})
		}
		fases, err := seguir(t, plazoSinServicio/4, respuestas...)
		if err != nil || !slices.Equal(fases, []string{"instalando", ""}) {
			t.Errorf("fases %v, error %v", fases, err)
		}
	})
	t.Run("la actualización falla", func(t *testing.T) {
		fases, err := seguir(t, cadaAvance, avance{fase: "descargando"}, avance{fase: "error"})
		if err == nil || err.Error() != "firma no válida" || !slices.Equal(fases, []string{"descargando", ""}) {
			t.Errorf("fases %v, error %v", fases, err)
		}
	})
	t.Run("otra ventana la cancela", func(t *testing.T) {
		fases, err := seguir(t, cadaAvance, avance{fase: "listo"}, avance{})
		if err != nil || !slices.Equal(fases, []string{"listo", ""}) {
			t.Errorf("fases %v, error %v", fases, err)
		}
	})
	t.Run("la consulta a la que se unió falla y nunca llega una fase", func(t *testing.T) {
		respuestas := make([]avance, sinFaseMaximo)
		fases, err := seguir(t, cadaAvance, respuestas...)
		if err != nil || !slices.Equal(fases, []string{""}) {
			t.Errorf("fases %v, error %v", fases, err)
		}
	})
}

// Una doble pulsación no empieza otra búsqueda ni otra instalación, y una búsqueda no pisa la instalación que empezó
// mientras consultaba.
func TestBuscarEInstalarUnaVez(t *testing.T) {
	var a actualizacionMenu
	if !a.buscar() || a.buscar() {
		t.Fatalf("buscar dos veces: %+v", a)
	}
	if !a.instalar() || a.instalar() || a.buscar() {
		t.Fatalf("instalar dos veces o buscar instalando: %+v", a)
	}
	a.buscado(actualizacionMenu{nueva: "v0.57.0", instalable: true})
	if a != (actualizacionMenu{nueva: "v0.57.0", instalable: true, fase: "descargando"}) {
		t.Errorf("la búsqueda pisó la instalación: %+v", a)
	}
}
