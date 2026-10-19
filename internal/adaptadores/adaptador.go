// Package adaptadores define el contrato con los agentes y el adaptador simulado para pruebas.
package adaptadores

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"tramtai/internal/nucleo"
)

// Evento es un fragmento emitido durante la ejecución (texto, avisos, progreso).
type Evento struct {
	Tipo  string
	Datos string
}

// Resultado es la salida final de una ejecución, con el uso y el modelo efectivo cuando el agente los informa.
type Resultado struct {
	Salida         string
	Uso            nucleo.Uso
	ModeloEfectivo string
	ContextoTokens int // el contexto del modelo efectivo, si el CLI lo informa; 0 si no
}

// Contextual es un adaptador que puede consultar el contexto de un modelo sin ejecutarlo (la API de Gemini).
type Contextual interface {
	Contexto(ctx context.Context, modelo string) (int, error)
}

// Adaptador ejecuta una solicitud contra un agente. Debe respetar ctx: al cancelarse, termina y devuelve ctx.Err().
// Recibe el modelo ya resuelto en solicitud.Modelo (el planificador traduce los perfiles).
type Adaptador interface {
	Ejecutar(ctx context.Context, solicitud nucleo.SolicitudEjecucion, emitir func(Evento) error) (Resultado, error)
}

// Resolvedor lo implementan los adaptadores que saben qué modelo concreto usarán para un alias (sonnet, haiku…) o
// para su predeterminado ("") sin completar una petición.
type Resolvedor interface {
	ResolverModelo(ctx context.Context, modelo string) (string, error)
}

// resueltos guarda lo que contestó ResolverModelo unas horas: los alias cambian de versión con el CLI.
var resueltos sync.Map // agente/modelo → resolucion

type resolucion struct {
	modelo string
	cuando time.Time
}

// resolviendo comparte una resolución en curso entre peticiones: mientras la caché está vacía, varias consultas a
// /api/v1/modelos no lanzan cada una su CLI.
var resolviendo singleflight.Group

// plazoResolver acota cada resolución, aunque quien la pide no tenga plazo: un CLI colgado antes de contestar (red,
// autenticación) no deja su proceso vivo para siempre. Es variable para las pruebas.
var plazoResolver = 60 * time.Second

// ModeloConcreto devuelve el modelo que usará a para modelo, o "" si el adaptador no sabe decirlo.
func ModeloConcreto(ctx context.Context, a Adaptador, agente, modelo string) string {
	r, ok := a.(Resolvedor)
	if !ok {
		return ""
	}
	clave := agente + "/" + modelo
	if v, ok := resueltos.Load(clave); ok && time.Since(v.(resolucion).cuando) < 6*time.Hour {
		return v.(resolucion).modelo
	}
	v, _, _ := resolviendo.Do(clave, func() (any, error) {
		ctx, cancelar := context.WithTimeout(ctx, plazoResolver)
		defer cancelar()
		concreto, err := r.ResolverModelo(ctx, modelo)
		if err != nil || concreto == "" {
			return "", nil
		}
		resueltos.Store(clave, resolucion{concreto, time.Now()})
		return concreto, nil
	})
	return v.(string)
}

// Simulado es un agente determinista para pruebas: espera Demora y devuelve Respuesta,
// o el contenido del último mensaje si Respuesta está vacía.
type Simulado struct {
	Demora    time.Duration
	Respuesta string
	Error     error
}

// Ejecutar implementa Adaptador.
func (s Simulado) Ejecutar(ctx context.Context, solicitud nucleo.SolicitudEjecucion, emitir func(Evento) error) (Resultado, error) {
	select {
	case <-time.After(s.Demora):
	case <-ctx.Done():
		return Resultado{}, ctx.Err()
	}
	if s.Error != nil {
		return Resultado{}, s.Error
	}
	respuesta := s.Respuesta
	if respuesta == "" && len(solicitud.Mensajes) > 0 {
		respuesta = solicitud.Mensajes[len(solicitud.Mensajes)-1].Contenido
	}
	if err := emitir(Evento{Tipo: "texto", Datos: respuesta}); err != nil {
		return Resultado{}, err
	}
	return Resultado{Salida: respuesta}, nil
}
