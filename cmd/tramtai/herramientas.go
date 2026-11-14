package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/credenciales"
	"tramtai/internal/idioma"
	"tramtai/internal/servicio"
)

// servicioHerramientas conecta con el servicio (o lo inicia); las pruebas lo sustituyen. «clave» no lo usa.
var servicioHerramientas = asegurarServicio

// comandoHerramientas apunta a Tramtai las herramientas de terceros (Claude Code, Codex, Continue, opencode, Kilo): lo
// que hay que cambiar en sus archivos lo hace el servicio; esto solo lo pide y cuenta cómo quedó.
func comandoHerramientas() *cobra.Command {
	grupo := &cobra.Command{Use: "herramientas", Aliases: []string{"tools"}, Short: idioma.T("Apunta a Tramtai herramientas como Claude Code o Codex, y deshace el cambio")}
	var global bool
	grupo.PersistentFlags().BoolVar(&global, "global", false, idioma.T("usa el espacio global aunque estés dentro de un proyecto"))
	// con conecta con el servicio, elige el espacio y le pasa la orden.
	con := func(f func(cmd *cobra.Command, c *cliente.Cliente, ctx context.Context, inst string, args []string) error) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			c, err := servicioHerramientas(cmd)
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			inst, err := instanciaActual(ctx, c, "", global)
			if err != nil {
				return err
			}
			return f(cmd, c, ctx, inst, args)
		}
	}
	// idsOTodas exige una herramienta o --todas, y no las dos cosas.
	idsOTodas := func(todas *bool) cobra.PositionalArgs {
		return func(_ *cobra.Command, args []string) error {
			if (len(args) > 0) == *todas { // ninguna de las dos, o las dos
				return errors.New(idioma.T("indica una o más herramientas, o --todas"))
			}
			return nil
		}
	}

	listar := &cobra.Command{Use: "listar", Aliases: []string{"list"}, Short: idioma.T("Lista las herramientas y si apuntan a Tramtai"), Args: cobra.NoArgs,
		RunE: con(func(cmd *cobra.Command, c *cliente.Cliente, ctx context.Context, inst string, _ []string) error {
			lista, err := c.Herramientas(ctx, inst)
			if err != nil {
				return err
			}
			escribirHerramientas(cmd.OutOrStdout(), lista)
			return nil
		})}

	var activarTodas bool
	var modelo string
	activar := &cobra.Command{Use: "activar <herramienta>… | --todas", Aliases: []string{"enable"}, Short: idioma.T("Apunta las herramientas a Tramtai (guarda una copia de su archivo antes del primer cambio)"),
		Args: idsOTodas(&activarTodas),
		RunE: con(func(cmd *cobra.Command, c *cliente.Cliente, ctx context.Context, inst string, args []string) error {
			lista, err := c.ActivarHerramientas(ctx, inst, contrato.ActivarHerramientas{IDs: args, Todas: activarTodas, Modelo: modelo})
			if err != nil {
				return err
			}
			escribirHerramientas(cmd.OutOrStdout(), lista)
			return nil
		})}
	activar.Flags().BoolVar(&activarTodas, "todas", false, idioma.T("todas las herramientas detectadas"))
	activar.Flags().BoolVar(&activarTodas, "all", false, "")
	_ = activar.Flags().MarkHidden("all")
	activar.Flags().StringVar(&modelo, "modelo", "", idioma.T("agente/modelo que usarán (por defecto, el primer agente conectado)"))
	activar.Flags().StringVar(&modelo, "model", "", "")
	_ = activar.Flags().MarkHidden("model")

	var desactivarTodas bool
	desactivar := &cobra.Command{Use: "desactivar <herramienta>… | --todas", Aliases: []string{"disable"}, Short: idioma.T("Quita Tramtai de las herramientas y detiene sus APIs"),
		Args: idsOTodas(&desactivarTodas),
		RunE: con(func(cmd *cobra.Command, c *cliente.Cliente, ctx context.Context, inst string, args []string) error {
			lista, err := c.DesactivarHerramientas(ctx, inst, args, desactivarTodas)
			if err != nil {
				return err
			}
			escribirHerramientas(cmd.OutOrStdout(), lista)
			return nil
		})}
	desactivar.Flags().BoolVar(&desactivarTodas, "todas", false, idioma.T("todas las herramientas"))
	desactivar.Flags().BoolVar(&desactivarTodas, "all", false, "")
	_ = desactivar.Flags().MarkHidden("all")

	olvidar := &cobra.Command{Use: "olvidar <herramienta>", Aliases: []string{"forget"}, Short: idioma.T("Desactiva la herramienta y borra su clave y lo que Tramtai sabía de ella"), Args: cobra.ExactArgs(1),
		RunE: con(func(cmd *cobra.Command, c *cliente.Cliente, ctx context.Context, inst string, args []string) error {
			if err := c.OlvidarHerramienta(ctx, inst, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("%s olvidada: su clave y lo anotado se han borrado\n"), args[0])
			return nil
		})}

	var si bool
	restaurar := &cobra.Command{Use: "restaurar <herramienta>", Aliases: []string{"restore"}, Short: idioma.T("Devuelve el archivo de la herramienta a como estaba antes de activarla"), Args: cobra.ExactArgs(1),
		RunE: con(func(cmd *cobra.Command, c *cliente.Cliente, ctx context.Context, inst string, args []string) error {
			if !si && !confirmar(cmd, idioma.Tf("¿Devolver el archivo de %s a como estaba antes de activarla? Pierdes los cambios que hayas hecho en él desde entonces.", args[0])) {
				return nil
			}
			copia, err := c.RestaurarHerramienta(ctx, inst, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("%s restaurada desde la copia %s\n"), args[0], copia)
			return nil
		})}
	restaurar.Flags().BoolVarP(&si, "si", "s", false, idioma.T("no pedir confirmación"))

	// clave la ejecutan Claude Code (apiKeyHelper) y Codex (auth.command), este cada 300 s: lee el almacén sin tocar el
	// servicio y escribe solo la clave y un salto de línea. Nada más sale por stdout.
	clave := &cobra.Command{Use: "clave <herramienta>", Aliases: []string{"key"}, Short: idioma.T("Escribe la clave de la herramienta guardada en el almacén (la piden Claude Code y Codex)"), Args: cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			nombre := servicio.NombreClaveHerramienta(args[0])
			if !credenciales.AdmitidoInterno(nombre) { // las demás herramientas llevan la clave en su archivo
				return fmt.Errorf(idioma.T("%s no guarda su clave en el almacén"), args[0])
			}
			valor, err := leerClave(nombre)
			if err != nil {
				return fmt.Errorf(idioma.T("no se pudo leer la clave de %s del almacén: %w"), args[0], err)
			}
			if valor == "" {
				return fmt.Errorf(idioma.T("%s no tiene clave: actívala con «tramtai herramientas activar %s»"), args[0], args[0])
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), valor)
			return err
		}}

	grupo.AddCommand(listar, activar, desactivar, olvidar, restaurar, clave)
	return grupo
}

// escribirHerramientas escribe la tabla de tramtai herramientas listar: cada fila, con el motivo debajo si lo hay.
func escribirHerramientas(w io.Writer, lista []contrato.EstadoHerramienta) {
	t := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(t, idioma.T("HERRAMIENTA\tID\tESTADO\tMODELO\tCOPIA\tARCHIVO"))
	for _, h := range lista {
		estado := map[string]string{"activa": idioma.T("Activa"), "desactivada": idioma.T("Desactivada"), "desactualizada": idioma.T("Desactualizada"),
			"detectada": idioma.T("Detectada"), "no_detectada": idioma.T("No detectada"), "solo_global": idioma.T("Solo global")}[h.Estado]
		if estado == "" {
			estado = h.Estado
		}
		if h.Estado == "activa" {
			estado = "● " + estado
		} else {
			estado = "○ " + estado
		}
		copia := "-"
		if h.HayCopia {
			copia = idioma.T("sí")
		}
		fmt.Fprintf(t, "%s\t%s\t%s\t%s\t%s\t%s\n", h.Nombre, h.ID, estado, valorO(h.Modelo, "-"), copia, valorO(h.Ruta, "-"))
		if h.Motivo != "" {
			fmt.Fprintf(t, "\t\t\t\t\t↳ %s\n", h.Motivo) // en la última columna, sin cerrar con tabulador: no ensancha ninguna
		}
	}
	t.Flush()
}
