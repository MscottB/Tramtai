package procesos

import "golang.org/x/sys/unix"

// limitarMemoria fija RLIMIT_AS del proceso recién lanzado; un fallo deja el proceso sin tope (no lo impide).
func limitarMemoria(pid, memoriaMB int) {
	limite := uint64(memoriaMB) << 20
	_ = unix.Prlimit(pid, unix.RLIMIT_AS, &unix.Rlimit{Cur: limite, Max: limite}, nil)
}
