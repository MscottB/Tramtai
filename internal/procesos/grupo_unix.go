//go:build !windows

package procesos

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// SinVentana no hace nada fuera de Windows: allí ningún proceso abre una ventana de consola propia.
func SinVentana(*exec.Cmd) {}

// preparar lanza el proceso como líder de un grupo nuevo para poder señalar a todo el árbol.
// Si el servicio muere de golpe el grupo sigue vivo: la recuperación lo termina con TerminarHuerfano.
func preparar(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

type grupo struct{ pgid int }

// agrupar no hace nada más en Unix: el grupo es el del proceso (Setpgid) y el tope de memoria lo pone conTope.
func agrupar(cmd *exec.Cmd, _ int) (*grupo, error) {
	return &grupo{pgid: cmd.Process.Pid}, nil
}

// terminar pide parada cooperativa (SIGTERM) y fuerza SIGKILL a los 3 segundos.
func (g *grupo) terminar() {
	_ = syscall.Kill(-g.pgid, syscall.SIGTERM)
	time.AfterFunc(3*time.Second, func() { _ = syscall.Kill(-g.pgid, syscall.SIGKILL) })
}

func (g *grupo) cerrar() { _ = syscall.Kill(-g.pgid, syscall.SIGKILL) }

// firmaInicio es la hora de inicio del proceso según ps (Linux y macOS); vacía si no existe.
func firmaInicio(pid int) string {
	salida, err := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(salida))
}

// terminarArbol mata el grupo del que el proceso es líder (Setpgid en preparar).
func terminarArbol(pid int) error { return syscall.Kill(-pid, syscall.SIGKILL) }
