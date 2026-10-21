package inicio

import (
	"cmp"
	"slices"
	"time"

	"tramtai/internal/nucleo"
)

// Este archivo es web/src/vivo.ts en Go: las cuentas que convierten los eventos de GET /api/v1/eventos en las peticiones
// que se ven en la red de Inicio. Son funciones puras; el reloj es el `ahora` que reciben (nunca time.Now). La terminal
// dibuja filas y no un mapa, así que no lleva el tramo ni el avance de cada tranvía.

// maximoVisibles son las filas de la red en vivo; las demás se cuentan en un «+N».
const maximoVisibles = 12

// evento es el JSON del evento «trabajo» (api.EventoTrabajo): dónde va una petición, nunca qué lleva.
type evento struct {
	ID          string    `json:"id"`
	Estado      string    `json:"estado"`
	Etapa       string    `json:"etapa"`
	Agente      string    `json:"agente"`
	Modelo      string    `json:"modelo"`
	API         string    `json:"api"`
	Origen      string    `json:"origen"`
	Intento     int       `json:"intento"`
	CodigoError string    `json:"codigo_error"`
	Creado      time.Time `json:"creado"`
	Actualizado time.Time `json:"actualizado"`
}

// tranvia es una petición en la red. Desde es cuándo se aceptó (o, si no se sabe, cuándo apareció); Terminado, cuándo
// llegó o falló, para borrarla después (cero mientras sigue).
type tranvia struct {
	ID, Agente, Modelo, API, Origen string
	Etapa, Estado                   string
	Intento                         int
	CodigoError                     string
	Desde, Terminado                time.Time
}

type tranvias map[string]tranvia

// visibleTras es cuánto sigue en la red una petición terminada, por estado.
func visibleTras(estado string) time.Duration {
	switch estado {
	case "fallido", "interrumpido":
		return 4 * time.Second
	}
	return 2 * time.Second
}

// aplicar pone un evento en el mapa. Uno que ya llegó y se borró no vuelve por un evento tardío.
func aplicar(t tranvias, e evento, ahora time.Time, borrados map[string]bool) {
	if borrados[e.ID] {
		return
	}
	previo, habia := t[e.ID]
	tr := tranvia{ID: e.ID, Agente: e.Agente, Modelo: e.Modelo, API: e.API, Origen: e.Origen, Etapa: e.Etapa, Estado: e.Estado, Intento: e.Intento,
		CodigoError: e.CodigoError, Desde: previo.Desde}
	if !habia {
		tr.Desde = cmp.Or(e.Creado, ahora)
	}
	if e.Etapa == "llegada" {
		tr.Terminado = cmp.Or(previo.Terminado, ahora)
	}
	t[e.ID] = tr
}

// limpiar quita las peticiones terminadas que ya llevan su tiempo a la vista: 2 s las que llegan, 4 s las que fallan.
func limpiar(t tranvias, ahora time.Time, borrados map[string]bool) {
	for id, tr := range t {
		if !tr.Terminado.IsZero() && ahora.Sub(tr.Terminado) >= visibleTras(tr.Estado) {
			delete(t, id)
			borrados[id] = true
		}
	}
}

// podar quita las que no llegaron y no están entre presentes: lo que la foto de un flujo nuevo no trajo ya terminó hace
// rato (un corte largo no deja filas congeladas).
func podar(t tranvias, presentes map[string]bool) {
	for id, tr := range t {
		if tr.Terminado.IsZero() && !presentes[id] {
			delete(t, id)
		}
	}
}

// reconciliar pone el mapa al día con una consulta de los trabajos recientes cuando no hay flujo: los del mapa que
// terminaron llegan, los que ya no salen se van y los terminados que no estaban no aparecen.
func reconciliar(t tranvias, eventos []evento, ahora time.Time, borrados map[string]bool) {
	presentes := map[string]bool{}
	for _, e := range eventos {
		presentes[e.ID] = true
		if _, esta := t[e.ID]; e.Etapa != "llegada" || esta {
			aplicar(t, e, ahora, borrados)
		}
	}
	podar(t, presentes)
}

// eventoDe convierte un trabajo de la consulta en el evento que habría mandado el flujo.
func eventoDe(j nucleo.Trabajo) evento {
	return evento{ID: j.ID, Estado: string(j.Estado), Etapa: etapaDe(string(j.Estado)), Agente: j.Solicitud.Agente,
		Modelo: cmp.Or(j.ModeloEfectivo, j.ModeloEnviado), API: j.PuntoAcceso, Origen: j.Origen, Intento: 1, CodigoError: string(j.CodigoError), Creado: j.CreadoEn}
}

// claveEtapa es la clave de la Web UI que dice dónde va una petición; la llegada se nombra según cómo terminó.
func claveEtapa(tr tranvia) string {
	if tr.Etapa != "llegada" || tr.Estado == "completado" {
		return map[string]string{"cola": "vivo.etapaCola", "preparacion": "vivo.etapaPreparacion", "ejecucion": "vivo.etapaEjecucion",
			"validacion": "vivo.etapaValidacion", "llegada": "vivo.etapaLlegada"}[tr.Etapa]
	}
	if tr.Estado == "cancelado" {
		return "vivo.etapaCancelada"
	}
	return "vivo.etapaFallida"
}

// visibles son las que caben (las más recientes) y cuántas quedan fuera.
func visibles(t tranvias, maximo int) (lista []tranvia, resto int) {
	todos := make([]tranvia, 0, len(t))
	for _, tr := range t {
		todos = append(todos, tr)
	}
	slices.SortFunc(todos, func(a, b tranvia) int { return cmp.Or(b.Desde.Compare(a.Desde), cmp.Compare(a.ID, b.ID)) })
	return todos[:min(maximo, len(todos))], max(0, len(todos)-maximo)
}

// etapaDe agrupa un estado en una de las 5 paradas, igual que EtapaDe del servicio (internal/api/vivo.go).
func etapaDe(estado string) string {
	switch estado {
	case "en_cola":
		return "cola"
	case "preparando_entrada", "esperando_agente":
		return "preparacion"
	case "ejecutando", "cancelacion_solicitada":
		return "ejecucion"
	case "validando_salida":
		return "validacion"
	}
	return "llegada"
}

// esperaReconexion es la espera antes del intento n de reconectar el flujo: 1, 2, 4, 8, 16 s y después 30 s.
func esperaReconexion(intento int) time.Duration {
	return min(time.Second<<min(max(intento, 0), 5), 30*time.Second)
}
