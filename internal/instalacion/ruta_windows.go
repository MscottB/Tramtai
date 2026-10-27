//go:build windows

package instalacion

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// EditarRutaUsuario añade o quita dir del PATH del usuario (HKCU\Environment), conservando el tipo del valor, y avisa
// a Windows para que las terminales que se abran después lo vean sin cerrar sesión.
func EditarRutaUsuario(dir string, agregar bool) error {
	return editarRutaEn(registry.CURRENT_USER, "Environment", dir, agregar)
}

// EditarRutaSistema es EditarRutaUsuario con el PATH del sistema (HKLM): el de «tramtai servidor instalar», que deja
// tramtai-servidor a mano en cualquier consola. Necesita ser administrador.
func EditarRutaSistema(dir string, agregar bool) error {
	return editarRutaEn(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, dir, agregar)
}

func editarRutaEn(raiz registry.Key, clave, dir string, agregar bool) error {
	k, err := registry.OpenKey(raiz, clave, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	// GetStringValue no expande REG_EXPAND_SZ: las entradas con %VARIABLES% se escriben de vuelta igual.
	actual, tipo, err := k.GetStringValue("Path")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	nueva := editarRuta(actual, dir, agregar)
	if nueva == actual {
		return nil
	}
	if tipo == registry.SZ {
		err = k.SetStringValue("Path", nueva)
	} else {
		err = k.SetExpandStringValue("Path", nueva)
	}
	if err != nil {
		return err
	}
	avisarCambioEntorno()
	return nil
}

// avisarCambioEntorno difunde WM_SETTINGCHANGE("Environment"), como hace el panel de variables de entorno.
func avisarCambioEntorno() {
	texto, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	const hwndBroadcast, wmSettingChange, smtoAbortIfHung = 0xffff, 0x001a, 0x0002
	var resultado uintptr
	windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW").Call(hwndBroadcast, wmSettingChange, 0,
		uintptr(unsafe.Pointer(texto)), smtoAbortIfHung, 5000, uintptr(unsafe.Pointer(&resultado)))
}
