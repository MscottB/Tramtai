package main

import (
	"os"

	"github.com/spf13/cobra"

	"tramtai/internal/cliente"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/servicio"
	"tramtai/internal/tui"
)

func comandoTerminal() *cobra.Command {
	return &cobra.Command{Use: "terminal", Short: idioma.T("Abre la terminal interactiva (también: tramtai sin argumentos)"),
		RunE: func(*cobra.Command, []string) error { return abrirTerminal() }}
}

func abrirTerminal() error {
	datos, err := nucleo.DirDatos()
	if err != nil {
		return err
	}
	o := tui.Opciones{Datos: datos, Iniciar: func() (*cliente.Cliente, error) {
		ejecutable, err := os.Executable()
		if err != nil {
			return nil, err
		}
		c, err := cliente.IniciarEnSegundoPlano(ejecutable, datos)
		if c != nil {
			c.Origen = nucleo.DesdeTerminal
		}
		return c, err
	}}
	if raiz := raizProyecto(); raiz != "" {
		if cfg, err := nucleo.CargarConfiguracion(raiz); err == nil {
			o.Proyecto, o.IDProyecto = raiz, servicio.IDDeCarpeta(datos, raiz, cfg)
		}
	}
	return tui.Ejecutar(o)
}
