package servicio

import (
	"os"
	"path/filepath"
	"runtime"

	"tramtai/internal/carpetas"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/credenciales"
	"tramtai/internal/version"
)

// acerca son los datos de «Acerca de Tramtai»: versión, compilación, sistema y servicio. Las rutas de carpetas solo van
// a una sesión local, porque llevan el nombre del usuario del sistema. Nunca incluye claves ni tokens.
func (s *Servicio) acerca(local bool) contrato.Acerca {
	commit, modificado := version.Commit()
	datos := contrato.Acerca{Version: version.Texto(), Commit: commit, Modificado: modificado, Go: runtime.Version(),
		Portable: carpetas.Portable(), Sistema: runtime.GOOS, Arquitectura: runtime.GOARCH, Almacen: credenciales.Almacen(),
		Servicio: contrato.ServicioAcerca{Direccion: s.Base(), PID: os.Getpid(), IniciadoEn: s.IniciadoEn.UTC()}}
	if local {
		configuracion, _ := configuracion.RutaGlobal()
		datos.Rutas = &contrato.RutasAcerca{Datos: s.opc.Datos, Configuracion: configuracion, Registro: filepath.Join(s.opc.Datos, "servicio.log")}
	}
	return datos
}
