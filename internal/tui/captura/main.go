// captura pinta una página de la terminal para las capturas de la portada y escribe su salida ANSI en stdout:
//
//	captura <claro|oscuro> <es|en> <ancho> <alto> [teclas…]
//
// Lo compila y lo ejecuta web/scripts/capturas.mjs junto a la copia portable de prueba (portable.txt), así que habla con
// el servicio de esa copia, con sus datos de ejemplo, y nunca con el Tramtai de quien saca las capturas.
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"tramtai/internal/carpetas"
	"tramtai/internal/cliente"
	"tramtai/internal/tui"
)

func main() {
	if err := capturar(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "captura:", err)
		os.Exit(1)
	}
}

func capturar(args []string) error {
	if len(args) < 4 || (args[0] != "claro" && args[0] != "oscuro") || (args[1] != "es" && args[1] != "en") {
		return fmt.Errorf("uso: captura <claro|oscuro> <es|en> <ancho> <alto> [teclas…]")
	}
	ancho, err1 := strconv.Atoi(args[2])
	alto, err2 := strconv.Atoi(args[3])
	if err1 != nil || err2 != nil {
		return fmt.Errorf("el ancho y el alto son números: %q %q", args[2], args[3])
	}
	if !carpetas.Portable() {
		return fmt.Errorf("solo junto a una copia portable de prueba (portable.txt): así no toca el Tramtai de nadie")
	}
	datos, err := carpetas.DirDatos()
	if err != nil {
		return err
	}
	cli, err := cliente.Conectar(datos)
	if err != nil {
		return err
	}
	// En la portada se enseña la marca de quien usa Tramtai en su equipo, no la de la copia portable de prueba.
	o := tui.Opciones{Datos: datos, Entorno: "escritorio"}
	salida, err := tui.Captura(o, cli, args[1], args[0] == "oscuro", ancho, alto, args[4:], 20*time.Second)
	if err != nil {
		return err
	}
	_, err = fmt.Print(salida)
	return err
}
