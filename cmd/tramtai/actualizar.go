package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"tramtai/internal/actualizacion"
	"tramtai/internal/idioma"
	"tramtai/internal/version"
)

func comandoActualizar() *cobra.Command {
	var comprobar, si bool
	cmd := &cobra.Command{Use: "actualizar", Short: idioma.T("Comprueba si hay una versión nueva de Tramtai y la instala"),
		Long: idioma.T("Consulta en GitHub la última versión publicada. Si es más nueva, descarga el instalador, comprueba su suma " +
			"SHA-256 y lo ejecuta en silencio: la app se cierra, el servicio se detiene y ambos vuelven con la versión nueva, " +
			"con tu configuración y tus datos. La instalación automática solo funciona en una copia instalada con el instalador de Windows."),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			ctx, cancelar := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancelar()
			v, err := actualizacion.Ultima(ctx)
			if err != nil {
				return err
			}
			if !actualizacion.MasNueva(v.Etiqueta, version.Version) {
				fmt.Fprintln(out, idioma.Tf("Ya tienes la última versión (%s).", version.Texto()))
				return nil
			}
			fmt.Fprintln(out, idioma.Tf("Hay una versión nueva: %s (tienes %s).", v.Etiqueta, version.Texto()))
			fmt.Fprintln(out, idioma.Tf("Novedades: %s", v.Pagina))
			carpeta, instalada := actualizacion.Instalada()
			if comprobar {
				return nil
			}
			if !instalada || !v.TieneInstalador() {
				fmt.Fprintln(out, idioma.T("Esta copia no se instaló con el instalador de Windows: descarga la versión nueva desde esa página."))
				return nil
			}
			if !si && !confirmar(cmd, idioma.T("¿Instalarla ahora? La app se cerrará y el servicio se detendrá unos segundos.")) {
				return nil
			}
			dir, err := os.MkdirTemp("", actualizacion.PrefijoCarpeta)
			if err != nil {
				return err
			}
			fmt.Fprintln(out, idioma.T("Descargando el instalador…"))
			ultimo := -1
			instalador, err := actualizacion.Descargar(ctx, v, dir, func(recibidos, total int64) {
				if porcentaje := int(recibidos * 100 / max(total, 1)); total > 0 && porcentaje != ultimo {
					ultimo = porcentaje
					fmt.Fprintf(out, "\r%s", idioma.Tf("Descargando… %d %% (%.1f de %.1f MB)", porcentaje, float64(recibidos)/(1<<20), float64(total)/(1<<20)))
				}
			})
			if ultimo >= 0 {
				fmt.Fprintln(out)
			}
			if err != nil {
				os.RemoveAll(dir)
				return err
			}
			fmt.Fprintln(out, idioma.T("Suma SHA-256 correcta. Instalando: Tramtai se cerrará y volverá a abrirse."))
			return actualizacion.Instalar(instalador, carpeta)
		}}
	cmd.Flags().BoolVar(&comprobar, "comprobar", false, idioma.T("solo comprueba si hay una versión nueva, sin instalarla"))
	cmd.Flags().BoolVarP(&si, "si", "s", false, idioma.T("no pedir confirmación"))
	return cmd
}
