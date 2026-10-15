package servicio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"tramtai/internal/actualizacion"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/version"
)

// versionNueva devuelve la versión publicada si es posterior a la actual y la búsqueda está activada
// (servicio.buscar_actualizaciones).
func (s *Servicio) versionNueva(ctx context.Context, esperar bool) (actualizacion.Version, bool) {
	if !s.buscar.Load() {
		return actualizacion.Version{}, false
	}
	return s.actualizaciones.Disponible(ctx, esperar)
}

// estadoActualizacion es lo que muestra la Web UI: la versión actual, si hay otra nueva y si se instala desde aquí.
// Con ahora consulta GitHub en el momento («Buscar ahora»), aunque la búsqueda automática esté desactivada.
func (s *Servicio) estadoActualizacion(ctx context.Context, ahora bool) map[string]any {
	var v actualizacion.Version
	var nueva bool
	if ahora {
		s.actualizaciones.Renovar()
		v, nueva = s.actualizaciones.Disponible(ctx, true)
	} else {
		v, nueva = s.versionNueva(ctx, true)
	}
	_, instalada := actualizacion.Instalada()
	estado := map[string]any{"actual": version.Version, "buscar": s.buscar.Load(), "disponible": nueva,
		"version": v.Etiqueta, "url": v.Pagina, "notas": v.Notas, "instalable": nueva && instalada && v.TieneInstalador()}
	if hecha, fallo := s.actualizaciones.Comprobado(); !hecha.IsZero() {
		estado["comprobado"], estado["fallo"] = hecha.UTC().Format(time.RFC3339), fallo
	}
	return estado
}

// fijarBusqueda guarda servicio.buscar_actualizaciones en la configuración global y lo aplica sin reiniciar.
func (s *Servicio) fijarBusqueda(buscar bool) error {
	ruta, err := nucleo.RutaGlobal()
	if err != nil {
		return err
	}
	if err := FijarValor(ruta, nucleo.OrigenGlobal, "servicio.buscar_actualizaciones", buscar); err != nil {
		return err
	}
	s.buscar.Store(buscar)
	return nil
}

// ProgresoActualizacion es lo que ve la Web UI mientras se actualiza: la fase (descargando, verificando, instalando o
// error), los bytes descargados y el total.
type ProgresoActualizacion struct {
	Fase      string `json:"fase"`
	Version   string `json:"version"`
	Recibidos int64  `json:"recibidos"`
	Total     int64  `json:"total"`
	Error     string `json:"error,omitempty"`
}

// progresoActualizacion devuelve el avance de la actualización en curso (fase vacía si no hay ninguna).
func (s *Servicio) progresoActualizacion() ProgresoActualizacion {
	s.muAvance.Lock()
	defer s.muAvance.Unlock()
	return s.avance
}

func (s *Servicio) fijarAvance(cambiar func(*ProgresoActualizacion)) {
	s.muAvance.Lock()
	defer s.muAvance.Unlock()
	cambiar(&s.avance)
}

// IniciarActualizacion comprueba que hay una versión nueva que se puede instalar desde aquí y, en segundo plano, la
// descarga, comprueba su suma SHA-256 y lanza su instalador en silencio; el avance se consulta con
// progresoActualizacion. El instalador detiene este servicio (los trabajos en curso terminan dentro del plazo de
// drenaje) y vuelve a abrir la app. Si ya hay una en curso, no empieza otra.
func (s *Servicio) IniciarActualizacion(ctx context.Context) (actualizacion.Version, error) {
	if p := s.progresoActualizacion(); p.Fase != "" && p.Fase != "error" {
		return actualizacion.Version{Etiqueta: p.Version}, nil
	}
	v, err := actualizacion.Ultima(ctx)
	if err != nil {
		return v, nucleo.NuevoError(nucleo.CodigoErrorInterno, idioma.Tf("no se pudo consultar la última versión: %s", err))
	}
	if !actualizacion.MasNueva(v.Etiqueta, version.Version) {
		return v, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("ya tienes la última versión (%s)", version.Texto()))
	}
	carpeta, instalada := actualizacion.Instalada()
	if !instalada || !v.TieneInstalador() {
		return v, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida,
			idioma.Tf("esta copia de Tramtai no se instaló con el instalador de Windows: descarga %s desde %s", v.Etiqueta, v.Pagina))
	}
	s.fijarAvance(func(p *ProgresoActualizacion) { *p = ProgresoActualizacion{Fase: "descargando", Version: v.Etiqueta} })
	go s.actualizar(v, carpeta)
	return v, nil
}

func (s *Servicio) actualizar(v actualizacion.Version, carpeta string) {
	fallar := func(err error) {
		s.fijarAvance(func(p *ProgresoActualizacion) { p.Fase, p.Error = "error", err.Error() })
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelar()
	dir, err := os.MkdirTemp("", "tramtai-actualizacion")
	if err != nil {
		fallar(err)
		return
	}
	instalador, err := actualizacion.Descargar(ctx, v, dir, func(recibidos, total int64) {
		s.fijarAvance(func(p *ProgresoActualizacion) {
			p.Recibidos, p.Total = recibidos, total
			if total > 0 && recibidos >= total {
				p.Fase = "verificando"
			}
		})
	})
	if err != nil {
		os.RemoveAll(dir)
		fallar(errors.New(idioma.Tf("no se pudo descargar la versión nueva: %s", err)))
		return
	}
	s.fijarAvance(func(p *ProgresoActualizacion) { p.Fase = "instalando" })
	fmt.Fprintf(s.opc.Registros, "actualizando de %s a %s con %s\n", version.Texto(), v.Etiqueta, instalador)
	if err := actualizacion.Instalar(instalador, carpeta); err != nil {
		fallar(err)
	}
}
