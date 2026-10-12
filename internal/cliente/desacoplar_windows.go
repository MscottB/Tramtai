//go:build windows

package cliente

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// desacoplar lanza el servicio sin consola ni grupo compartido: sobrevive al cierre de la terminal.
func desacoplar(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS, HideWindow: true}
}
