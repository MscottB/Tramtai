//go:build linux

package servidor

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

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
	if err := prepararDatos(r.Datos, cmp.Or(o.Como, terminos.ComoServidorInstalar)); err != nil {
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

// configInicial es la configuración del servidor recién instalado: solo en este equipo.
const configInicial = "servicio:\n  host: 127.0.0.1\n"

// prepararDatos crea la carpeta del servidor y datos/, solo de la cuenta del servicio (0700), la configuración inicial
// (si falta) y la aceptación de los términos, y lo deja todo a nombre de la cuenta del servicio. La raíz es de esa
// cuenta, que podría dejar enlaces para que root escribiera fuera (datos/terminos.json → /etc/shadow): dentro de ella
// todo pasa por un os.Root, y datos o config.yaml enlazados son un error.
func prepararDatos(raiz, como string) error {
	if err := os.MkdirAll(raiz, 0o700); err != nil { // /var/lib es de root: la raíz en sí no la puede cambiar la cuenta
		return err
	}
	r, err := os.OpenRoot(raiz)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := r.Mkdir("datos", 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	if err := comprobarTipo(r, "datos", fs.ModeDir); err != nil {
		return err
	}
	for _, d := range []string{".", "datos"} { // ya existían con otros permisos
		if err := r.Chmod(d, 0o700); err != nil {
			return err
		}
	}
	if _, err := r.Lstat("config.yaml"); errors.Is(err, fs.ErrNotExist) {
		if err := escribirEn(r, "config.yaml", []byte(configInicial), 0o600); err != nil {
			return err
		}
	}
	if _, err := leerConfiguracion(r); err != nil {
		return err
	}
	contenido, err := terminos.Contenido(como)
	if err != nil {
		return err
	}
	if err := escribirEn(r, "datos/terminos.json", contenido, 0o600); err != nil {
		return err
	}
	return ajustarDueno(r)
}

// comprobarTipo falla si nombre no es del tipo dado (una carpeta, o un archivo normal con 0): un enlace no vale.
func comprobarTipo(r *os.Root, nombre string, tipo fs.FileMode) error {
	info, err := r.Lstat(nombre)
	if err != nil {
		return err
	}
	if info.Mode().Type() != tipo {
		return tipoInesperado(filepath.Join(r.Name(), nombre))
	}
	return nil
}

// tipoInesperado es el error de una ruta de la raíz que no es lo que se espera: un enlace, un FIFO…
type tipoInesperado string

func (e tipoInesperado) Error() string {
	return idioma.Tf("%s no es lo que se espera (¿un enlace?): quítalo y repite la instalación", string(e))
}

// escribirEn escribe un archivo de dentro de la raíz en un temporal nuevo y lo renombra encima: si el destino era un
// enlace, se sustituye el enlace, no lo que hay detrás.
func escribirEn(r *os.Root, nombre string, datos []byte, permisos os.FileMode) error {
	temporal := nombre + "." + rand.Text() + ".tmp"
	f, err := r.OpenFile(temporal, os.O_WRONLY|os.O_CREATE|os.O_EXCL, permisos)
	if err != nil {
		return err
	}
	_, err = f.Write(datos)
	if err == nil {
		err = f.Chmod(permisos) // sin la umask
	}
	if err == nil {
		err = f.Sync()
	}
	if cerrar := f.Close(); err == nil {
		err = cerrar
	}
	if err == nil {
		err = r.Rename(temporal, nombre)
	}
	if err != nil {
		_ = r.Remove(temporal)
	}
	return err
}

// leerConfiguracion lee config.yaml de la raíz del servidor sin seguir un enlace. Lo abre sin bloquear y comprueba el
// descriptor abierto: si entre el Lstat y la apertura lo cambiaron por un FIFO, root no se queda colgado leyéndolo.
func leerConfiguracion(r *os.Root) (*configuracion.Configuracion, error) {
	if err := comprobarTipo(r, "config.yaml", 0); err != nil {
		return nil, err
	}
	f, err := r.OpenFile("config.yaml", os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		return nil, cmp.Or(err, error(tipoInesperado(filepath.Join(r.Name(), "config.yaml"))))
	}
	bruto, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	capa, err := configuracion.CargarCapaDe(bruto, filepath.Join(r.Name(), "config.yaml"), configuracion.OrigenGlobal)
	if err != nil {
		return nil, err
	}
	return configuracion.Resolver(capa), nil
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
