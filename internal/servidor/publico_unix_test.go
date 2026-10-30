//go:build !windows

package servidor

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestPuertoPublicadoNoRegular: en /run/tramtai escribe la cuenta del servicio, así que el Tramtai del usuario (o la CLI
// como root) no se queda colgado con un FIFO que ponga ahí ni sigue un enlace: solo lee un archivo normal.
func TestPuertoPublicadoNoRegular(t *testing.T) {
	ruta := conRutaPublica(t)
	if err := syscall.Mkfifo(ruta, 0o644); err != nil {
		t.Fatal(err)
	}
	hecho := make(chan bool, 1)
	go func() { _, ok := PuertoDelSistema(); hecho <- ok }()
	select {
	case ok := <-hecho:
		if ok {
			t.Error("un FIFO no es el puerto")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("leer el puerto se queda colgado con un FIFO")
	}
	os.Remove(ruta)
	destino := filepath.Join(t.TempDir(), "otro")
	if err := os.WriteFile(destino, []byte("17374\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(destino, ruta); err != nil {
		t.Fatal(err)
	}
	if p, ok := PuertoDelSistema(); ok {
		t.Errorf("un enlace no es el puerto: %d", p)
	}
}
