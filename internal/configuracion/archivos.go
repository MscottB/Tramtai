package configuracion

import (
	"os"
	"path/filepath"
	"strings"
	"tramtai/internal/carpetas"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// RutaGlobal es el archivo de configuración global del usuario, en RaizUsuario; TRAMTAI_CONFIG_GLOBAL lo sustituye
// (y con él la carpeta profiles/ de al lado).
func RutaGlobal() (string, error) {
	if ruta := os.Getenv("TRAMTAI_CONFIG_GLOBAL"); ruta != "" {
		return ruta, nil
	}
	raiz, err := carpetas.RaizUsuario()
	if err != nil {
		return "", err
	}
	return filepath.Join(raiz, "config.yaml"), nil
}

// BuscarProyecto sube desde dir hasta encontrar .tramtai/config.yaml y devuelve la raíz del proyecto.
func BuscarProyecto(dir string) (string, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real // ruta real: sin enlaces ni nombres cortos de Windows
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

// CargarConfiguracion resuelve las capas de configuración (interna, global, proyecto, local y entorno). Con raíz vacía
// usa solo la global y el entorno (instancia global); con la raíz de un proyecto añade su configuración versionada y la
// local. agentes es la capa interna de los agentes (agentes.Internos()): sus comandos, su concurrencia y la
// autenticación de cada CLI.
func CargarConfiguracion(raiz string, agentes Capa) (*Configuracion, error) {
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
	c := Resolver(append(capas, entorno, agentes)...)
	var ignoradas []string
	for _, capa := range capas {
		ignoradas = append(ignoradas, capa.Ignoradas...)
	}
	if len(ignoradas) > 0 {
		c.ignoradas.Store(&ignoradas)
	}
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

// RutaPermitida resuelve un archivo local que la CLI o la terminal van a enviar y devuelve su ruta real si está
// dentro de instancia_global.raices_permitidas. Sin raíces configuradas no hay límite. Como se resuelven los
// enlaces, un enlace dentro de una raíz que apunte fuera de ella se rechaza.
func (c *Configuracion) RutaPermitida(ruta string) (string, error) {
	abs, err := filepath.Abs(carpetas.ExpandirCasa(ruta))
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	raices := c.Lista("instancia_global.raices_permitidas")
	if len(raices) == 0 {
		return real, nil
	}
	for _, v := range raices {
		texto, _ := v.(string)
		if texto == "" {
			continue
		}
		raiz, err := filepath.Abs(carpetas.ExpandirCasa(texto))
		if err != nil {
			continue
		}
		if r, err := filepath.EvalSymlinks(raiz); err == nil {
			raiz = r
		}
		if carpetas.Dentro(raiz, real) {
			return real, nil
		}
	}
	return "", nucleo.NuevoError(nucleo.CodigoRutaNoPermitida, idioma.Tf("%s está fuera de las carpetas desde las que se pueden enviar archivos: muévelo a una de ellas", ruta))
}

// ProyectoConfiable indica si la raíz figura en proyectos_confiables. Solo la configuración global (o el entorno)
// puede declararlo: la versionada y local.yaml lo rechazan, así que un repositorio clonado nunca se marca confiable a
// sí mismo.
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
