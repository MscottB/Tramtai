// Package version expone la versión del ejecutable para la CLI, el servicio y la aplicación de escritorio.
package version

import (
	"runtime/debug"

	"tramtai/internal/idioma"
)

// Version se inyecta al compilar: -ldflags "-X tramtai/internal/version.Version=vX.Y.Z+DDMMAAAA" (scripts/compilar.ps1).
var Version = "dev"

// Commit devuelve el commit compilado y si había cambios sin confirmar, según los datos que Go incrusta al compilar.
func Commit() (revision string, modificado bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modificado = s.Value == "true"
		}
	}
	return revision, modificado
}

// Texto es la versión que ven las personas, por ejemplo "v0.7.1+12102026".
func Texto() string { return Version }

// Detalle añade el commit compilado, por ejemplo "v0.1.0+12102026 (7deffc4)"; solo para tramtai version --detalle.
func Detalle() string {
	revision, modificado := Commit()
	if revision == "" {
		return Version
	}
	detalle := revision[:min(7, len(revision))]
	if modificado {
		detalle += idioma.T(", con cambios sin confirmar")
	}
	return Version + " (" + detalle + ")"
}
