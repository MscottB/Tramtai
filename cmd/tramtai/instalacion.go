package main

import (
	"github.com/spf13/cobra"

	"tramtai/internal/idioma"
	"tramtai/internal/instalacion"
)

// comandoInstalacion lo usa el instalador de Windows (scripts/instalador/tramtai.nsi); no aparece en la ayuda.
func comandoInstalacion() *cobra.Command {
	cmd := &cobra.Command{Use: "instalacion", Short: idioma.T("Tareas del instalador de Windows"), Hidden: true}
	for _, accion := range []string{"agregar-ruta", "quitar-ruta"} {
		agregar := accion == "agregar-ruta"
		cmd.AddCommand(&cobra.Command{Use: accion + " <carpeta>", Short: idioma.T("Añade o quita la carpeta del PATH del usuario"),
			Args: cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error { return instalacion.EditarRutaUsuario(args[0], agregar) }})
	}
	return cmd
}
