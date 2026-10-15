//go:build windows

package idioma

import "golang.org/x/sys/windows"

// delSistema es el primer idioma de la interfaz de Windows del usuario, por ejemplo "es-CL".
func delSistema() string {
	if idiomas, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME); err == nil && len(idiomas) > 0 {
		return idiomas[0]
	}
	return ""
}
