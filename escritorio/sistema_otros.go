//go:build !windows

package main

// shortcut: fuera de Windows, iconos del menú de 16 px y tema claro; detectar el DPI y el tema del escritorio cuando haga falta.
func ladoIconoMenu() int { return 16 }

func menuOscuro() bool { return false }
