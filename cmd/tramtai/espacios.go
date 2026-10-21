package main

import (
	"fmt"
	"io"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/servicio"
)

// comandoEspacios lista todos los espacios de trabajo, también los detenidos y los que no encuentran su carpeta.
func comandoEspacios() *cobra.Command {
	return &cobra.Command{Use: "espacios", Short: idioma.T("Lista los espacios de trabajo: activos, detenidos y no encontrados"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := conectar()
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			lista, err := c.Espacios(ctx)
			if err != nil {
				return err
			}
			escribirEspacios(cmd.OutOrStdout(), lista)
			return nil
		}}
}

// escribirEspacios escribe la tabla de tramtai espacios.
func escribirEspacios(w io.Writer, lista []contrato.Espacio) {
	t := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(t, idioma.T("ESPACIO\tID\tESTADO\tAPIS\tPETICIONES\tCARPETA"))
	for _, e := range lista {
		estado := map[string]string{"activo": idioma.T("activo"), "detenido": idioma.T("detenido"), "no_encontrado": idioma.T("no encontrado")}[e.Estado]
		if e.Ambito == "proyecto" && !e.Confiado && e.Estado != "no_encontrado" {
			estado += idioma.T(", sin confiar")
		}
		fmt.Fprintf(t, "%s\t%s\t%s\t%d\t%d\t%s\n", e.Nombre, e.ID, estado, e.APIs, e.Peticiones, e.Ruta)
		if e.UltimoError != "" {
			fmt.Fprintf(t, "  ↳ %s\t\t\t\t\t\n", e.UltimoError)
		}
	}
	t.Flush()
}

// comandoEspacio activa, detiene u olvida un espacio, por su carpeta o su id (sin nada, el del proyecto actual o la
// global).
func comandoEspacio() *cobra.Command {
	grupo := &cobra.Command{Use: "espacio", Short: idioma.T("Activa, detiene u olvida un espacio de trabajo")}
	// resolver da el ámbito, la carpeta y el id del espacio que nombra arg.
	resolver := func(arg string) (ambito, raiz, id string, err error) {
		if arg == "" {
			if raiz = raizProyecto(); raiz == "" {
				return "global", "", "global", nil
			}
			arg = raiz
		}
		if arg == "global" {
			return "global", "", "global", nil
		}
		if raiz, ok := nucleo.BuscarProyecto(arg); ok {
			c, err := nucleo.CargarConfiguracion(raiz)
			if err != nil {
				return "", "", "", err
			}
			return "proyecto", raiz, servicio.IDDeCarpeta(raiz, c), nil
		}
		cli, err := conectar()
		if err != nil {
			return "", "", "", err
		}
		ctx, cancelar := contexto()
		defer cancelar()
		lista, err := cli.Espacios(ctx)
		if err != nil {
			return "", "", "", err
		}
		for _, e := range lista {
			if e.ID == arg {
				return e.Ambito, e.Ruta, e.ID, nil
			}
		}
		return "", "", "", nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("no hay ningún espacio %s: mira tramtai espacios", filepath.Clean(arg)))
	}
	iniciar := &cobra.Command{Use: "iniciar [carpeta|id]", Short: idioma.T("Activa el espacio (inicia el servicio si hace falta)"), Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ambito, raiz, _, err := resolver(primero(args))
			if err != nil {
				return err
			}
			c, err := asegurarServicio(cmd)
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			a, err := c.Activar(ctx, ambito, raiz)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("espacio %s activo en %s\n"), a.Instancia.ID, c.Base)
			return nil
		}}
	detener := &cobra.Command{Use: "detener [carpeta|id]", Short: idioma.T("Detiene el espacio: vacía su cola sin afectar a los demás"), Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, id, err := resolver(primero(args))
			if err != nil {
				return err
			}
			c, err := conectar()
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			if err := c.Desactivar(ctx, id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("espacio %s detenido\n"), id)
			return nil
		}}
	var nombre string
	olvidar := &cobra.Command{Use: "olvidar <id>", Short: idioma.T("Olvida un espacio y borra sus datos en Tramtai (nunca la carpeta del proyecto)"), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := conectar()
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			if err := c.Olvidar(ctx, args[0], nombre); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("espacio %s olvidado\n"), args[0])
			return nil
		}}
	olvidar.Flags().StringVar(&nombre, "nombre", "", idioma.T("el nombre del espacio, para confirmar"))
	_ = olvidar.MarkFlagRequired("nombre")
	grupo.AddCommand(iniciar, detener, olvidar)
	return grupo
}

func primero(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
