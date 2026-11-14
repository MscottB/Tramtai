package preparar

import (
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// pathUsuario son las carpetas del Path de HKCU\Environment: donde los instaladores dejan el CLI para el usuario.
func pathUsuario() []string {
	k, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	valor, tipo, err := k.GetStringValue("Path")
	if err != nil {
		return nil
	}
	if tipo == registry.EXPAND_SZ {
		if expandido, err := registry.ExpandString(valor); err == nil {
			valor = expandido
		}
	}
	return filepath.SplitList(valor)
}
