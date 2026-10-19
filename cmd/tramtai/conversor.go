package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/documentos"
	"tramtai/internal/idioma"
)

// servicioConversor conecta con el servicio (o lo inicia); las pruebas lo sustituyen.
var servicioConversor = asegurarServicio

// plazoConversor cubre una preparación entera (el servicio la corta a los 30 minutos).
const plazoConversor = 35 * time.Minute

func comandoConversor() *cobra.Command {
	grupo := &cobra.Command{Use: "conversor", Short: idioma.T("Conversor de archivos (markitdown): estado y preparación")}
	estado := &cobra.Command{Use: "estado", Short: idioma.T("Muestra si el conversor de archivos está listo"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, inst, ctx, cancelar, err := conectarConversor(cmd)
			if err != nil {
				return err
			}
			defer cancelar()
			e, err := c.Conversor(ctx, inst)
			if err != nil {
				return err
			}
			mostrarConversor(cmd.OutOrStdout(), e)
			return nil
		}}
	var en string
	preparar := &cobra.Command{Use: "preparar", Short: idioma.T("Descarga Python y markitdown en la carpeta de Tramtai (no instala nada en el sistema)"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			if en != "" {
				return prepararEnCarpeta(out, en)
			}
			c, inst, ctx, cancelar, err := conectarConversor(cmd)
			if err != nil {
				return err
			}
			defer cancelar()
			e, err := c.PrepararConversor(ctx, inst)
			ultimo := ""
			for err == nil && e.Estado == "preparando" {
				if linea := fmt.Sprintf("%s %d%%", e.Etapa, e.Progreso); linea != ultimo {
					fmt.Fprintln(out, linea)
					ultimo = linea
				}
				time.Sleep(time.Second)
				e, err = c.Conversor(ctx, inst)
			}
			if err != nil {
				return err
			}
			mostrarConversor(out, e)
			if e.Estado != "listo" || e.Mensaje != "" {
				return errors.New(idioma.T("no se pudo preparar el conversor de archivos"))
			}
			return nil
		}}
	preparar.Flags().StringVar(&en, "en", "", idioma.T("prepara el conversor en esa carpeta, sin el servicio (para el paquete del conversor)"))
	grupo.AddCommand(estado, preparar)
	return grupo
}

// conectarConversor conecta con el servicio y elige la instancia (la del proyecto o la global: el conversor es el
// mismo para todas).
func conectarConversor(cmd *cobra.Command) (*cliente.Cliente, string, context.Context, context.CancelFunc, error) {
	c, err := servicioConversor(cmd)
	if err != nil {
		return nil, "", nil, nil, err
	}
	ctx, cancelar := context.WithTimeout(context.Background(), plazoConversor)
	return c, instanciaActual(ctx, c, ""), ctx, cancelar, nil
}

func mostrarConversor(out io.Writer, e contrato.EstadoConversor) {
	fmt.Fprintf(out, idioma.T("estado: %s\n"), e.Estado)
	if e.Origen != "" {
		fmt.Fprintf(out, idioma.T("origen: %s\n"), e.Origen)
	}
	if e.Version != "" {
		fmt.Fprintf(out, "markitdown %s\n", e.Version)
	}
	if e.Mensaje != "" {
		fmt.Fprintf(out, idioma.T("último error: %s\n"), e.Mensaje)
	}
	if e.Estado == "falta" || e.Estado == "error" {
		fmt.Fprintln(out, idioma.T("prepáralo con «tramtai conversor preparar» o en la Web UI, en Ajustes → Archivos"))
	}
}

// prepararEnCarpeta prepara el conversor en dir sin el servicio, con el uv que viene con Tramtai.
func prepararEnCarpeta(out io.Writer, dir string) error {
	uv, err := documentos.BuscarUV()
	if err != nil {
		return err
	}
	destino, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	ctx, cancelar := context.WithTimeout(context.Background(), plazoConversor)
	defer cancelar()
	return documentos.PrepararEn(ctx, destino, uv, func(etapa string, progreso int) { fmt.Fprintf(out, "%s %d%%\n", etapa, progreso) })
}
