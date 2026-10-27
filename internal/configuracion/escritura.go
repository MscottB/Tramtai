package configuracion

import (
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"tramtai/internal/carpetas"
	"tramtai/internal/idioma"
)

// escritores serializa, dentro del proceso, las escrituras de cada ruta: en Windows dos renombrados a la vez sobre el
// mismo archivo pueden fallar.
var escritores sync.Map // ruta → *sync.Mutex

// raizServidor es la raíz de datos del servicio del sistema; las pruebas la sustituyen.
var raizServidor = carpetas.RaizServidor

// EscribirAtomico escribe datos en un temporal propio de la misma carpeta y lo renombra: quien lee a la vez (un Recargar,
// la CLI, la TUI) ve el archivo anterior o el nuevo, nunca uno a medias, y un corte a mitad no deja el archivo truncado.
// En modo servidor, lo que va dentro de la raíz del servidor se escribe a través de un os.Root (EscribirEnRaiz).
func EscribirAtomico(ruta string, datos []byte, permisos os.FileMode) error {
	if carpetas.Servidor() {
		raiz := raizServidor()
		if abs, err := filepath.Abs(ruta); err == nil && carpetas.Dentro(raiz, abs) {
			return escribirEnServidor(raiz, abs, datos, permisos)
		}
	}
	if real, err := filepath.EvalSymlinks(ruta); err == nil {
		ruta = real // un config.yaml enlazado (dotfiles) sigue siendo un enlace: se escribe el archivo al que apunta
	}
	defer bloquear(ruta)()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(ruta), filepath.Base(ruta)+".*.tmp")
	if err != nil {
		return err
	}
	temporal := f.Name()
	_, err = f.Write(datos)
	if err == nil {
		err = f.Sync()
	}
	if cerrar := f.Close(); err == nil {
		err = cerrar
	}
	if err == nil {
		err = os.Chmod(temporal, permisos)
	}
	if err == nil {
		err = renombrar(os.Rename, temporal, ruta)
	}
	if err != nil {
		_ = os.Remove(temporal)
	}
	return err
}

func bloquear(ruta string) (soltar func()) {
	m, _ := escritores.LoadOrStore(filepath.Clean(ruta), &sync.Mutex{})
	m.(*sync.Mutex).Lock()
	return m.(*sync.Mutex).Unlock
}

// escribirEnServidor escribe dentro de la raíz del servidor. La raíz es de la cuenta del servicio, que podría dejar
// enlaces o puntos de unión para que la CLI, como root o administrador, escribiera fuera (config.yaml → /etc/shadow):
// la raíz no se crea aquí (la crea la instalación), las carpetas intermedias no pueden ser enlaces y todo pasa por un
// os.Root, que nunca sale de ella.
func escribirEnServidor(raiz, ruta string, datos []byte, permisos os.FileMode) error {
	defer bloquear(ruta)()
	r, err := AbrirRaiz(raiz)
	if err != nil {
		return err
	}
	defer r.Close()
	rel, err := filepath.Rel(raiz, ruta)
	if err != nil {
		return err
	}
	partes := strings.Split(rel, string(filepath.Separator))
	for i := 1; i < len(partes); i++ {
		if err := carpetaIntermedia(r, filepath.Join(partes[:i]...)); err != nil {
			return err
		}
	}
	return EscribirEn(r, rel, datos, permisos)
}

// carpetaIntermedia crea la carpeta dentro del Root si falta y falla si es otra cosa (un enlace, una unión).
func carpetaIntermedia(r *os.Root, carpeta string) error {
	info, err := r.Lstat(carpeta)
	if errors.Is(err, fs.ErrNotExist) {
		if err = r.Mkdir(carpeta, 0o700); !errors.Is(err, fs.ErrExist) {
			return err
		}
		info, err = r.Lstat(carpeta) // otra escritura la creó a la vez: vale si es una carpeta
	}
	if err == nil && info.Mode().Type() != fs.ModeDir {
		return ErrTipoInesperado(filepath.Join(r.Name(), carpeta))
	}
	return err
}

// EscribirEn escribe un archivo de dentro de un os.Root en un temporal nuevo y lo renombra encima: si el destino era un
// enlace, se sustituye el enlace, no lo que hay detrás.
func EscribirEn(r *os.Root, nombre string, datos []byte, permisos os.FileMode) error {
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
		err = renombrar(r.Rename, temporal, nombre)
	}
	if err != nil {
		_ = r.Remove(temporal)
	}
	return err
}

// ErrTipoInesperado es una ruta de la raíz del servidor que no es lo que se espera: un enlace, un punto de unión, un FIFO…
type ErrTipoInesperado string

func (e ErrTipoInesperado) Error() string {
	return idioma.Tf("%s no es lo que se espera (¿un enlace?): quítalo y repite la instalación", string(e))
}

// AbrirRaiz abre la raíz del servidor como os.Root y comprueba que es la carpeta que se miró antes de abrirla, sin
// seguir enlaces: si entre medias la cambiaron por un enlace o un punto de unión, es un error en lugar de trabajar en
// otra carpeta.
func AbrirRaiz(raiz string) (*os.Root, error) {
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
			err = ErrTipoInesperado(raiz)
		}
		f.Close()
	}
	if err != nil {
		r.Close()
		return nil, err
	}
	return r, nil
}

// renombrar reintenta en Windows durante unos 600 ms: allí Rename falla mientras otro proceso tiene abierto el destino
// (un Recargar que lo lee, la CLI, la terminal o el antivirus). En los demás sistemas no hace falta.
func renombrar(mover func(origen, destino string) error, origen, destino string) error {
	err := mover(origen, destino)
	for espera := 5 * time.Millisecond; err != nil && runtime.GOOS == "windows" && espera <= 320*time.Millisecond; espera *= 2 {
		time.Sleep(espera)
		err = mover(origen, destino)
	}
	return err
}
