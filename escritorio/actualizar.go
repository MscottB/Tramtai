package main

import (
	"errors"
	"time"

	"tramtai/internal/contrato"
)

// «Buscar actualizaciones» e «Instalar la versión…» desde el menú de la bandeja, sin abrir la ventana: el menú cuenta
// la búsqueda y el avance con lo mismo que usan la Web UI y la terminal (la administración del servicio).

// plazoAlDia es cuánto enseña el menú «Tienes la última versión» tras buscar; después vuelve «Buscar actualizaciones».
const plazoAlDia = 5 * time.Minute

// cadaAvance es cada cuánto se pregunta el avance de la actualización, como la Web UI y la terminal.
const cadaAvance = 400 * time.Millisecond

// sinFaseMaximo son las consultas sin fase tras las que se deja de esperar sin haber visto ninguna (30 s, como
// abandonarSinFase de la Web UI): la consulta a la que esta se unió falló.
const sinFaseMaximo = 75

// plazoSinServicio es cuánto se espera, desde «listo», a que el servicio vuelva a responder: el instalador lo detiene,
// instala y cierra la app en un par de minutos (con el drenaje de las peticiones en curso). Si pasado el plazo la app
// sigue abierta, la actualización no llegó a nada y el menú deja de decir «Actualizando…».
const plazoSinServicio = 10 * time.Minute

// actualizacionMenu es lo que la bandeja sabe de lo que se busca o instala desde su menú.
type actualizacionMenu struct {
	buscando   bool
	fallo      bool      // la última búsqueda no pudo consultar GitHub
	alDia      string    // la versión instalada, si la última búsqueda dijo que no hay otra
	alDiaHasta time.Time // hasta cuándo se enseña alDia
	nueva      string    // la que encontró la búsqueda: con la búsqueda automática apagada, el estado del servicio no la trae
	instalable bool      // si nueva se instala desde aquí (una copia del instalador de Windows)
	fase       string    // la de la actualización que se instala desde el menú; vacía si no hay ninguna
}

// resultadoBusqueda es lo que enseña el menú tras consultar GitHub en el momento. Si GitHub falla pero el servicio
// conserva una versión nueva de antes, se ofrece esa, como en la Web UI.
func resultadoBusqueda(e contrato.EstadoActualizacion, err error, ahora time.Time) actualizacionMenu {
	switch {
	case err == nil && e.Disponible:
		return actualizacionMenu{nueva: e.Version, instalable: e.Instalable}
	case err != nil || e.Fallo:
		return actualizacionMenu{fallo: true}
	}
	return actualizacionMenu{alDia: e.Actual, alDiaHasta: ahora.Add(plazoAlDia)}
}

// buscar marca que empieza una búsqueda; false si ya hay una búsqueda o una instalación en curso (una doble pulsación).
func (a *actualizacionMenu) buscar() bool {
	if a.buscando || a.fase != "" {
		return false
	}
	*a = actualizacionMenu{buscando: true}
	return true
}

// buscado guarda el resultado de la búsqueda sin pisar una instalación que empezó mientras se consultaba.
func (a *actualizacionMenu) buscado(r actualizacionMenu) {
	r.fase = a.fase
	*a = r
}

// instalar marca que empieza la instalación; false si ya hay una en curso.
func (a *actualizacionMenu) instalar() bool {
	if a.fase != "" {
		return false
	}
	a.fase = "descargando"
	return true
}

// vigente quita «al día» cuando pasa su plazo.
func (a actualizacionMenu) vigente(ahora time.Time) actualizacionMenu {
	if !ahora.Before(a.alDiaHasta) {
		a.alDia = ""
	}
	return a
}

// seguirAvance sigue la actualización en curso como el avance de la Web UI y la terminal: tras cada espera pregunta el
// progreso y le pasa cada fase a alFase. Desde «listo», si el servicio no responde (el instalador lo detuvo, o lo
// reiniciaron) queda la última fase y se sigue preguntando hasta plazoSinServicio: lo normal es que el instalador cierre
// la app antes. Termina con alFase("") cuando falla (y devuelve el error) o cuando ya no hay nada que esperar.
func seguirAvance(progreso func() (contrato.ProgresoActualizacion, error), esperar func(), alFase func(string), ahora func() time.Time) error {
	fase, vistaFase, sinFase, respondio := "", false, 0, ahora()
	for {
		esperar()
		p, err := progreso()
		switch {
		case err != nil && (fase == "listo" || fase == "instalando"):
			if ahora().Sub(respondio) < plazoSinServicio {
				continue
			}
			sinFase = sinFaseMaximo
		case err != nil:
			sinFase++ // antes de «listo», queda la última fase y se sigue preguntando un rato
		case p.Fase == "error":
			alFase("")
			return errors.New(p.Error)
		case p.Fase != "":
			fase, vistaFase, sinFase, respondio = p.Fase, true, 0, ahora()
			alFase(fase)
			continue
		case vistaFase: // la cancelaron (aquí o en otra ventana) o el servicio volvió sin ella
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
