//go:build windows

package actualizacion

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

// Instalada devuelve la carpeta de una instalación hecha con el instalador: la del ejecutable, si tiene al lado
// desinstalar.exe. Una copia compilada a mano no se actualiza sola.
func Instalada() (string, bool) {
	ejecutable, err := os.Executable()
	if err != nil {
		return "", false
	}
	if real, err := filepath.EvalSymlinks(ejecutable); err == nil {
		ejecutable = real
	}
	carpeta := filepath.Dir(ejecutable)
	if _, err := os.Stat(filepath.Join(carpeta, "desinstalar.exe")); err != nil {
		return "", false
	}
	return carpeta, true
}

// Instalar ejecuta el instalador en silencio sobre carpeta y no lo espera: el instalador cierra la app, detiene el
// servicio (también el proceso que lo lanzó), reemplaza los archivos y vuelve a abrir la app (/abrir).
func Instalar(instalador, carpeta string) error {
	cmd := exec.Command(instalador)
	// NSIS exige /D= al final y sin comillas aunque la ruta tenga espacios: la línea de órdenes se escribe a mano.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       syscall.EscapeArg(instalador) + " /S /abrir /D=" + carpeta,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
