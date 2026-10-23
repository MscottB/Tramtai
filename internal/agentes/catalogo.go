// Package agentes es el catálogo de agentes: cada uno vive en su módulo (internal/agentes/<nombre>) con su ficha, y
// el resto del código pregunta aquí en lugar de nombrar agentes. Añadir uno es crear su módulo y añadir su ficha a
// fichas (docs/equipo/nuevo-agente.md).
package agentes

import (
	"slices"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes/antigravity"
	"tramtai/internal/agentes/claude"
	"tramtai/internal/agentes/codex"
	"tramtai/internal/agentes/copilot"
	"tramtai/internal/agentes/gemini"
	"tramtai/internal/configuracion"
)

// fichas son los agentes que conoce Tramtai, en el orden de su ficha.
var fichas = ordenar(claude.Ficha, codex.Ficha, copilot.Ficha, gemini.Ficha, antigravity.Ficha)

func ordenar(lista ...adaptadores.Ficha) []adaptadores.Ficha {
	slices.SortStableFunc(lista, func(a, b adaptadores.Ficha) int { return a.Orden - b.Orden })
	return lista
}

// Internos es la capa de los valores predeterminados de cada agente (su comando, su concurrencia y la autenticación de
// los CLI), sacados de su ficha: configuracion no puede importar este paquete, así que quien carga la configuración
// la pasa a configuracion.CargarConfiguracion.
func Internos() configuracion.Capa {
	return configuracion.Capa{Origen: configuracion.OrigenInterno, Fuente: "interno", Valores: internos()}
}

func internos() map[string]any {
	valores := map[string]any{}
	for _, f := range fichas {
		valores["concurrencia_servicio.adaptadores."+f.Nombre] = f.Concurrencia
		if f.Comando != "" {
			valores["comandos_agentes."+f.Nombre] = f.Comando
			valores["adaptadores."+f.Nombre+".autenticacion"] = adaptadores.AutenticacionClaveAPI
		}
	}
	return valores
}

// Todos devuelve las fichas en su orden.
func Todos() []adaptadores.Ficha { return slices.Clone(fichas) }

// Buscar devuelve la ficha de un agente.
func Buscar(nombre string) (adaptadores.Ficha, bool) {
	i := slices.IndexFunc(fichas, func(f adaptadores.Ficha) bool { return f.Nombre == nombre })
	if i < 0 {
		return adaptadores.Ficha{}, false
	}
	return fichas[i], true
}

// Registro crea los adaptadores según la configuración efectiva y devuelve el límite de ejecuciones simultáneas de
// cada uno (concurrencia_servicio.adaptadores).
func Registro(c *configuracion.Configuracion) (map[string]adaptadores.Adaptador, map[string]int) {
	registro, limites := map[string]adaptadores.Adaptador{}, map[string]int{}
	for _, f := range fichas {
		registro[f.Nombre] = f.Crear(c)
		limites[f.Nombre] = c.Entero("concurrencia_servicio.adaptadores." + f.Nombre)
	}
	return registro, limites
}

// NombreVisible es el nombre con que se muestra un agente; el id si no está en el catálogo.
func NombreVisible(nombre string) string {
	if f, ok := Buscar(nombre); ok {
		return f.NombreVisible
	}
	return nombre
}

// Niveles devuelve los niveles de esfuerzo que admite un agente (nil si no admite ninguno o no se conoce).
func Niveles(nombre string) []string {
	f, _ := Buscar(nombre)
	if len(f.ValoresEsfuerzo) == 0 {
		return nil
	}
	return adaptadores.NivelesDe(f.ValoresEsfuerzo)
}

// NivelesDeModelo son los niveles que admite el agente con ese modelo: ninguno si su ficha dice que con él no se elige.
func NivelesDeModelo(nombre, modelo string) []string {
	if f, _ := Buscar(nombre); slices.Contains(f.ModelosSinEsfuerzo, modelo) {
		return nil
	}
	return Niveles(nombre)
}

// EsfuerzoPredeterminado es el nivel que usa el CLI con un modelo concreto si no se le indica uno; "" si no se conoce.
func EsfuerzoPredeterminado(nombre, modelo string) string {
	if f, ok := Buscar(nombre); ok && f.EsfuerzoPredeterminado != nil {
		return f.EsfuerzoPredeterminado(modelo)
	}
	return ""
}

// AdmiteSuscripcion indica si el agente se puede usar con la suscripción del usuario.
func AdmiteSuscripcion(nombre string) bool {
	f, _ := Buscar(nombre)
	return slices.Contains(f.Autenticaciones, adaptadores.AutenticacionSuscripcion)
}

// PermitidoEnApps indica si el agente se puede usar desde apps y APIs: su canario pasó o
// adaptadores.<nombre>.permitir_apps lo habilita. Un agente fuera del catálogo (los simulados de las pruebas) sí.
func PermitidoEnApps(c *configuracion.Configuracion, nombre string) bool {
	f, ok := Buscar(nombre)
	return !ok || f.PermitidoEnApps || (c != nil && c.Booleano("adaptadores."+nombre+".permitir_apps"))
}
