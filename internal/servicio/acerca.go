package servicio

import (
	"os"
	"path/filepath"
	"runtime"

	"tramtai/internal/carpetas"
	"tramtai/internal/configuracion"
	"tramtai/internal/credenciales"
	"tramtai/internal/version"
)

// acerca son los datos de «Acerca de Tramtai»: versión, compilación, sistema y servicio. Las rutas de carpetas solo van
// a una sesión local, porque llevan el nombre del usuario del sistema. Nunca incluye claves ni tokens.
func (s *Servicio) acerca(local bool) map[string]any {
	commit, modificado := version.Commit()
	datos := map[string]any{"version": version.Texto(), "commit": commit, "modificado": modificado, "go": runtime.Version(),
		"portable": carpetas.Portable(),
		"sistema":  runtime.GOOS, "arquitectura": runtime.GOARCH, "almacen": credenciales.Almacen(),
		"servicio": map[string]any{"direccion": s.Base(), "pid": os.Getpid(), "iniciado_en": s.IniciadoEn.UTC()}}
	if local {
		configuracion, _ := configuracion.RutaGlobal()
		datos["rutas"] = map[string]string{"datos": s.opc.Datos, "configuracion": configuracion,
			"registro": filepath.Join(s.opc.Datos, "servicio.log")}
	}
	return datos
}
