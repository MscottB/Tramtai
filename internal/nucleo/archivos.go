package nucleo

import (
	"os"
	"path/filepath"
	"strings"
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
	c := Resolver(append(capas, entorno)...)
	if raiz != "" {
		tramtai := filepath.Join(raiz, ".tramtai")
		c.perfiles = append(c.perfiles, carpetaPerfiles{dir: filepath.Join(tramtai, "profiles"), limite: tramtai})
	}
	c.perfiles = append(c.perfiles, carpetaPerfiles{dir: filepath.Join(filepath.Dir(global), "profiles"), limite: filepath.Dir(global)})
	return c, nil
}

// Lista devuelve un valor de tipo lista, o nil si no existe.
func (c *Configuracion) Lista(clave string) []any {
	v, _ := c.e().valores[clave].([]any)
	return v
}

// ProyectoConfiable indica si la raíz figura en proyectos_confiables. La configuración versionada no puede
// declararlo (clave administrada), así que un repositorio clonado nunca se marca confiable a sí mismo (sección 6.3).
func (c *Configuracion) ProyectoConfiable(raiz string) bool {
	if raiz == "" {
		return false
	}
	// os.SameFile identifica la carpeta aunque la ruta se escriba distinto (mayúsculas, 8.3 o barra final).
	destino, err := os.Stat(raiz)
	for _, v := range c.Lista("proyectos_confiables") {
		texto, ok := v.(string)
		if !ok || texto == "" {
			continue
		}
		if info, errInfo := os.Stat(texto); err == nil && errInfo == nil && os.SameFile(destino, info) {
			return true
		}
		if strings.EqualFold(filepath.Clean(texto), filepath.Clean(raiz)) {
			return true
		}
	}
	return false
}
