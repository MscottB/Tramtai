package main

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"runtime"
	"strings"
)

// uniqueID identifica la app para la instancia única: la instalada usa siempre el mismo, y cada portable el suyo según
// su carpeta, así que abrir una no trae al frente a la otra. La carpeta se compara como el sistema (sin distinguir
// mayúsculas fuera de Linux) y la parte variable empieza por «p» porque D-Bus, en Linux, no admite una que empiece por
// un dígito.
func uniqueID(carpeta string, portable bool) string {
	if !portable {
		return "cl.tramtai.escritorio"
	}
	carpeta = filepath.Clean(carpeta)
	if runtime.GOOS != "linux" {
		carpeta = strings.ToLower(carpeta)
	}
	suma := sha256.Sum256([]byte(carpeta))
	return "cl.tramtai.escritorio.portable.p" + hex.EncodeToString(suma[:4])
}
