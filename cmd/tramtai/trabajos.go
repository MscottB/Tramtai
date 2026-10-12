package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"tramtai/internal/cliente"
	"tramtai/internal/nucleo"
	"tramtai/internal/servicio"
)

func comandoTrabajos() *cobra.Command {
	grupo := &cobra.Command{Use: "trabajos", Short: "Trabajos de una instancia (la del proyecto actual si está activa; si no, la global)"}
	var instancia string
	grupo.PersistentFlags().StringVarP(&instancia, "instancia", "i", "", "identificador de la instancia")
	elegir := func(ctx context.Context, c *cliente.Cliente) string {
		if instancia != "" {
			return instancia
		}
		if raiz := raizProyecto(); raiz != "" {
			if cfg, err := nucleo.CargarConfiguracion(raiz); err == nil {
				id := servicio.IDProyecto(raiz, cfg)
				if e, err := c.Estado(ctx); err == nil {
					for _, i := range e.Instancias {
						if i.ID == id {
							return id
						}
					}
				}
			}
		}
		return "global"
	}
	var limite int
	listar := &cobra.Command{Use: "listar", Short: "Lista los trabajos más recientes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := conectar()
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			inst := elegir(ctx, c)
			lista, err := c.Trabajos(ctx, inst, limite)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "instancia %s\n", inst)
			for _, t := range lista {
				fmt.Fprintf(out, "  %-32s %-12s %-28s %s\n", t.ID, t.Estado, modeloDe(t), t.CreadoEn.Local().Format("02-01 15:04:05"))
			}
			if len(lista) == 0 {
				fmt.Fprintln(out, "  sin trabajos")
			}
			return nil
		}}
	listar.Flags().IntVarP(&limite, "limite", "n", 20, "cuántos trabajos mostrar (máximo 200)")
	mostrar := &cobra.Command{Use: "mostrar <id>", Short: "Muestra un trabajo y su salida", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := conectar()
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			t, err := c.Trabajo(ctx, elegir(ctx, c), args[0])
			if err != nil {
				return err
			}
			imprimirTrabajo(cmd.OutOrStdout(), t)
			return nil
		}}
	accion := func(nombre, corto string) *cobra.Command {
		return &cobra.Command{Use: nombre + " <id>", Short: corto, Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := conectar()
				if err != nil {
					return err
				}
				ctx, cancelar := contexto()
				defer cancelar()
				t, err := c.Accion(ctx, elegir(ctx, c), args[0], nombre)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", t.ID, t.Estado)
				return nil
			}}
	}
	eliminar := &cobra.Command{Use: "eliminar <id>", Short: "Elimina un trabajo terminado", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := conectar()
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			if err := c.Eliminar(ctx, elegir(ctx, c), args[0]); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "eliminado")
			return nil
		}}
	grupo.AddCommand(listar, mostrar, accion("cancelar", "Cancela un trabajo"), accion("reintentar", "Crea un intento nuevo de un trabajo terminado"), eliminar)
	return grupo
}

func modeloDe(t nucleo.Trabajo) string {
	modelo := t.ModeloEfectivo
	if modelo == "" {
		modelo = t.ModeloEnviado
	}
	if modelo == "" {
		return t.Solicitud.Agente
	}
	return t.Solicitud.Agente + "/" + modelo
}

func imprimirTrabajo(out io.Writer, t nucleo.Trabajo) {
	fmt.Fprintf(out, "%s · %s · %s\n", t.ID, t.Estado, modeloDe(t))
	if t.PuntoAcceso != "" {
		fmt.Fprintf(out, "punto de acceso: %s\n", t.PuntoAcceso)
	}
	fmt.Fprintf(out, "uso: %d entrada / %d salida", t.Uso.TokensEntrada, t.Uso.TokensSalida)
	if t.Uso.CostoUSD > 0 {
		fmt.Fprintf(out, " · US$ %.4f", t.Uso.CostoUSD)
	}
	fmt.Fprintln(out)
	if t.CodigoError != "" {
		fmt.Fprintf(out, "error: %s: %s\n", t.CodigoError, t.MensajeError)
	}
	if t.Salida != "" {
		fmt.Fprintln(out, "\n"+t.Salida)
	}
}
