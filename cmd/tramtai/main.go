// Comando tramtai: CLI y terminal interactiva de Tramtai. Sin argumentos abre la terminal interactiva.
package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/version"
)

func main() {
	raiz := &cobra.Command{Use: "tramtai", Short: idioma.T("Pasarela local para ejecutar agentes CLI"),
		Long: idioma.T("Tramtai expone Claude Code, Codex y Gemini como una API local.\n\n" +
			"  tramtai          terminal interactiva\n" +
			"  tramtai abrir    Web UI en el navegador (inicia el servicio si hace falta)"),
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
	if !idioma.Ingles() {
		ayudaEnEspanol(raiz)
	}
	if err := raiz.Execute(); err != nil {
		os.Exit(1)
	}
}

// plantillaUso es la de cobra con los títulos en español; [opciones] sustituye al [flags] de la línea de uso.
const plantillaUso = `Uso:{{if .Runnable}}
  {{.UseLine}}{{if .HasAvailableFlags}} [opciones]{{end}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [comando]{{end}}{{if gt (len .Aliases) 0}}

Alias:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Ejemplos:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Comandos disponibles:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Comandos adicionales:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Opciones:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Opciones globales:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Temas de ayuda adicionales:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Usa "{{.CommandPath}} [comando] --help" para ver más información de un comando.{{end}}
`

// ayudaEnEspanol cambia los textos propios de cobra (en inglés) por su versión en español: la plantilla de uso, los
// comandos help y completion y las opciones --help y --version, que cobra crea en cada comando.
func ayudaEnEspanol(raiz *cobra.Command) {
	raiz.SetUsageTemplate(plantillaUso)
	raiz.SetVersionTemplate(`{{with .DisplayName}}{{printf "%s " .}}{{end}}{{printf "versión %s" .Version}}` + "\n")
	raiz.InitDefaultVersionFlag()
	raiz.Flags().Lookup("version").Usage = "muestra la versión de " + raiz.DisplayName()
	raiz.InitDefaultHelpCmd()
	raiz.InitDefaultCompletionCmd()
	var recorrer func(*cobra.Command)
	recorrer = func(c *cobra.Command) {
		c.DisableFlagsInUseLine = true // la plantilla añade [opciones]
		c.InitDefaultHelpFlag()
		c.Flags().Lookup("help").Usage = "muestra la ayuda de " + c.DisplayName()
		switch {
		case c.Name() == "help" && c.Parent() == raiz:
			c.Use, c.Short = "help [comando]", "Muestra la ayuda de cualquier comando"
			c.Long = "Muestra la ayuda de cualquier comando.\nEscribe " + raiz.DisplayName() + " help [comando] para ver todos los detalles."
		case c.Name() == "completion" && c.Parent() == raiz:
			c.Short = "Genera el script de autocompletado para el intérprete de órdenes indicado"
			c.Long = "Genera el script de autocompletado de " + raiz.DisplayName() + " para el intérprete de órdenes indicado.\n" +
				"La ayuda de cada subcomando explica cómo usar el script generado.\n"
		case c.Parent() != nil && c.Parent().Name() == "completion":
			c.Short = "Genera el script de autocompletado para " + c.Name()
		}
		for _, hijo := range c.Commands() {
			recorrer(hijo)
		}
	}
	recorrer(raiz)
}

func comandoConfiguracion() *cobra.Command {
	configuracion := &cobra.Command{Use: "configuracion", Short: idioma.T("Valida y muestra la configuración efectiva")}
	validar := &cobra.Command{Use: "validar", Short: idioma.T("Valida la configuración global, del proyecto, local y de entorno"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := cargarConfiguracion(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), idioma.T("configuración válida"))
			return nil
		}}
	var efectiva bool
	mostrar := &cobra.Command{Use: "mostrar", Short: idioma.T("Muestra cada valor efectivo con su origen"),
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
	mostrar.Flags().BoolVar(&efectiva, "efectiva", true, idioma.T("vista efectiva con origen por propiedad (única vista disponible)"))
	explicar := &cobra.Command{Use: "explicar <clave>", Short: idioma.T("Muestra el valor efectivo de una clave y su origen"), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cargarConfiguracion()
			if err != nil {
				return err
			}
			for _, e := range c.Efectiva() {
				if e.Clave == args[0] {
					fmt.Fprintf(cmd.OutOrStdout(), idioma.T("%s = %v (origen: %s)\n"), e.Clave, e.Valor, e.Origen)
					return nil
				}
			}
			return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("la clave no tiene valor: %s", args[0]))
		}}
	configuracion.AddCommand(validar, mostrar, explicar)
	return configuracion
}

// raizProyecto es la raíz del proyecto que contiene el directorio actual, o "" fuera de un proyecto.
func comandoVersion() *cobra.Command {
	var detalle bool
	cmd := &cobra.Command{Use: "version", Short: idioma.T("Muestra la versión"), Args: cobra.NoArgs, Run: func(cmd *cobra.Command, _ []string) {
		texto := version.Texto()
		if detalle {
			texto = version.Detalle()
		}
		fmt.Fprintln(cmd.OutOrStdout(), "tramtai", texto)
	}}
	cmd.Flags().BoolVar(&detalle, "detalle", false, idioma.T("incluye el commit compilado"))
	return cmd
}

// comandoAbrir es el atajo para usar la Web UI: elige la instancia del proyecto actual (o la global), inicia el
// servicio si hace falta y abre el navegador ya dentro.
func comandoAbrir() *cobra.Command {
	var enNavegador bool
	abrir := &cobra.Command{Use: "abrir", Short: idioma.T("Abre Tramtai en la app de escritorio o en el navegador (inicia el servicio si hace falta)"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ambito := "global"
			if raizProyecto() != "" {
				ambito = "proyecto"
			}
			return abrirWebUI(cmd, ambito, enNavegador)
		}}
	abrir.Flags().BoolVar(&enNavegador, "navegador", false, idioma.T("abre la Web UI en el navegador en lugar de la app de escritorio"))
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
