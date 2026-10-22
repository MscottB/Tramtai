package carpetas

import (
	"golang.org/x/sys/windows"
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
