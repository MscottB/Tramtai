package servicio

import (
	"context"
	"fmt"
	"os"

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
func (s *Servicio) estadoActualizacion(ctx context.Context) map[string]any {
	v, nueva := s.versionNueva(ctx, true)
	_, instalada := actualizacion.Instalada()
	return map[string]any{"actual": version.Version, "buscar": s.buscar.Load(), "disponible": nueva,
		"version": v.Etiqueta, "url": v.Pagina, "notas": v.Notas, "instalable": nueva && instalada && v.TieneInstalador()}
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

// Actualizar descarga la última versión, comprueba su suma SHA-256 y lanza su instalador en silencio. El instalador
// detiene este servicio (los trabajos en curso terminan dentro del plazo de drenaje) y vuelve a abrir la app.
func (s *Servicio) Actualizar(ctx context.Context) (actualizacion.Version, error) {
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
	dir, err := os.MkdirTemp("", "tramtai-actualizacion")
	if err != nil {
		return v, err
	}
	instalador, err := actualizacion.Descargar(ctx, v, dir)
	if err != nil {
		os.RemoveAll(dir)
		return v, nucleo.NuevoError(nucleo.CodigoErrorInterno, idioma.Tf("no se pudo descargar la versión nueva: %s", err))
	}
	fmt.Fprintf(s.opc.Registros, "actualizando de %s a %s con %s\n", version.Texto(), v.Etiqueta, instalador)
	return v, actualizacion.Instalar(instalador, carpeta)
}
