//go:build windows

package tui

import (
	"os"

	"golang.org/x/sys/windows"
)

// consolaUTF8 dice si la consola pinta UTF-8. Windows Terminal y las terminales de los editores sí, aunque la página de
// códigos diga otra cosa; la consola clásica, solo con la 65001 (chcp 65001).
func consolaUTF8() bool {
	if os.Getenv("WT_SESSION") != "" || os.Getenv("TERM_PROGRAM") != "" {
		return true
	}
	cp, err := windows.GetConsoleOutputCP()
	return err != nil || cp == 0 || cp == 65001 // sin consola (una tubería, el CI): no hay nada que estropear
}
