//go:build !windows

package main

import "github.com/spf13/cobra"

// ejecutarComoServicio solo hace algo en Windows: en Linux el servicio es una unidad systemd normal.
func ejecutarComoServicio(*cobra.Command, string) (bool, error) { return false, nil }

// comoServicioWindows solo es cierto dentro del servicio de Windows.
func comoServicioWindows() bool { return false }
