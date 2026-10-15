// Comando tramtai: CLI y terminal interactiva de Tramtai. Sin argumentos abre la terminal interactiva.
package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"tramtai/internal/nucleo"
	"tramtai/internal/version"
)

func main() {
	raiz := &cobra.Command{Use: "tramtai", Short: "Pasarela local para ejecutar agentes CLI",
		Long: "Tramtai expone Claude Code, Codex y Gemini como una API local.\n\n" +
			"  tramtai          terminal interactiva\n" +
			"  tramtai abrir    Web UI en el navegador (inicia el servicio si hace falta)",
		SilenceUsage: true, Version: version.Texto(),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
				return cmd.Help()
			}
			return abrirTerminal()
		}}
	raiz.AddCommand(
		comandoVersion(),
		comandoAbrir(),
		comandoTerminal(),
		comandoIniciar(),
		comandoServicio(),
		comandoInstancia("global"),
		comandoInstancia("proyecto"),
		comandoAcceso(),
		comandoTrabajos(),
		comandoEjecutar(),
		comandoConvertir(),
		comandoAgentes(),
		comandoConfiguracion(),
		comandoClaves(),
		comandoInstalacion(),
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

// raizProyecto es la raíz del proyecto que contiene el directorio actual, o "" fuera de un proyecto.
func comandoVersion() *cobra.Command {
	var detalle bool
	cmd := &cobra.Command{Use: "version", Short: "Muestra la versión", Args: cobra.NoArgs, Run: func(cmd *cobra.Command, _ []string) {
		texto := version.Texto()
		if detalle {
			texto = version.Detalle()
		}
		fmt.Fprintln(cmd.OutOrStdout(), "tramtai", texto)
	}}
	cmd.Flags().BoolVar(&detalle, "detalle", false, "incluye el commit compilado")
	return cmd
}

// comandoAbrir es el atajo para usar la Web UI: elige la instancia del proyecto actual (o la global), inicia el
// servicio si hace falta y abre el navegador ya dentro.
func comandoAbrir() *cobra.Command {
	var enNavegador bool
	abrir := &cobra.Command{Use: "abrir", Short: "Abre Tramtai en la app de escritorio o en el navegador (inicia el servicio si hace falta)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ambito := "global"
			if raizProyecto() != "" {
				ambito = "proyecto"
			}
			return abrirWebUI(cmd, ambito, enNavegador)
		}}
	abrir.Flags().BoolVar(&enNavegador, "navegador", false, "abre la Web UI en el navegador en lugar de la app de escritorio")
	return abrir
}

func raizProyecto() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	raiz, _ := nucleo.BuscarProyecto(dir)
	return raiz
}

// cargarConfiguracion resuelve las capas de la sección 12.2 para el proyecto actual (o solo la global).
func cargarConfiguracion() (*nucleo.Configuracion, error) {
	return nucleo.CargarConfiguracion(raizProyecto())
}
