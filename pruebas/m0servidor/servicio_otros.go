//go:build m0servidor && !windows

package main

// En Linux la sonda es una unidad de systemd normal: no hay protocolo de servicio que hablar.
func enServicio() bool { return false }

func ejecutarServicio(string, string) error { return nil }
