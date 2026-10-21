//go:build !windows

package tui

import (
	"os"
	"strings"
)

// consolaUTF8 dice si la consola pinta UTF-8, por el locale (LC_ALL, LC_CTYPE o LANG, el primero que haya). Sin
// ninguno (pasa por SSH), se supone que sí.
func consolaUTF8() bool {
	for _, v := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if s := strings.ToLower(os.Getenv(v)); s != "" {
			return strings.Contains(s, "utf-8") || strings.Contains(s, "utf8")
		}
	}
	return true
}
