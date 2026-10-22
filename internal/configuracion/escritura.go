package configuracion

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// escritores serializa, dentro del proceso, las escrituras de cada ruta: en Windows dos renombrados a la vez sobre el
// mismo archivo pueden fallar.
var escritores sync.Map // ruta → *sync.Mutex

// EscribirAtomico escribe datos en un temporal propio de la misma carpeta y lo renombra: quien lee a la vez (un Recargar,
// la CLI, la TUI) ve el archivo anterior o el nuevo, nunca uno a medias, y un corte a mitad no deja el archivo truncado.
func EscribirAtomico(ruta string, datos []byte, permisos os.FileMode) error {
	if real, err := filepath.EvalSymlinks(ruta); err == nil {
		ruta = real // un config.yaml enlazado (dotfiles) sigue siendo un enlace: se escribe el archivo al que apunta
	}
	m, _ := escritores.LoadOrStore(filepath.Clean(ruta), &sync.Mutex{})
	m.(*sync.Mutex).Lock()
	defer m.(*sync.Mutex).Unlock()
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
		err = renombrar(temporal, ruta)
	}
	if err != nil {
		_ = os.Remove(temporal)
	}
	return err
}

// renombrar reintenta en Windows durante unos 600 ms: allí Rename falla mientras otro proceso tiene abierto el destino
// (un Recargar que lo lee, la CLI, la terminal o el antivirus). En los demás sistemas no hace falta.
func renombrar(origen, destino string) error {
	err := os.Rename(origen, destino)
	for espera := 5 * time.Millisecond; err != nil && runtime.GOOS == "windows" && espera <= 320*time.Millisecond; espera *= 2 {
		time.Sleep(espera)
		err = os.Rename(origen, destino)
	}
	return err
}
