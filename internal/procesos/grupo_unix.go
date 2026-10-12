//go:build !windows

package procesos

import (
	"os/exec"
	"syscall"
	"time"
)

// preparar lanza el proceso como líder de un grupo nuevo para poder señalar a todo el árbol.
// shortcut: si el servicio muere de golpe, el grupo sigue vivo (en Windows el Job Object lo cierra el sistema);
// hace falta guardar pgid y firma de inicio y terminarlos al arrancar (sección 4.3) antes de soportar Unix en M8.
func preparar(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

type grupo struct{ pgid int }

func agrupar(cmd *exec.Cmd) (*grupo, error) { return &grupo{pgid: cmd.Process.Pid}, nil }

// terminar pide parada cooperativa (SIGTERM) y fuerza SIGKILL a los 3 segundos.
func (g *grupo) terminar() {
	_ = syscall.Kill(-g.pgid, syscall.SIGTERM)
	time.AfterFunc(3*time.Second, func() { _ = syscall.Kill(-g.pgid, syscall.SIGKILL) })
}

func (g *grupo) cerrar() { _ = syscall.Kill(-g.pgid, syscall.SIGKILL) }
