package procesos

import "testing"

// Sin ningún hilo que reanudar (un PID que no existe), reanudar da error en lugar de dejar el proceso suspendido.
func TestReanudarSinHilos(t *testing.T) {
	if err := reanudar(0xFFFFFFF0); err == nil {
		t.Fatal("reanudar un proceso sin hilos debe dar error")
	}
}
