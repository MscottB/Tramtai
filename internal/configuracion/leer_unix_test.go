//go:build unix

package configuracion

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestLeerSinColgarseNiSeguirEnlaces: la configuración y los perfiles se leen sin quedarse colgados si son un FIFO (los
// plantaría la cuenta del servicio, que escribe en la carpeta del servidor, para colgar la CLI de root) y, en el
// servidor, sin seguir enlaces. Fuera del servidor un enlace vale (config.yaml en un repositorio de dotfiles).
func TestLeerSinColgarseNiSeguirEnlaces(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "config.yaml")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	hecho := make(chan error, 1)
	go func() { _, err := CargarCapa(fifo, OrigenGlobal); hecho <- err }()
	select {
	case err := <-hecho:
		if err == nil {
			t.Error("un FIFO no es una configuración")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("leer un FIFO como config.yaml se queda colgado")
	}
	if _, err := ClavesAdministradas(fifo); err == nil {
		t.Error("ClavesAdministradas: un FIFO no es una configuración")
	}

	real := filepath.Join(dir, "real.yaml")
	if err := os.WriteFile(real, []byte("servicio:\n  puerto: 17400\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	enlace := filepath.Join(dir, "enlace.yaml")
	if err := os.Symlink(real, enlace); err != nil {
		t.Fatal(err)
	}
	if c, err := CargarCapa(enlace, OrigenGlobal); err != nil || c.Valores["servicio.puerto"] == nil {
		t.Errorf("fuera del servidor un enlace vale: %+v %v", c.Valores, err)
	}
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	if _, err := CargarCapa(enlace, OrigenGlobal); err == nil {
		t.Error("en el servidor no se sigue un enlace")
	}
	perfiles := filepath.Join(dir, "profiles")
	if err := os.MkdirAll(perfiles, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(perfiles, "fifo.yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &Configuracion{perfiles: []carpetaPerfiles{{dir: perfiles, limite: dir}}}
	go func() { _, _, err := c.buscarPerfil("fifo"); hecho <- err }()
	select {
	case err := <-hecho:
		if err == nil {
			t.Error("un FIFO no es un perfil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("leer un FIFO como perfil se queda colgado")
	}
}
