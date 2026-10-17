package nucleo

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"tramtai/internal/idioma"
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

// CargarConfiguracion resuelve las capas de configuración (interna, global, proyecto, local y entorno). Con raíz vacía usa solo la global y el entorno
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

// VariableSecreta indica si el nombre de una variable de entorno parece guardar un secreto (claves, tokens,
// contraseñas, credenciales): los programas externos que lanza Tramtai no la heredan.
func VariableSecreta(nombre string) bool {
	nombre = strings.ToUpper(nombre)
	for _, marca := range []string{"API_KEY", "TOKEN", "SECRET", "PASSWORD", "CREDENTIAL", "PRIVATE", "CLAVE", "PASSPHRASE"} {
		if strings.Contains(nombre, marca) {
			return true
		}
	}
	for _, final := range []string{"_KEY", "_PASS", "_PWD"} {
		if strings.HasSuffix(nombre, final) {
			return true
		}
	}
	return nombre == "SSH_AUTH_SOCK"
}

// credencialEnURL es una URL con usuario y contraseña (postgres://u:p@h, http://u:p@proxy).
var credencialEnURL = regexp.MustCompile(`://[^/@\s:]+:[^/@\s]+@`)

// EntornoSecreto dice si un par «NOMBRE=valor» del entorno no debe llegar a un programa externo: por su nombre
// (VariableSecreta) o porque su valor es una URL con credenciales.
func EntornoSecreto(par string) bool {
	nombre, valor, _ := strings.Cut(par, "=")
	// Un proxy con usuario y clave sí pasa: los CLI lo necesitan para llegar a su proveedor y no tienen herramientas
	// con que filtrarlo.
	proxy := strings.HasSuffix(strings.ToUpper(nombre), "_PROXY")
	return VariableSecreta(nombre) || (!proxy && credencialEnURL.MatchString(valor))
}

// Lista devuelve un valor de tipo lista, o nil si no existe.
func (c *Configuracion) Lista(clave string) []any {
	v, _ := c.e().valores[clave].([]any)
	return v
}

// ExpandirCasa cambia un ~ inicial por la carpeta personal, como hace la terminal en macOS y Linux.
func ExpandirCasa(ruta string) string {
	if ruta != "~" && !strings.HasPrefix(ruta, "~/") && !strings.HasPrefix(ruta, `~\`) {
		return ruta
	}
	casa, err := os.UserHomeDir()
	if err != nil {
		return ruta
	}
	return filepath.Join(casa, ruta[1:])
}

// RutaPermitida resuelve un archivo local que la CLI o la terminal van a enviar y devuelve su ruta real si está
// dentro de instancia_global.raices_permitidas. Sin raíces configuradas no hay límite. Como se resuelven los
// enlaces, un enlace dentro de una raíz que apunte fuera de ella se rechaza.
func (c *Configuracion) RutaPermitida(ruta string) (string, error) {
	abs, err := filepath.Abs(ExpandirCasa(ruta))
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
		raiz, err := filepath.Abs(ExpandirCasa(texto))
		if err != nil {
			continue
		}
		if r, err := filepath.EvalSymlinks(raiz); err == nil {
			raiz = r
		}
		if dentroDe(raiz, real) {
			return real, nil
		}
	}
	return "", NuevoError(CodigoRutaNoPermitida, idioma.Tf("%s está fuera de instancia_global.raices_permitidas", ruta))
}

// dentroDe indica si ruta es raiz o está debajo de ella; fuera de Linux las mayúsculas no cuentan.
func dentroDe(raiz, ruta string) bool {
	if runtime.GOOS != "linux" {
		raiz, ruta = strings.ToLower(raiz), strings.ToLower(ruta)
	}
	rel, err := filepath.Rel(raiz, ruta)
	return err == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ProyectoConfiable indica si la raíz figura en proyectos_confiables. Solo la configuración global (o el entorno)
// puede declararlo: la versionada y local.yaml lo rechazan, así que un repositorio clonado nunca se marca confiable a sí mismo.
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
