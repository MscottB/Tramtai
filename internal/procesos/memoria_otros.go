//go:build !windows && !linux

package procesos

// limitarMemoria no hace nada fuera de Linux y Windows: macOS no permite fijar el tope a otro proceso.
// shortcut: sin tope en macOS; si hiciera falta, lanzar el hijo a través de un ayudante que fije setrlimit.
func limitarMemoria(int, int) {}
