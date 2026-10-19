//go:build !windows && !linux

package procesos

// conTope no cambia la orden fuera de Linux y Windows: macOS no hace cumplir RLIMIT_AS.
// shortcut: sin tope de memoria en macOS; si hiciera falta, vigilar el tamaño del proceso desde Tramtai.
func conTope(programa string, args []string, _ int) (string, []string) { return programa, args }
