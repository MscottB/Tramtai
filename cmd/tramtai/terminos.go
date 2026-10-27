package main

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"tramtai/internal/carpetas"
	"tramtai/internal/idioma"
	"tramtai/internal/terminos"
)

// dirDatos es la carpeta de datos de este entorno; las pruebas la cambian por una temporal.
var dirDatos = carpetas.DirDatos

func comandoTerminos() *cobra.Command {
	cmd := &cobra.Command{Use: "terminos", Aliases: []string{"terms"}, Short: idioma.T("Muestra los términos de uso de Tramtai"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), terminos.Texto)
			return err
		}}
	var como string
	aceptar := &cobra.Command{Use: "aceptar", Aliases: []string{"accept"}, Short: idioma.T("Acepta los términos de uso en este equipo"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			datos, err := dirDatos()
			if err != nil {
				return err
			}
			if err := terminos.Aceptar(datos, como); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("términos de uso aceptados (versión %s)\n"), terminos.Version)
			return nil
		}}
	aceptar.Flags().StringVar(&como, "como", terminos.ComoCLI, idioma.T("cómo se aceptan (lo usa el instalador)"))
	_ = aceptar.Flags().MarkHidden("como")
	cmd.AddCommand(aceptar)
	return cmd
}

// sinAvisoDeTerminos son las órdenes que no avisan de que faltan los términos: las que los tratan, las de ayuda y
// autocompletado, la terminal (la raíz sin argumentos también), que los pide en su propia pantalla, y «servidor», cuyo
// «instalar» los pide (y avisar crearía datos/ a nombre de root antes de la instalación).
var sinAvisoDeTerminos = []string{"terminos", "terminal", "servidor", "help", "completion", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd}

// sinAviso dice si la orden no avisa: las de sinAvisoDeTerminos (y las que cuelgan de ellas), la raíz sola y
// «servicio iniciar», que se lanza en segundo plano con su stderr en servicio-salida.log y lo ensuciaría en cada arranque.
func sinAviso(cmd *cobra.Command) bool {
	if cmd.Parent() == nil || cmd.Name() == "iniciar" && cmd.Parent().Name() == "servicio" {
		return true
	}
	for c := cmd; c != nil; c = c.Parent() {
		if slices.Contains(sinAvisoDeTerminos, c.Name()) {
			return true
		}
	}
	return false
}

// avisarTerminos escribe en stderr una línea si este entorno no ha aceptado los términos vigentes. La CLI no se bloquea:
// no rompe los scripts de quien actualiza, y la salida normal queda como estaba.
func avisarTerminos(cmd *cobra.Command, _ []string) {
	if sinAviso(cmd) {
		return
	}
	datos, err := dirDatos()
	if err != nil {
		return
	}
	if pendiente, _ := terminos.Pendiente(datos); pendiente {
		fmt.Fprintln(cmd.ErrOrStderr(), idioma.Tf("aviso: aún no has aceptado los términos de uso de Tramtai (versión %s): léelos con «tramtai terminos» y acéptalos con «tramtai terminos aceptar»", terminos.Version))
	}
}
