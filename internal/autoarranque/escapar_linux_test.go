package autoarranque

import "testing"

// Una ruta con espacios, comillas, $, ` y % llega entera al Exec de la entrada de arranque.
func TestEscaparExec(t *testing.T) {
	casos := map[string]string{
		"/opt/tramtai/tramtai-escritorio":  `"/opt/tramtai/tramtai-escritorio"`,
		"/home/ana maría/100% Tramtai/app": `"/home/ana maría/100%% Tramtai/app"`,
		`/home/a"b/$x/` + "`y`":            `"/home/a\\"b/\\$x/\\` + "`" + `y\\` + "`" + `"`,
		`/home/a\b`:                        `"/home/a\\\\b"`,
	}
	for ruta, esperado := range casos {
		if r := escaparExec(ruta); r != esperado {
			t.Errorf("escaparExec(%q) = %s; se esperaba %s", ruta, r, esperado)
		}
	}
}
