// Package adaptadores define el contrato con los agentes y el adaptador simulado para pruebas.
package adaptadores

import (
	"context"
	"time"

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
}

// Adaptador ejecuta una solicitud contra un agente. Debe respetar ctx: al cancelarse, termina y devuelve ctx.Err().
// Recibe el modelo ya resuelto en solicitud.Modelo (el planificador traduce los perfiles).
type Adaptador interface {
	Ejecutar(ctx context.Context, solicitud nucleo.SolicitudEjecucion, emitir func(Evento) error) (Resultado, error)
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
