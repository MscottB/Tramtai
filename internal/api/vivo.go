package api

import (
	"context"
	"net/http"
	"time"

	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
)

const (
	// recientesVivos son los terminados que entran en la foto inicial: una petición corta alcanza a verse llegando.
	recientesVivos = 10 * time.Second
	// esperaLecturas junta los avisos seguidos: como mucho una lectura de la base cada 200 ms por flujo.
	esperaLecturas = 200 * time.Millisecond
	// margenCambios vuelve a mirar un poco antes de la última lectura: dos cambios con la misma hora no se pierden (lo
	// repetido no se reenvía).
	margenCambios = 50 * time.Millisecond
)

// visto es lo último que un flujo envió de un trabajo; intento cuenta los agentes que probó desde que se abrió el flujo.
type visto struct {
	estado, agente string
	intento        int
	terminado      bool
	cambio         time.Time
}

// olvidar quita los terminados que cambiaron antes de antes: ya no vuelven a leerse, así un flujo abierto días no
// guarda todos los trabajos que vio.
func olvidar(vistos map[string]visto, antes time.Time) {
	for id, v := range vistos {
		if v.terminado && v.cambio.Before(antes) {
			delete(vistos, id)
		}
	}
}

// leerCambios lee los trabajos cambiados después de desde; es variable para que las pruebas cuenten las lecturas.
var leerCambios = func(s *Servidor, ctx context.Context, punto string, desde time.Time) ([]nucleo.Trabajo, error) {
	return s.Planificador.CambiadosDesde(ctx, punto, desde)
}

// porPaginas envía lo que lee desde desde y, mientras las páginas lleguen llenas, sigue desde el último.
// shortcut: si otro trabajo comparte al nanosegundo la hora del último de una página llena se salta hasta su próximo
// cambio; si alguna vez pasa, paginar también por rowid.
func porPaginas(desde time.Time, leer func(time.Time) ([]nucleo.Trabajo, error), enviar func(nucleo.Trabajo) error) error {
	for {
		pagina, err := leer(desde)
		if err != nil {
			return err
		}
		for _, t := range pagina {
			if err := enviar(t); err != nil {
				return err
			}
		}
		if len(pagina) < persistencia.LimiteCambios {
			return nil
		}
		desde = pagina[len(pagina)-1].ActualizadoEn
	}
}

// eventosInstancia es GET /api/v1/eventos: una foto de los trabajos vivos, un evento foto que la cierra (el cliente
// borra lo que no vino) y después un evento trabajo por cada cambio de estado o de agente. Una API por aplicación solo
// ve los suyos.
func (s *Servidor) eventosInstancia(w http.ResponseWriter, r *http.Request) {
	liberar, ok := s.Planificador.ReservarFlujo()
	if !ok {
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoLimiteFlujos, idioma.T("hay demasiados flujos de eventos abiertos en este espacio")))
		return
	}
	defer liberar()
	punto := ""
	if s.Punto != nil {
		punto = s.Punto.Nombre
	}
	ctx := r.Context()
	f := abrirFlujo(w)
	vistos := map[string]visto{}
	enviar := func(t nucleo.Trabajo) error {
		previo, conocido := vistos[t.ID]
		v := visto{estado: string(t.Estado), agente: t.Solicitud.Agente, intento: max(previo.intento, 1), terminado: t.Estado.Terminal(), cambio: t.ActualizadoEn}
		if conocido && previo.agente != v.agente {
			v.intento = previo.intento + 1
		}
		vistos[t.ID] = v
		if conocido && v.estado == previo.estado && v.agente == previo.agente {
			return nil
		}
		return f.json("trabajo", contrato.EventoTrabajo{ID: t.ID, Estado: v.estado, Etapa: t.Estado.Etapa(), Agente: v.agente,
			Modelo: modeloDe(t), API: t.PuntoAcceso, Origen: t.Origen, Intento: v.intento, CodigoError: string(t.CodigoError),
			Creado: t.CreadoEn, Actualizado: t.ActualizadoEn})
	}

	aviso := s.Planificador.Avisos() // antes de la foto: un cambio entre la lectura y la espera no se pierde
	ultimo := time.Now().UTC()
	terminadosDesde := ultimo.Add(-recientesVivos)
	err := porPaginas(time.Time{}, func(despues time.Time) ([]nucleo.Trabajo, error) {
		return s.Planificador.Vivos(ctx, punto, terminadosDesde, despues)
	}, enviar)
	if err != nil || f.json("foto", struct{}{}) != nil {
		return
	}
	ultimoLatido := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-aviso:
		case <-time.After(s.sondeo()):
		}
		if !esperar(r, esperaLecturas) {
			return
		}
		aviso = s.Planificador.Avisos()
		desde := ultimo
		ultimo = time.Now().UTC()
		err := porPaginas(desde.Add(-margenCambios), func(d time.Time) ([]nucleo.Trabajo, error) {
			return leerCambios(s, ctx, punto, d)
		}, enviar)
		if err != nil {
			return
		}
		olvidar(vistos, desde.Add(-margenCambios))
		if time.Since(ultimoLatido) > 15*time.Second {
			if err := f.latido(); err != nil {
				return
			}
			ultimoLatido = time.Now()
		}
	}
}

// esperar duerme d salvo que el cliente se desconecte (falso).
func esperar(r *http.Request, d time.Duration) bool {
	select {
	case <-r.Context().Done():
		return false
	case <-time.After(d):
		return true
	}
}

// modeloDe es el modelo que informó el agente o, si no lo dijo, el que se le pidió.
func modeloDe(t nucleo.Trabajo) string {
	if t.ModeloEfectivo != "" {
		return t.ModeloEfectivo
	}
	if t.ModeloEnviado != "" {
		return t.ModeloEnviado
	}
	return t.Solicitud.Modelo
}
