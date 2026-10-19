// Package navegador abre una URL en el navegador del sistema.
package navegador

import (
	"os/exec"
	"runtime"
)

// Abrir abre la URL con el programa del sistema; la URL va como argumento, nunca por una shell.
func Abrir(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // recoge el proceso al terminar: en Unix no queda zombi mientras viva la TUI o la app
	return nil
}
