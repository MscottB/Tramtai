package nucleo

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// protegerCarpeta deja la carpeta solo para el usuario actual y SYSTEM, sin heredar permisos de la carpeta de encima:
// una portable en C:\Tramtai heredaría «Usuarios autenticados: Modificar», y dentro está el token que administra el
// servicio. En un volumen sin ACL (FAT o exFAT de un pendrive) no hay nada que poner y no es un error.
func protegerCarpeta(ruta string) error {
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

// conACL dice si el volumen de ruta guarda permisos (NTFS sí; FAT y exFAT no).
func conACL(ruta string) bool {
	volumen, err := windows.UTF16PtrFromString(filepath.VolumeName(ruta) + `\`)
	if err != nil {
		return true
	}
	var marcas uint32
	if err := windows.GetVolumeInformation(volumen, nil, 0, nil, nil, &marcas, nil, 0); err != nil {
		return true
	}
	return marcas&windows.FILE_PERSISTENT_ACLS != 0
}
