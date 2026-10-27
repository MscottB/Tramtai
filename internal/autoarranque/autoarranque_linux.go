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

// contenido sigue la especificación de arranque automático de freedesktop (GNOME, KDE, Xfce y otros escritorios). Abre la
// app oculta en la bandeja o, en ModoServicio, inicia solo el servicio.
func contenido(escritorio string, modo Modo, _ string) string {
	comentario, orden := "Tramtai en la bandeja con el servicio en marcha", escaparExec(escritorio)+" --oculto"
	if modo == ModoServicio {
		comentario, orden = "Tramtai: el servicio en marcha, sin ventana", escaparExec(programa(escritorio))+" servicio iniciar -d"
	}
	return fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Tramtai
Comment=%s
Exec=%s
Icon=tramtai
Terminal=false
X-GNOME-Autostart-enabled=true
`, comentario, orden)
}

// escaparExec entrecomilla una ruta para la clave Exec de freedesktop: dentro de las comillas se escapan ", `, $ y \, y
// después se aplica el escape de las cadenas del archivo (cada \ se duplica); % se escribe %% porque introduce códigos
// de campo como %u.
func escaparExec(ruta string) string {
	citada := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", `$`, `\$`).Replace(ruta)
	return `"` + strings.NewReplacer(`\`, `\\`, `%`, `%%`).Replace(citada) + `"`
}
