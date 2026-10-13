//go:build windows

package main

import "github.com/wailsapp/wails/v3/pkg/w32"

// ladoIconoMenu es el lado, en píxeles y ya escalado al DPI, del hueco de los menús donde van los iconos.
// Wails no escala las imágenes del menú, así que se dibujan a esta medida.
func ladoIconoMenu() int {
	const smCxMenuCheck = 71
	if n := w32.GetSystemMetrics(smCxMenuCheck); n >= 12 {
		return n
	}
	return 16
}

// menuOscuro indica si los menús de la bandeja se ven en modo oscuro (siguen el tema de las aplicaciones).
func menuOscuro() bool { return w32.IsCurrentlyDarkMode() }
