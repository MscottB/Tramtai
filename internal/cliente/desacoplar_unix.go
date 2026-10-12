//go:build !windows

package cliente

import (
	"os/exec"
	"syscall"
)

// desacoplar lanza el servicio en una sesión nueva: sobrevive al cierre de la terminal.
func desacoplar(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
