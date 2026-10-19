//go:build windows

package instalacion

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// Un proceso de la carpeta aparece (y se cierra); con otra carpeta, no. Hace de proceso una copia del binario de la
// prueba, que espera sin hacer nada.
func TestProcesosEn(t *testing.T) {
	if os.Getenv("ESPERAR") == "1" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	propio, _ := os.Executable()
	carpeta := t.TempDir()
	copia := filepath.Join(carpeta, "tramtai-prueba.exe")
	datos, _ := os.ReadFile(propio)
	if err := os.WriteFile(copia, datos, 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(copia, "-test.run=TestProcesosEn")
	cmd.Env = append(os.Environ(), "ESPERAR=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	pid := uint32(cmd.Process.Pid)

	pids, err := ProcesosEn(carpeta, "TRAMTAI-PRUEBA.EXE")
	if err != nil || !slices.Contains(pids, pid) {
		t.Fatalf("no apareció el proceso de la carpeta: %v %v", pids, err)
	}
	if pids, _ := ProcesosEn(t.TempDir(), "tramtai-prueba.exe"); slices.Contains(pids, pid) {
		t.Fatal("apareció un proceso de otra carpeta")
	}
	if err := Cerrar(carpeta, "tramtai-prueba.exe"); err != nil {
		t.Fatal(err)
	}
	listo := make(chan error, 1)
	go func() { listo <- cmd.Wait() }()
	select {
	case <-listo:
	case <-time.After(10 * time.Second):
		t.Fatal("Cerrar no terminó el proceso")
	}
}
