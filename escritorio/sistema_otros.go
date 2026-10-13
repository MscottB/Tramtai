//go:build !windows

package main

// shortcut: tamaño fijo y tema claro fuera de Windows, mientras la app de escritorio sea solo para Windows.
func ladoIconoMenu() int { return 16 }

func menuOscuro() bool { return false }
