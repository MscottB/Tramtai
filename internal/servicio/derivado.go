package servicio

import (
	"context"

	"tramtai/internal/autoarranque"
	"tramtai/internal/contrato"
)

// Lo derivado lo calcula el servicio (docs/equipo/decisiones/001-estado-del-agente.md): la Web UI y la terminal pintan
// lo que llega y no repiten la regla.

// umbralApiPropia son las peticiones de apps con la clave del espacio en 24 h a partir de las que conviene darles su
// propia API (para limitarlas, medirlas y revocarlas por separado). No obliga a nada.
const umbralApiPropia = 20

func sugerirApiPropia(peticiones int) bool { return peticiones > umbralApiPropia }

// estadoAutoarranque dice si este sistema puede arrancar Tramtai al iniciar sesión y si lo hace (Opciones.Autoarranque
// lo cambia).
func (s *Servicio) estadoAutoarranque() (disponible, activo bool) {
	if s.opc.Autoarranque != nil {
		return s.opc.Autoarranque()
	}
	if !autoarranque.Disponible {
		return false, false
	}
	activo, _ = autoarranque.Activo()
	return true, activo
}

// primerosPasos son los de Inicio: un agente listo, una petición completada, una API y, para el usuario de este
// equipo en un sistema que lo admite, arrancar con el sistema. Comprueba los agentes (ejecuta sus CLI) y los devuelve.
func (s *Servicio) primerosPasos(ctx context.Context, inst *Instancia, local bool) contrato.PrimerosPasos {
	agentes := inst.api.Adaptadores(ctx)
	listos := 0
	for _, a := range agentes {
		if a.Estado == contrato.AgenteListo {
			listos++
		}
	}
	pasos := []contrato.Paso{
		{Pagina: "agentes", Hecho: listos > 0, Listos: listos, Total: len(agentes)},
		{Pagina: "ejecutar", Hecho: inst.estado(ctx).Trabajos["completado"] > 0},
		{Pagina: "conectar", Hecho: len(s.puntosDe(inst.ID)) > 0},
	}
	if disponible, activo := s.estadoAutoarranque(); disponible && local {
		pasos = append(pasos, contrato.Paso{Pagina: "configuracion", Hecho: activo})
	}
	return contrato.PrimerosPasos{Pasos: pasos, Adaptadores: agentes}
}
