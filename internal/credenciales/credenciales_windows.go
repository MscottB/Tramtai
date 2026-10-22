//go:build windows

package credenciales

import (
	"errors"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"tramtai/internal/idioma"
)

// Administrador de credenciales de Windows (credenciales genéricas del usuario, persistencia local).
var (
	advapi32   = windows.NewLazySystemDLL("advapi32.dll")
	credWrite  = advapi32.NewProc("CredWriteW")
	credRead   = advapi32.NewProc("CredReadW")
	credDelete = advapi32.NewProc("CredDeleteW")
	credFree   = advapi32.NewProc("CredFree")
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
	errorNoEncontrada       = syscall.Errno(1168) // ERROR_NOT_FOUND
	maximoBytesCredencial   = 5 * 512             // CRED_MAX_CREDENTIAL_BLOB_SIZE
)

// credencial es CREDENTIALW.
type credencial struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// Disponible indica si el sistema tiene almacén de credenciales soportado.
func Disponible() bool { return true }

// Almacen es el nombre del almacén, para las instrucciones de la interfaz.
func Almacen() string { return idioma.T("Administrador de credenciales de Windows") }

// Guardar crea o sustituye la clave.
func Guardar(nombre, valor string) error {
	if err := admitir(nombre); err != nil {
		return err
	}
	if err := valorValido(valor); err != nil {
		return err
	}
	if len(valor) > maximoBytesCredencial {
		return errors.New(idioma.T("la clave está vacía o es demasiado larga"))
	}
	destino, _ := windows.UTF16PtrFromString(objetivo(nombre))
	comentario, _ := windows.UTF16PtrFromString("Clave de API usada por Tramtai")
	usuario, _ := windows.UTF16PtrFromString("tramtai")
	datos := []byte(valor)
	c := credencial{Type: credTypeGeneric, TargetName: destino, Comment: comentario, CredentialBlobSize: uint32(len(datos)),
		CredentialBlob: &datos[0], Persist: credPersistLocalMachine, UserName: usuario}
	if r, _, err := credWrite.Call(uintptr(unsafe.Pointer(&c)), 0); r == 0 {
		return err
	}
	return nil
}

// Leer devuelve la clave, o "" si no está guardada.
func Leer(nombre string) (string, error) {
	destino, _ := windows.UTF16PtrFromString(objetivo(nombre))
	var p *credencial
	r, _, err := credRead.Call(uintptr(unsafe.Pointer(destino)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&p)))
	if r == 0 {
		if errors.Is(err, errorNoEncontrada) {
			return "", nil
		}
		return "", err
	}
	defer credFree.Call(uintptr(unsafe.Pointer(p)))
	return string(unsafe.Slice(p.CredentialBlob, p.CredentialBlobSize)), nil
}

// Borrar elimina la clave; no es un error si no existía.
func Borrar(nombre string) error {
	if err := admitir(nombre); err != nil {
		return err
	}
	destino, _ := windows.UTF16PtrFromString(objetivo(nombre))
	if r, _, err := credDelete.Call(uintptr(unsafe.Pointer(destino)), credTypeGeneric, 0); r == 0 && !errors.Is(err, errorNoEncontrada) {
		return err
	}
	return nil
}
