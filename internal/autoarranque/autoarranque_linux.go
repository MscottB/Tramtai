//go:build linux

package autoarranque

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Las entradas de ~/.config/autostart de la instalada y de la portable; las pruebas usan otras para no tocar las reales.
var (
	nombreArchivo  = "tramtai.desktop"
	nombrePortable = "tramtai-portable.desktop"
)

func carpeta() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "autostart"), nil
}

// contenido sigue la especificación de arranque automático de freedesktop (GNOME, KDE, Xfce y otros escritorios).
func contenido(escritorio, _ string) string {
	return fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Tramtai
Comment=Tramtai en la bandeja con el servicio en marcha
Exec=%s --oculto
Icon=tramtai
Terminal=false
X-GNOME-Autostart-enabled=true
`, escaparExec(escritorio))
}

// escaparExec entrecomilla una ruta para la clave Exec de freedesktop: dentro de las comillas se escapan ", `, $ y \, y
// después se aplica el escape de las cadenas del archivo (cada \ se duplica); % se escribe %% porque introduce códigos
// de campo como %u.
func escaparExec(ruta string) string {
	citada := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", `$`, `\$`).Replace(ruta)
	return `"` + strings.NewReplacer(`\`, `\\`, `%`, `%%`).Replace(citada) + `"`
}
