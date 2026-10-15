//go:build !windows

package idioma

import "os"

// delSistema sigue la precedencia de POSIX (LC_ALL, LC_MESSAGES y LANG), por ejemplo "es_CL.UTF-8".
func delSistema() string {
	for _, variable := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(variable); v != "" {
			return v
		}
	}
	return ""
}
