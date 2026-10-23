//go:build !windows

package servicio

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func bloquear(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return errOcupado
	}
	return err
}
