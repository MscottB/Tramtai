package servicio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"tramtai/internal/actualizacion"
	"tramtai/internal/configuracion"
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
	ruta, err := configuracion.RutaGlobal()
	if err != nil {
		return err
	}
	if err := configuracion.FijarValor(ruta, configuracion.OrigenGlobal, "servicio.buscar_actualizaciones", buscar); err != nil {
		return err
	}
	s.buscar.Store(buscar)
	return nil
}

// ProgresoActualizacion es lo que ve la Web UI mientras se actualiza: la fase (descargando, verificando, listo,
// instalando o error), los bytes descargados y el total; en listo, cuándo instalará.
type ProgresoActualizacion struct {
	Fase      string     `json:"fase"`
	Version   string     `json:"version"`
	Recibidos int64      `json:"recibidos"`
	Total     int64      `json:"total"`
	InstalaEn *time.Time `json:"instala_en,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// instalar, ultimaVersion y esperaInstalacion son variables para que las pruebas usen un instalador y una versión
// simulados y una espera corta.
var (
	instalar          = actualizacion.Instalar
	ultimaVersion     = actualizacion.Ultima
	esperaInstalacion = 10 * time.Second
)

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
// drenaje) y vuelve a abrir la app. Si ya hay una en curso, no empieza otra: se une a ella y devuelve su versión,
// para que otra pestaña (o la misma tras recargar) siga el mismo progreso.
func (s *Servicio) IniciarActualizacion(ctx context.Context) (actualizacion.Version, error) {
	// Comprobar y ocupar van juntos, antes de ir a la red: dos peticiones a la vez no lanzan dos descargas.
	s.muAvance.Lock()
	if p := s.avance; p.Fase != "" && p.Fase != "error" {
		s.muAvance.Unlock()
		return actualizacion.Version{Etiqueta: p.Version}, nil
	}
	s.avance = ProgresoActualizacion{Fase: "descargando"}
	s.muAvance.Unlock()
	v, carpeta, err := versionParaInstalar(ctx)
	if err != nil {
		s.fijarAvance(func(p *ProgresoActualizacion) { *p = ProgresoActualizacion{} })
		return v, err
	}
	s.fijarAvance(func(p *ProgresoActualizacion) { p.Version = v.Etiqueta })
	go s.actualizar(v, carpeta)
	return v, nil
}

// versionParaInstalar devuelve la última versión y la carpeta de la instalación si hay una nueva que se puede
// instalar desde aquí.
func versionParaInstalar(ctx context.Context) (actualizacion.Version, string, error) {
	v, err := ultimaVersion(ctx)
	if err != nil {
		return v, "", nucleo.NuevoError(nucleo.CodigoErrorInterno, idioma.Tf("no se pudo consultar la última versión: %s", err))
	}
	if !actualizacion.MasNueva(v.Etiqueta, version.Version) {
		return v, "", nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("ya tienes la última versión (%s)", version.Texto()))
	}
	carpeta, instalada := actualizacion.Instalada()
	if !instalada || !v.TieneInstalador() {
		return v, "", nucleo.NuevoError(nucleo.CodigoSolicitudInvalida,
			idioma.Tf("esta copia de Tramtai no se instaló con el instalador de Windows: descarga %s desde %s", v.Etiqueta, v.Pagina))
	}
	return v, carpeta, nil
}

func (s *Servicio) actualizar(v actualizacion.Version, carpeta string) {
	fallar := func(err error) {
		s.fijarAvance(func(p *ProgresoActualizacion) { p.Fase, p.Error = "error", err.Error() })
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelar()
	dir, err := os.MkdirTemp("", actualizacion.PrefijoCarpeta)
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
	s.esperarEInstalar(v, instalador, dir, carpeta)
}

// esperarEInstalar deja la descarga lista y avisa (fase listo) de que instalará al vencer esperaInstalacion, para que
// el cierre de la app no llegue de golpe; antes, «Reiniciar ahora» instala ya y «Cancelar» borra la descarga.
func (s *Servicio) esperarEInstalar(v actualizacion.Version, instalador, dir, carpeta string) {
	orden := make(chan bool, 1)
	cuando := time.Now().Add(esperaInstalacion)
	s.muAvance.Lock()
	s.avance.Fase, s.avance.InstalaEn, s.ordenInstalar = "listo", &cuando, orden
	s.muAvance.Unlock()
	reloj := time.NewTimer(esperaInstalacion)
	defer reloj.Stop()
	instalarYa := true
	select {
	case instalarYa = <-orden:
	case <-reloj.C:
	}
	s.muAvance.Lock()
	select { // una orden que entró mientras vencía el plazo manda: un Cancelar confirmado nunca instala
	case instalarYa = <-orden:
	default:
	}
	s.ordenInstalar = nil // desde aquí ya no se decide: lo que llegue tarde responde 409
	if instalarYa {
		s.avance.Fase, s.avance.InstalaEn = "instalando", nil
	} else {
		s.avance = ProgresoActualizacion{}
	}
	s.muAvance.Unlock()
	if !instalarYa {
		os.RemoveAll(dir)
		return
	}
	fmt.Fprintf(s.opc.Registros, "actualizando de %s a %s con %s\n", version.Texto(), v.Etiqueta, instalador)
	if err := instalar(instalador, carpeta); err != nil {
		s.fijarAvance(func(p *ProgresoActualizacion) { p.Fase, p.Error = "error", err.Error() })
	}
}

// decidirInstalacion atiende «Reiniciar ahora» (true) o «Cancelar» (false) mientras la descarga espera en la fase listo.
func (s *Servicio) decidirInstalacion(instalarYa bool) error {
	s.muAvance.Lock()
	defer s.muAvance.Unlock()
	if s.ordenInstalar == nil {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("no hay ninguna actualización esperando para instalarse"))
	}
	s.ordenInstalar <- instalarYa // búfer 1 y solo una orden: las siguientes responden 409
	s.ordenInstalar = nil
	return nil
}
