package main

import (
	"errors"

	"github.com/spf13/cobra"

	"tramtai/internal/idioma"
	"tramtai/internal/instalacion"
)

// comandoInstalacion son las tareas del instalador de Windows (scripts/instalador/tramtai.nsi); no aparece en la ayuda.
func comandoInstalacion() *cobra.Command {
	cmd := &cobra.Command{Use: "instalacion", Short: idioma.T("Tareas del instalador de Windows"), Hidden: true}
	for _, accion := range []string{"agregar-ruta", "quitar-ruta"} {
		agregar := accion == "agregar-ruta"
		cmd.AddCommand(&cobra.Command{Use: accion + " <carpeta>", Short: idioma.T("Añade o quita la carpeta del PATH del usuario"),
			Args: cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error { return instalacion.EditarRutaUsuario(args[0], agregar) }})
	}
	// abierto y cerrar miran solo los procesos de la carpeta de la instalación: el instalador no cierra una portable.
	cmd.AddCommand(&cobra.Command{Use: "abierto <carpeta> <exe>", Short: idioma.T("Sale con 0 si hay un proceso exe de la carpeta"),
		Args: cobra.ExactArgs(2), SilenceUsage: true, SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			pids, err := instalacion.ProcesosEn(args[0], args[1])
			if err != nil {
				return nil // sin poder mirar, se da por abierto: es lo seguro para el instalador
			}
			if len(pids) == 0 {
				return errors.New(idioma.T("no hay ninguno abierto"))
			}
			return nil
		}})
	cmd.AddCommand(&cobra.Command{Use: "cerrar <carpeta> <exe>", Short: idioma.T("Cierra los procesos exe de la carpeta"),
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error { return instalacion.Cerrar(args[0], args[1]) }})
	return cmd
}
