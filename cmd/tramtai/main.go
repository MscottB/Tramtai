// Comando tramtai: CLI del núcleo. En M1 ofrece versión y configuración; el resto de comandos llega con sus hitos.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"tramtai/internal/nucleo"
)

var version = "dev" // se inyecta con -ldflags desde scripts/compilar.ps1

func main() {
	raiz := &cobra.Command{Use: "tramtai", Short: "Pasarela local para ejecutar agentes CLI", SilenceUsage: true}
	raiz.AddCommand(
		&cobra.Command{Use: "version", Short: "Muestra la versión", Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "tramtai", version)
		}},
		comandoConfiguracion(),
	)
	if err := raiz.Execute(); err != nil {
		os.Exit(1)
	}
}

func comandoConfiguracion() *cobra.Command {
	configuracion := &cobra.Command{Use: "configuracion", Short: "Valida y muestra la configuración efectiva"}
	validar := &cobra.Command{Use: "validar", Short: "Valida la configuración global, del proyecto, local y de entorno",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := cargarConfiguracion(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "configuración válida")
			return nil
		}}
	var efectiva bool
	mostrar := &cobra.Command{Use: "mostrar", Short: "Muestra cada valor efectivo con su origen",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := cargarConfiguracion()
			if err != nil {
				return err
			}
			for _, e := range c.Efectiva() {
				fmt.Fprintf(cmd.OutOrStdout(), "%-46s %-14v %s\n", e.Clave, e.Valor, e.Origen)
			}
			return nil
		}}
	mostrar.Flags().BoolVar(&efectiva, "efectiva", true, "vista efectiva con origen por propiedad (única vista disponible)")
	explicar := &cobra.Command{Use: "explicar <clave>", Short: "Muestra el valor efectivo de una clave y su origen", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cargarConfiguracion()
			if err != nil {
				return err
			}
			for _, e := range c.Efectiva() {
				if e.Clave == args[0] {
					fmt.Fprintf(cmd.OutOrStdout(), "%s = %v (origen: %s)\n", e.Clave, e.Valor, e.Origen)
					return nil
				}
			}
			return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, "la clave no tiene valor: "+args[0])
		}}
	configuracion.AddCommand(validar, mostrar, explicar)
	return configuracion
}

// cargarConfiguracion resuelve las capas de la sección 12.2 para el directorio actual.
func cargarConfiguracion() (*nucleo.Configuracion, error) {
	dirUsuario, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	fuentes := []struct {
		ruta   string
		origen nucleo.Origen
	}{
		{filepath.Join(dirUsuario, "tramtai", "config.yaml"), nucleo.OrigenGlobal},
		{filepath.Join(".tramtai", "config.yaml"), nucleo.OrigenProyecto},
		{filepath.Join(".tramtai", "local.yaml"), nucleo.OrigenLocal},
	}
	var capas []nucleo.Capa
	for _, f := range fuentes {
		capa, err := nucleo.CargarCapa(f.ruta, f.origen)
		if err != nil {
			return nil, err
		}
		capas = append(capas, capa)
	}
	entorno, err := nucleo.CapaEntorno(os.Environ())
	if err != nil {
		return nil, err
	}
	return nucleo.Resolver(append(capas, entorno)...), nil
}
