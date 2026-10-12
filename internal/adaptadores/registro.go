package adaptadores

import "tramtai/internal/nucleo"

// Registro crea los adaptadores reales según la configuración efectiva y devuelve el límite de ejecuciones
// simultáneas de cada uno (concurrencia_servicio.adaptadores).
func Registro(c *nucleo.Configuracion) (map[string]Adaptador, map[string]int) {
	registro := map[string]Adaptador{
		"claude": Claude{Programa: c.Texto("comandos_agentes.claude"), Autenticacion: c.Texto("adaptadores.claude.autenticacion")},
		"codex":  Codex{Programa: c.Texto("comandos_agentes.codex"), Autenticacion: c.Texto("adaptadores.codex.autenticacion")},
	}
	limites := map[string]int{}
	for nombre := range registro {
		limites[nombre] = c.Entero("concurrencia_servicio.adaptadores." + nombre)
	}
	return registro, limites
}
