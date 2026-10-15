//go:build darwin

package autoarranque

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
)

// nombreArchivo es el LaunchAgent del usuario; las pruebas usan otro para no tocar el real.
var nombreArchivo = "com.mscottb.tramtai.plist"

func archivo() (string, error) {
	casa, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(casa, "Library", "LaunchAgents", nombreArchivo), nil
}

// contenido es un LaunchAgent de launchd que abre la app oculta, en la barra de menús, al iniciar sesión.
func contenido(escritorio string) string {
	var ruta bytes.Buffer
	_ = xml.EscapeText(&ruta, []byte(escritorio))
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.mscottb.tramtai</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>--oculto</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, ruta.String())
}
