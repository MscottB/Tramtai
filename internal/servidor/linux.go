//go:build linux

package servidor

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"tramtai/internal/configuracion"
	"tramtai/internal/idioma"
	"tramtai/internal/terminos"
)

// envoltorio es tramtai-servidor: el programa del servidor con la raíz de datos del servidor.
const envoltorio = "#!/bin/sh\nexec /opt/tramtai/tramtai --servidor \"$@\"\n"

// conSystemd dice si el sistema arrancó con systemd (como sd_booted); las pruebas lo sustituyen.
var conSystemd = func() bool {
	_, err := os.Stat("/run/systemd/system")
	return err == nil
}

// chown deja una ruta a nombre de la cuenta del servicio; las pruebas lo sustituyen (no hay cuenta tramtai ni root).
var chown = func(ruta string) error { return cambiarDueno(ruta, Cuenta) }

func cambiarDueno(ruta, cuenta string) error {
	u, err := user.Lookup(cuenta)
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}
	return os.Lchown(ruta, uid, gid)
}

// AjustarDueno deja a nombre de la cuenta del servicio lo que la CLI, como root, escribe en la raíz del servidor: si no,
// el servicio, que corre como tramtai, no podría leerlo.
func AjustarDueno(ruta string) error { return chown(ruta) }

// EsAdministrador dice si el proceso corre como root.
func EsAdministrador() bool { return os.Geteuid() == 0 }

// orden lanza una orden del sistema; si falla, el error lleva lo que dijo.
func orden(nombre string, args ...string) error {
	if salida, err := ejecutar(nombre, args...); err != nil {
		return fmt.Errorf(idioma.T("falló «%s» (%v): %s"), strings.Join(append([]string{nombre}, args...), " "), err, strings.TrimSpace(string(salida)))
	}
	return nil
}

func existe(ruta string) bool {
	_, err := os.Stat(ruta)
	return err == nil
}

// escribirSiCambia escribe el archivo solo si su contenido es otro, y dice si lo cambió. Pone uno nuevo en su lugar:
// el programa en marcha no se puede sobrescribir.
func escribirSiCambia(ruta string, contenido []byte, permisos os.FileMode) (bool, error) {
	if actual, err := os.ReadFile(ruta); err == nil && bytes.Equal(actual, contenido) {
		return false, nil
	}
	return true, configuracion.EscribirAtomico(ruta, contenido, permisos)
}

// Instalar crea la cuenta del servicio, copia el programa, prepara la carpeta de datos, escribe la unidad y la habilita
// y arranca. Repetirla es seguro: con la misma versión no cambia nada; con otra, actualiza y reinicia.
func Instalar(o Opciones) error {
	if !o.AceptoTerminos {
		return sinAceptar()
	}
	if !conSystemd() {
		return noDisponible()
	}
	r := Rutas()
	if _, err := ejecutar("id", Cuenta); err != nil {
		if err := orden("useradd", "--system", "--home", r.Datos, "--shell", "/usr/sbin/nologin", Cuenta); err != nil {
			return err
		}
	}
	instalado := existe(filepath.Join(r.Programa, "tramtai"))
	// La cuenta del servicio solo lee el programa: es de root.
	if err := os.MkdirAll(r.Programa, 0o755); err != nil {
		return err
	}
	cambiado := false
	for _, nombre := range []string{"tramtai", "uv"} {
		contenido, err := os.ReadFile(filepath.Join(o.Origen, nombre))
		if err != nil {
			return fmt.Errorf(idioma.T("falta %s junto a este tramtai: instala el servidor desde el .tar.gz de Linux (%v)"), nombre, err)
		}
		c, err := escribirSiCambia(filepath.Join(r.Programa, nombre), contenido, 0o755)
		if err != nil {
			return err
		}
		cambiado = cambiado || c
	}
	if _, err := escribirSiCambia(r.Envoltorio, []byte(envoltorio), 0o755); err != nil {
		return err
	}
	config, err := prepararDatos(r.Datos, cmp.Or(o.Como, terminos.ComoServidorInstalar))
	if err != nil {
		return err
	}
	capa, err := configuracion.CargarCapa(config, configuracion.OrigenGlobal)
	if err != nil {
		return err
	}
	c := configuracion.Resolver(capa)
	unidad := Unidad(UnidadOpciones{PlazoDrenaje: c.Entero("servicio.plazo_drenaje_segundos"), LetsEncrypt: o.LetsEncrypt,
		PuertoBajo: o.PuertoBajo || c.Entero("servicio.puerto") < 1024})
	nueva, err := escribirSiCambia(r.Unidad, []byte(unidad), 0o644)
	if err != nil {
		return err
	}
	cambiado = cambiado || nueva
	if cambiado {
		if err := orden("systemctl", "daemon-reload"); err != nil {
			return err
		}
	}
	if err := orden("systemctl", "enable", "--now", "tramtai"); err != nil {
		return err
	}
	if cambiado && instalado { // ya estaba en marcha con el programa o la unidad de antes
		return orden("systemctl", "restart", "tramtai")
	}
	return nil
}

// prepararDatos crea la carpeta del servidor y datos/, solo de la cuenta del servicio (0700), la configuración inicial
// solo en loopback (si falta) y la aceptación de los términos, todo a nombre de la cuenta del servicio. Devuelve la ruta
// de la configuración.
func prepararDatos(raiz, como string) (string, error) {
	datos := filepath.Join(raiz, "datos")
	for _, d := range []string{raiz, datos} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return "", err
		}
		if err := os.Chmod(d, 0o700); err != nil { // ya existía con otros permisos
			return "", err
		}
		if err := chown(d); err != nil {
			return "", err
		}
	}
	config := filepath.Join(raiz, "config.yaml")
	if !existe(config) {
		if err := configuracion.FijarValor(config, configuracion.OrigenGlobal, "servicio.host", "127.0.0.1"); err != nil {
			return "", err
		}
	}
	if err := chown(config); err != nil {
		return "", err
	}
	if err := terminos.Aceptar(datos, como); err != nil {
		return "", err
	}
	return config, chown(filepath.Join(datos, "terminos.json"))
}

// Desinstalar detiene el servicio (con drenaje) y quita la unidad, el programa y el envoltorio. Con borrarDatos borra
// también la carpeta de datos y la cuenta.
func Desinstalar(borrarDatos bool) error {
	r := Rutas()
	if existe(r.Unidad) {
		if err := orden("systemctl", "disable", "--now", "tramtai"); err != nil {
			return err
		}
		if err := os.Remove(r.Unidad); err != nil {
			return err
		}
		if err := orden("systemctl", "daemon-reload"); err != nil {
			return err
		}
	}
	if err := os.Remove(r.Envoltorio); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.RemoveAll(r.Programa); err != nil {
		return err
	}
	if !borrarDatos {
		return nil
	}
	if err := os.RemoveAll(r.Datos); err != nil {
		return err
	}
	if _, err := ejecutar("id", Cuenta); err == nil {
		return orden("userdel", Cuenta)
	}
	return nil
}

// Estado dice si la unidad está instalada y lo que dice systemctl is-active (que sale con error si no está activa).
func Estado(_ context.Context) (EstadoSistema, error) {
	if !existe(Rutas().Unidad) {
		return EstadoSistema{}, nil
	}
	salida, err := ejecutar("systemctl", "is-active", "tramtai")
	activo := strings.TrimSpace(string(salida))
	if activo == "" && err != nil {
		return EstadoSistema{Instalado: true}, err
	}
	return EstadoSistema{Instalado: true, Activo: activo}, nil
}

// Reiniciar reinicia el servicio; el que se detiene deja terminar las peticiones en curso.
func Reiniciar() error { return orden("systemctl", "restart", "tramtai") }
