package carpetas

import (
	"errors"
	"io/fs"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"

	"tramtai/internal/idioma"
)

// protegerCarpeta deja la carpeta solo para el usuario actual y SYSTEM, sin heredar permisos de la de encima: dentro está
// el token que administra el servicio. Solo se aplica si aún no está protegida, porque una DACL se propaga a todo lo de
// dentro. En un volumen sin ACL (FAT o exFAT) no hay nada que poner y no es un error.
func protegerCarpeta(ruta string) error {
	if actual, err := windows.GetNamedSecurityInfo(ruta, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION); err == nil {
		if control, _, err := actual.Control(); err == nil && control&windows.SE_DACL_PROTECTED != 0 {
			return nil
		}
	}
	usuario, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + usuario.User.Sid.String() + ")(A;OICI;FA;;;SY)")
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	err = windows.SetNamedSecurityInfo(ruta, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
	if err != nil && !conACL(ruta) {
		return nil
	}
	return err
}

// ProtegerServidor deja la carpeta del servidor solo para SYSTEM, los administradores y la cuenta del servicio
// (NT SERVICE\Tramtai), con control total heredable, sin heredar de %ProgramData% (que deja a los usuarios crear
// archivos) y con dueño Administradores. La cuenta virtual solo se resuelve con el servicio ya creado.
//
// Si la carpeta no existe, nace ya con ese descriptor: no hay un momento en que otro usuario pueda entrar. Si existía,
// tiene que ser de Administradores o de SYSTEM: la de otro usuario pudo dejar dentro archivos con permisos propios
// (config.yaml, datos\administrador) que cambiar la raíz no quita, y es un error. Si además no estaba protegida (no la
// dejó una instalación), tampoco puede haber dentro nada de otro dueño. Se mira y se cambia sobre la carpeta abierta
// sin seguir puntos de reanálisis: un punto de unión a otra carpeta es un error.
func ProtegerServidor(ruta, cuenta string) error {
	servicio, _, _, err := windows.LookupSID("", cuenta)
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("O:BAD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;" + servicio.String() + ")")
	if err != nil {
		return err
	}
	nombre, err := windows.UTF16PtrFromString(ruta)
	if err != nil {
		return err
	}
	atributos := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	if err := windows.CreateDirectory(nombre, &atributos); !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return err
	}
	h, info, err := abrirSinSeguir(ruta, windows.READ_CONTROL|windows.WRITE_DAC|windows.WRITE_OWNER)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return errors.New(idioma.Tf("%s no es una carpeta (¿un punto de unión o un enlace?): quítala y repite la instalación", ruta))
	}
	ajena := errors.New(idioma.Tf("%s la creó otro usuario: revísala y quítala antes de instalar", ruta))
	dueno, err := duenoDe(h)
	if err != nil {
		return err
	}
	if !dueno.IsWellKnown(windows.WinBuiltinAdministratorsSid) && !dueno.IsWellKnown(windows.WinLocalSystemSid) {
		return ajena
	}
	actual, err := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	control, _, err := actual.Control()
	if err != nil {
		return err
	}
	administradores, _, err := sd.Owner()
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if err := windows.SetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, administradores, nil, dacl, nil); err != nil {
		return err
	}
	if control&windows.SE_DACL_PROTECTED != 0 { // la dejó así una instalación: lo de dentro es nuestro
		return nil
	}
	// Sin proteger, la carpeta tenía los permisos de %ProgramData% (la creó, por ejemplo, una orden --servidor antes de
	// instalar) y un usuario pudo dejar algo suyo dentro. Se mira ya protegida, cuando nadie más puede crear nada.
	return filepath.WalkDir(ruta, func(p string, _ fs.DirEntry, err error) error {
		if err != nil || p == ruta {
			return err
		}
		hijo, info, err := abrirSinSeguir(p, windows.READ_CONTROL)
		if err != nil {
			return err
		}
		defer windows.CloseHandle(hijo)
		dueno, err := duenoDe(hijo)
		if err != nil {
			return err
		}
		propio := dueno.IsWellKnown(windows.WinBuiltinAdministratorsSid) || dueno.IsWellKnown(windows.WinLocalSystemSid) || dueno.Equals(servicio)
		if !propio || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 { // una unión: no se sigue
			return ajena
		}
		return nil
	})
}

// abrirSinSeguir abre un archivo o una carpeta sin seguir puntos de reanálisis (uniones, enlaces) y da sus atributos.
func abrirSinSeguir(ruta string, acceso uint32) (windows.Handle, windows.ByHandleFileInformation, error) {
	var info windows.ByHandleFileInformation
	nombre, err := windows.UTF16PtrFromString(ruta)
	if err != nil {
		return 0, info, err
	}
	h, err := windows.CreateFile(nombre, acceso, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return 0, info, err
	}
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		windows.CloseHandle(h)
		return 0, info, err
	}
	return h, info, nil
}

// duenoDe lee el dueño de una carpeta abierta; las pruebas lo sustituyen (darle otro dueño exige privilegios).
var duenoDe = func(h windows.Handle) (*windows.SID, error) {
	sd, err := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return nil, err
	}
	dueno, _, err := sd.Owner()
	return dueno, err
}

// conACL dice si el volumen de ruta guarda permisos (NTFS sí; FAT y exFAT no). GetVolumePathName da el volumen real
// también si está montado en una carpeta (C:\mnt\usb).
func conACL(ruta string) bool {
	p, err := windows.UTF16PtrFromString(ruta)
	if err != nil {
		return true
	}
	volumen := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(p, &volumen[0], uint32(len(volumen))); err != nil {
		return true
	}
	var marcas uint32
	if err := windows.GetVolumeInformation(&volumen[0], nil, 0, nil, nil, &marcas, nil, 0); err != nil {
		return true
	}
	return marcas&windows.FILE_PERSISTENT_ACLS != 0
}
