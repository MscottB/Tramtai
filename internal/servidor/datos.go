package servidor

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"tramtai/internal/configuracion"
	"tramtai/internal/idioma"
	"tramtai/internal/terminos"
)

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

// configInicial es la configuración del servidor recién instalado: solo en este equipo.
const configInicial = "servicio:\n  host: 127.0.0.1\n"

// prepararDatos crea la carpeta del servidor y datos/, solo de la cuenta del servicio (0700), la configuración inicial
// (si falta), la aceptación de los términos y, con un paquete, el conversor, y lo deja todo a nombre de la cuenta del
// servicio (en Linux; en Windows lo resuelve la DACL heredable). La raíz es de esa cuenta, que podría dejar enlaces o
// puntos de unión para que el administrador escribiera fuera (datos/terminos.json → /etc/shadow): dentro de ella todo
// pasa por un os.Root, y datos o config.yaml enlazados son un error.
func prepararDatos(raiz, como, conversor string) error {
	if err := os.MkdirAll(raiz, 0o700); err != nil { // /var/lib es de root: la raíz en sí no la puede cambiar la cuenta
		return err
	}
	r, err := abrirRaiz(raiz)
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
	if conversor != "" {
		if err := instalarConversor(r, conversor); err != nil {
			return err
		}
	}
	return ajustarDueno(r)
}

// abrirRaiz abre la raíz del servidor como os.Root y comprueba que es la carpeta que se miró antes de abrirla, sin
// seguir enlaces: si entre medias la cambiaron por un enlace o un punto de unión (con el servicio en marcha, al
// reinstalar), es un error en lugar de trabajar en otra carpeta.
func abrirRaiz(raiz string) (*os.Root, error) {
	antes, err := os.Lstat(raiz)
	if err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(raiz)
	if err != nil {
		return nil, err
	}
	f, err := r.Open(".")
	if err == nil {
		var ahora fs.FileInfo
		if ahora, err = f.Stat(); err == nil && (antes.Mode().Type() != fs.ModeDir || !os.SameFile(antes, ahora)) {
			err = tipoInesperado(raiz)
		}
		f.Close()
	}
	if err != nil {
		r.Close()
		return nil, err
	}
	return r, nil
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

// leerConfiguracion lee config.yaml de la raíz del servidor sin seguir un enlace. En Linux lo abre sin bloquear y
// comprueba el descriptor abierto: si entre el Lstat y la apertura lo cambiaron por un FIFO, root no se queda colgado
// leyéndolo.
func leerConfiguracion(r *os.Root) (*configuracion.Configuracion, error) {
	if err := comprobarTipo(r, "config.yaml", 0); err != nil {
		return nil, err
	}
	f, err := r.OpenFile("config.yaml", os.O_RDONLY|sinBloquear, 0)
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
