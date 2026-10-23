//go:build darwin

package autoarranque

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Los LaunchAgents del usuario de la instalada y de la portable; las pruebas usan otros para no tocar los reales.
var (
	nombreArchivo  = "com.mscottb.tramtai.plist"
	nombrePortable = "com.mscottb.tramtai.portable.plist"
)

func carpeta() (string, error) {
	casa, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(casa, "Library", "LaunchAgents"), nil
}

// contenido es un LaunchAgent de launchd que abre la app oculta, en la barra de menús, al iniciar sesión. Su Label es
// el nombre del archivo: launchd no carga dos con la misma, y la instalada y la portable pueden estar las dos activas.
func contenido(escritorio, nombre string) string {
	var ruta bytes.Buffer
	_ = xml.EscapeText(&ruta, []byte(escritorio))
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>--oculto</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, strings.TrimSuffix(nombre, ".plist"), ruta.String())
}
