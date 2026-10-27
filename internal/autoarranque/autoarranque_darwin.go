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

// contenido es un LaunchAgent de launchd que abre la app oculta, en la barra de menús, al iniciar sesión o, en
// ModoServicio, inicia solo el servicio (tramtai servicio iniciar -d). Su Label es el nombre del archivo: launchd no
// carga dos con la misma, y la instalada y la portable pueden estar las dos activas.
func contenido(escritorio string, modo Modo, nombre string) string {
	argumentos := []string{escritorio, "--oculto"}
	if modo == ModoServicio {
		argumentos = []string{programa(escritorio), "servicio", "iniciar", "-d"}
	}
	var lista strings.Builder
	for _, a := range argumentos {
		var escapado bytes.Buffer
		_ = xml.EscapeText(&escapado, []byte(a))
		fmt.Fprintf(&lista, "\t\t<string>%s</string>\n", escapado.String())
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
%s	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, strings.TrimSuffix(nombre, ".plist"), lista.String())
}
