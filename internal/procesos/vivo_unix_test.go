//go:build !windows

package procesos

import "syscall"

func vivo(pid int) bool { return syscall.Kill(pid, 0) == nil }
