//go:build linux

package servidor

import (
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
	"syscall"

	"tramtai/internal/idioma"
	"tramtai/internal/terminos"
)

// envoltorio es tramtai-servidor: el programa del servidor con la raíz de datos del servidor.
const envoltorio = "#!/bin/sh\nexec /opt/tramtai/tramtai --servidor \"$@\"\n"

// sinBloquear abre config.yaml sin quedarse colgado si es un FIFO (leerConfiguracion).
const sinBloquear = syscall.O_NONBLOCK

// conSystemd dice si el sistema arrancó con systemd (como sd_booted); las pruebas lo sustituyen.
var conSystemd = func() bool {
	_, err := os.Stat("/run/systemd/system")
	return err == nil
}

// Cambiar el dueño a la cuenta del servicio; las pruebas lo sustituyen (no hay cuenta tramtai ni root). lchown no sigue
// enlaces y no sale de la raíz.
var (
	idsServicio = func() (uid, gid int, err error) { return idsDe(Cuenta) }
	lchown      = func(r *os.Root, nombre string, uid, gid int) error { return r.Lchown(nombre, uid, gid) }
)

// idsDe son el uid y el gid (el del grupo principal) de una cuenta.
func idsDe(cuenta string) (uid, gid int, err error) {
	u, err := user.Lookup(cuenta)
	if err != nil {
		return 0, 0, err
	}
	if uid, err = strconv.Atoi(u.Uid); err != nil {
		return 0, 0, err
	}
	gid, err = strconv.Atoi(u.Gid)
	return uid, gid, err
}

// AjustarDueno deja toda la raíz del servidor a nombre de la cuenta del servicio: lo que la CLI escribe como root
// (configuración, términos, administrador) el servicio, que corre como tramtai, no podría leerlo. La raíz es de esa
// cuenta, que podría dejar enlaces: el recorrido va por un os.Root, no sigue enlaces y no sale de ella. Sin la raíz no
// hace nada, y tampoco sin la cuenta (aún no se ha instalado).
func AjustarDueno(raiz string) error {
	if _, _, err := idsServicio(); errors.As(err, new(user.UnknownUserError)) {
		return nil
	}
	r, err := os.OpenRoot(raiz)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer r.Close()
	return ajustarDueno(r)
}

func ajustarDueno(r *os.Root) error {
	uid, gid, err := idsServicio()
	if err != nil {
		return err
	}
	return fs.WalkDir(r.FS(), ".", func(nombre string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			var info fs.FileInfo
			// Un enlace duro puede ser un archivo de fuera (/etc/shadow, con fs.protected_hardlinks=0): no se regala.
			if info, err = r.Lstat(nombre); err == nil && info.Sys().(*syscall.Stat_t).Nlink > 1 {
				return nil
			}
		}
		if err == nil {
			err = lchown(r, nombre, uid, gid)
		}
		if errors.Is(err, fs.ErrNotExist) { // el servicio, en marcha, renombró o borró un temporal: se sigue con el resto
			return nil
		}
		return err
	})
}

// EsAdministrador dice si el proceso corre como root.
func EsAdministrador() bool { return os.Geteuid() == 0 }

// orden lanza una orden del sistema; si falla, el error lleva lo que dijo.
func orden(nombre string, args ...string) error {
	if salida, err := ejecutar(nombre, args...); err != nil {
		return fmt.Errorf(idioma.T("falló «%s» (%v): %s"), strings.Join(append([]string{nombre}, args...), " "), err, strings.TrimSpace(string(salida)))
	}
	return nil
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
		// --user-group: el grupo tramtai también donde USERGROUPS_ENAB es no.
		if err := orden("useradd", "--system", "--user-group", "--home", r.Datos, "--shell", "/usr/sbin/nologin", Cuenta); err != nil {
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
	if err := prepararDatos(r.Datos, cmp.Or(o.Como, terminos.ComoServidorInstalar), o.Conversor); err != nil {
		return err
	}
	nueva, err := actualizarUnidad(r, UnidadOpciones{LetsEncrypt: o.LetsEncrypt, PuertoBajo: o.PuertoBajo})
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

// actualizarUnidad escribe la unidad con la configuración actual del servidor, si cambió, y dice si la cambió. extra
// suma lo que pida quien llama; Let's Encrypt (servicio.tls.modo) se calculará aquí, como el puerto bajo.
func actualizarUnidad(s Sitios, extra UnidadOpciones) (bool, error) {
	r, err := os.OpenRoot(s.Datos)
	if err != nil {
		return false, err
	}
	defer r.Close()
	c, err := leerConfiguracion(r)
	if err != nil {
		return false, err
	}
	o := UnidadOpciones{PlazoDrenaje: c.Entero("servicio.plazo_drenaje_segundos"), LetsEncrypt: extra.LetsEncrypt,
		PuertoBajo: extra.PuertoBajo || c.Entero("servicio.puerto") < 1024}
	return escribirSiCambia(s.Unidad, []byte(Unidad(o)), 0o644)
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

// Reiniciar reinicia el servicio con la unidad al día con la configuración (plazo de drenaje, puerto bajo): si cambió,
// la reescribe y hace daemon-reload antes. El que se detiene deja terminar las peticiones en curso.
func Reiniciar() error {
	r := Rutas()
	if err := AjustarDueno(r.Datos); err != nil { // lo que root haya dejado ahí, legible para el servicio que arranca
		return err
	}
	if existe(r.Unidad) {
		// Sin opciones extra: la Tarea 11 derivará LetsEncrypt de servicio.tls.modo en actualizarUnidad.
		cambiada, err := actualizarUnidad(r, UnidadOpciones{})
		if err != nil {
			return err
		}
		if cambiada {
			if err := orden("systemctl", "daemon-reload"); err != nil {
				return err
			}
		}
	}
	return orden("systemctl", "restart", "tramtai")
}
