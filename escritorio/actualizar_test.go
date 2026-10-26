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
	if a := resultadoBusqueda(contrato.EstadoActualizacion{Actual: "v0.56.1", Disponible: true, Version: "v0.57.0"}, nil, ahora); a != (actualizacionMenu{nueva: "v0.57.0"}) {
		t.Errorf("con versión nueva: %+v", a)
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

// seguir pasa las respuestas por seguirAvance, sin esperas, y devuelve las fases que recibió el menú y el resultado.
func seguir(t *testing.T, respuestas ...avance) ([]string, error) {
	t.Helper()
	var fases []string
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
	}, func() {}, func(f string) { fases = append(fases, f) })
	return fases, err
}

func TestSeguirAvance(t *testing.T) {
	caido := errors.New("connection refused")
	t.Run("hasta que el instalador detiene el servicio", func(t *testing.T) {
		fases, err := seguir(t, avance{fase: "descargando"}, avance{err: caido}, avance{fase: "descargando"}, avance{fase: "verificando"},
			avance{fase: "listo"}, avance{fase: "instalando"}, avance{err: caido})
		// La última fase se queda: el menú dice que se instala hasta que el instalador cierra la app.
		if err != nil || !slices.Equal(fases, []string{"descargando", "descargando", "verificando", "listo", "instalando"}) {
			t.Errorf("fases %v, error %v", fases, err)
		}
	})
	t.Run("la actualización falla", func(t *testing.T) {
		fases, err := seguir(t, avance{fase: "descargando"}, avance{fase: "error"})
		if err == nil || err.Error() != "firma no válida" || !slices.Equal(fases, []string{"descargando", ""}) {
			t.Errorf("fases %v, error %v", fases, err)
		}
	})
	t.Run("otra ventana la cancela", func(t *testing.T) {
		fases, err := seguir(t, avance{fase: "listo"}, avance{})
		if err != nil || !slices.Equal(fases, []string{"listo", ""}) {
			t.Errorf("fases %v, error %v", fases, err)
		}
	})
	t.Run("la consulta a la que se unió falla y nunca llega una fase", func(t *testing.T) {
		respuestas := make([]avance, sinFaseMaximo)
		fases, err := seguir(t, respuestas...)
		if err != nil || !slices.Equal(fases, []string{""}) {
			t.Errorf("fases %v, error %v", fases, err)
		}
	})
}
