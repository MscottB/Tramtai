package nucleo

import (
	"os"
	"path/filepath"
)

// RutaGlobal es el archivo de configuración global del usuario; TRAMTAI_CONFIG_GLOBAL lo sustituye
// (pruebas e instalaciones portátiles).
func RutaGlobal() (string, error) {
	if ruta := os.Getenv("TRAMTAI_CONFIG_GLOBAL"); ruta != "" {
		return ruta, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tramtai", "config.yaml"), nil
}

// BuscarProyecto sube desde dir hasta encontrar .tramtai/config.yaml y devuelve la raíz del proyecto.
func BuscarProyecto(dir string) (string, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, ".tramtai", "config.yaml")); err == nil && !info.IsDir() {
			return dir, true
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			return "", false
		}
		dir = padre
	}
}

// CargarConfiguracion resuelve las capas de la sección 12.2. Con raíz vacía usa solo la global y el entorno
// (instancia global); con la raíz de un proyecto añade su configuración versionada y la local.
func CargarConfiguracion(raiz string) (*Configuracion, error) {
	global, err := RutaGlobal()
	if err != nil {
		return nil, err
	}
	rutas := []string{global}
	origenes := []Origen{OrigenGlobal}
	if raiz != "" {
		rutas = append(rutas, filepath.Join(raiz, ".tramtai", "config.yaml"), filepath.Join(raiz, ".tramtai", "local.yaml"))
		origenes = append(origenes, OrigenProyecto, OrigenLocal)
	}
	var capas []Capa
	for i, ruta := range rutas {
		capa, err := CargarCapa(ruta, origenes[i])
		if err != nil {
			return nil, err
		}
		capas = append(capas, capa)
	}
	entorno, err := CapaEntorno(os.Environ())
	if err != nil {
		return nil, err
	}
	return Resolver(append(capas, entorno)...), nil
}

// Lista devuelve un valor de tipo lista, o nil si no existe.
func (c *Configuracion) Lista(clave string) []any {
	v, _ := c.valores[clave].([]any)
	return v
}
