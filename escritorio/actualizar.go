package main

import (
	"errors"
	"time"

	"tramtai/internal/contrato"
)

// «Buscar actualizaciones» e «Instalar la versión…» desde el menú de la bandeja, sin abrir la ventana: el menú cuenta
// la búsqueda y el avance con lo mismo que usan la Web UI y la terminal (la administración del servicio).

// plazoAlDia es cuánto enseña el menú «Tramtai está al día» tras buscar; después vuelve «Buscar actualizaciones».
const plazoAlDia = 5 * time.Minute

// cadaAvance es cada cuánto se pregunta el avance de la actualización, como la Web UI y la terminal.
const cadaAvance = 400 * time.Millisecond

// sinFaseMaximo son las consultas sin fase tras las que se deja de esperar sin haber visto ninguna (30 s, como
// abandonarSinFase de la Web UI): la consulta a la que esta se unió falló.
const sinFaseMaximo = 75

// actualizacionMenu es lo que la bandeja sabe de lo que se busca o instala desde su menú.
type actualizacionMenu struct {
	buscando   bool
	fallo      bool      // la última búsqueda no pudo consultar GitHub
	alDia      string    // la versión instalada, si la última búsqueda dijo que no hay otra
	alDiaHasta time.Time // hasta cuándo se enseña alDia
	nueva      string    // la que encontró la búsqueda: con la búsqueda automática apagada, el estado del servicio no la trae
	fase       string    // la de la actualización que se instala desde el menú; vacía si no hay ninguna
}

// resultadoBusqueda es lo que enseña el menú tras consultar GitHub en el momento.
func resultadoBusqueda(e contrato.EstadoActualizacion, err error, ahora time.Time) actualizacionMenu {
	switch {
	case err != nil || e.Fallo:
		return actualizacionMenu{fallo: true}
	case e.Disponible:
		return actualizacionMenu{nueva: e.Version}
	}
	return actualizacionMenu{alDia: e.Actual, alDiaHasta: ahora.Add(plazoAlDia)}
}

// vigente quita «al día» cuando pasa su plazo.
func (a actualizacionMenu) vigente(ahora time.Time) actualizacionMenu {
	if !ahora.Before(a.alDiaHasta) {
		a.alDia = ""
	}
	return a
}

// seguirAvance sigue la actualización en curso como el avance de la Web UI y la terminal: tras cada espera pregunta el
// progreso y le pasa cada fase a alFase. Termina cuando el instalador detiene el servicio (queda la última fase hasta que
// cierra la app), cuando falla (devuelve el error) o cuando ya no hay nada que esperar; en esos dos, con alFase("").
func seguirAvance(progreso func() (contrato.ProgresoActualizacion, error), esperar func(), alFase func(string)) error {
	fase, vistaFase, sinFase := "", false, 0
	for {
		esperar()
		p, err := progreso()
		switch {
		case err != nil:
			if fase == "listo" || fase == "instalando" { // el instalador ya detuvo el servicio
				return nil
			}
			sinFase++ // antes de «listo», queda la última fase y se sigue preguntando un rato
		case p.Fase == "error":
			alFase("")
			return errors.New(p.Error)
		case p.Fase != "":
			fase, vistaFase, sinFase = p.Fase, true, 0
			alFase(fase)
			continue
		case vistaFase: // otra ventana la canceló
			alFase("")
			return nil
		default:
			sinFase++
		}
		if sinFase >= sinFaseMaximo {
			alFase("")
			return nil
		}
	}
}
