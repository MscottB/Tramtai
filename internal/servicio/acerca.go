package servicio

import (
	"cmp"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"tramtai/internal/carpetas"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/credenciales"
	"tramtai/internal/version"
)

// entorno es servidor, portable o escritorio: el de Opciones o, sin él, el del proceso.
func (s *Servicio) entorno() string { return cmp.Or(s.opc.Entorno, carpetas.Entorno()) }

// acerca son los datos de «Acerca de Tramtai»: versión, compilación, sistema, servicio, entorno y, en el servidor, su
// cuenta. Las rutas de carpetas solo van a una sesión local, porque llevan el nombre del usuario del sistema (y la sesión
// remota del servidor no ve sus carpetas). Nunca incluye claves ni tokens.
func (s *Servicio) acerca(local bool) contrato.Acerca {
	commit, modificado := version.Commit()
	datos := contrato.Acerca{Version: version.Texto(), Commit: commit, Modificado: modificado, Go: runtime.Version(),
		Portable: carpetas.Portable(), Entorno: s.entorno(), Sistema: runtime.GOOS, Arquitectura: runtime.GOARCH, Almacen: credenciales.Almacen(),
		Servicio: contrato.ServicioAcerca{Direccion: s.BasePublica(), PID: os.Getpid(), IniciadoEn: s.IniciadoEn.UTC()}}
	if datos.Entorno == "servidor" {
		if u, err := user.Current(); err == nil {
			datos.Cuenta = u.Username
		}
	}
	if local {
		configuracion, _ := configuracion.RutaGlobal()
		datos.Rutas = &contrato.RutasAcerca{Datos: s.opc.Datos, Configuracion: configuracion, Registro: filepath.Join(s.opc.Datos, "servicio.log")}
	}
	return datos
}
