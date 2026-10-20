package api

import (
	"net/http"
	"time"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// EventoTrabajo es lo que el flujo de la instancia cuenta de cada trabajo: dónde va, nunca qué lleva (ni el mensaje,
// ni la respuesta, ni los archivos).
type EventoTrabajo struct {
	ID          string    `json:"id"`
	Estado      string    `json:"estado"`
	Etapa       string    `json:"etapa"`
	Agente      string    `json:"agente"`
	Modelo      string    `json:"modelo"`
	API         string    `json:"api"`
	Origen      string    `json:"origen"`
	Intento     int       `json:"intento"`
	CodigoError string    `json:"codigo_error"`
	Actualizado time.Time `json:"actualizado"`
}

// EtapaDe agrupa los estados en las 5 paradas de la ruta de un trabajo (las mismas que Ruta.vue).
func EtapaDe(e nucleo.Estado) string {
	switch e {
	case nucleo.EnCola:
		return "cola"
	case nucleo.PreparandoEntrada, nucleo.EsperandoAgente:
		return "preparacion"
	case nucleo.Ejecutando, nucleo.CancelacionSolicitada:
		return "ejecucion"
	case nucleo.ValidandoSalida:
		return "validacion"
	default:
		return "llegada"
	}
}

const (
	// recientesVivos son los terminados que entran en la foto inicial: una petición corta alcanza a verse llegando.
	recientesVivos = 10 * time.Second
	// esperaLecturas junta los avisos seguidos: como mucho una lectura de la base cada 200 ms por flujo.
	esperaLecturas = 200 * time.Millisecond
	// margenCambios vuelve a mirar un poco antes de la última lectura: dos cambios con la misma hora no se pierden (lo
	// repetido no se reenvía).
	margenCambios = 50 * time.Millisecond
)

// eventosInstancia es GET /api/v1/eventos: una foto de los trabajos vivos y después un evento trabajo por cada cambio
// de estado o de agente. Una API por aplicación solo ve los suyos.
func (s *Servidor) eventosInstancia(w http.ResponseWriter, r *http.Request) {
	liberar, ok := s.Planificador.ReservarFlujo()
	if !ok {
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoLimiteFlujos, idioma.T("hay demasiados flujos de eventos abiertos en esta instancia")))
		return
	}
	defer liberar()
	punto := ""
	if s.Punto != nil {
		punto = s.Punto.Nombre
	}
	ctx := r.Context()
	f := abrirFlujo(w)
	type visto struct {
		estado, agente string
		intento        int
	}
	vistos := map[string]visto{}
	enviar := func(t nucleo.Trabajo) error {
		previo, conocido := vistos[t.ID]
		v := visto{estado: string(t.Estado), agente: t.Solicitud.Agente, intento: max(previo.intento, 1)}
		if conocido && previo.agente != v.agente {
			v.intento = previo.intento + 1
		}
		if conocido && v == previo {
			return nil
		}
		vistos[t.ID] = v
		return f.json("trabajo", EventoTrabajo{ID: t.ID, Estado: v.estado, Etapa: EtapaDe(t.Estado), Agente: v.agente,
			Modelo: modeloDe(t), API: t.PuntoAcceso, Origen: t.Origen, Intento: v.intento, CodigoError: string(t.CodigoError),
			Actualizado: t.ActualizadoEn})
	}

	aviso := s.Planificador.Avisos() // antes de la foto: un cambio entre la lectura y la espera no se pierde
	ultimo := time.Now().UTC()
	foto, err := s.Planificador.Vivos(ctx, punto, ultimo.Add(-recientesVivos))
	if err != nil {
		return
	}
	for _, t := range foto {
		if err := enviar(t); err != nil {
			return
		}
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
		cambiados, err := s.Planificador.CambiadosDesde(ctx, punto, desde.Add(-margenCambios))
		if err != nil {
			return
		}
		for _, t := range cambiados {
			if err := enviar(t); err != nil {
				return
			}
		}
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
