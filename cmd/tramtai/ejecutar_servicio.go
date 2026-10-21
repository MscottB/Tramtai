package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/documentos"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/servicio"
	"tramtai/internal/version"
)

// instanciaActual elige el espacio: el indicado (-i), la global si se pide (--global), el del proyecto actual si está
// activo, o la global fuera de un proyecto. Dentro de un proyecto detenido es un error: no se usa la global sin decirlo.
func instanciaActual(ctx context.Context, c *cliente.Cliente, preferida string, global bool) (string, error) {
	if preferida != "" || global {
		return elegirEspacio("", nil, preferida, global)
	}
	raiz := raizProyecto()
	if raiz == "" {
		return "global", nil
	}
	e, err := c.Estado(ctx)
	if err != nil {
		return "", err
	}
	return elegirEspacio(raiz, e.Instancias, "", false)
}

// elegirEspacio es la decisión de instanciaActual con el estado ya leído.
func elegirEspacio(raiz string, activos []contrato.EstadoInstancia, preferida string, global bool) (string, error) {
	switch {
	case preferida != "":
		return preferida, nil
	case global || raiz == "":
		return "global", nil
	}
	for _, i := range activos {
		if i.Ambito == "proyecto" && servicio.MismaRuta(i.Raiz, raiz) {
			return i.ID, nil
		}
	}
	return "", nucleo.NuevoError(nucleo.CodigoEspacioDetenido, idioma.Tf("el espacio %s está detenido: actívalo con tramtai espacio iniciar, o usa --global", filepath.Base(raiz)))
}

// ejecutarEnServicio envía el trabajo al servicio en marcha, de modo que el mismo trabajo se ve en la terminal
// interactiva y en la Web UI (recorrido compartido entre interfaces).
func ejecutarEnServicio(cmd *cobra.Command, c *cliente.Cliente, s nucleo.SolicitudEjecucion, archivos []string) error {
	ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelar()
	c.Origen = nucleo.DesdeCLI
	global, _ := cmd.Flags().GetBool("global")
	inst, err := instanciaActual(ctx, c, "", global)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(archivos, func(ruta string) bool { return documentos.NecesitaConversor(ruta) }) {
		if e, err := c.Conversor(ctx, inst); err == nil && e.Estado == "preparando" {
			return nucleo.NuevoError(nucleo.CodigoConversorNoDisponible, idioma.T("el conversor de archivos se está preparando: vuelve a intentarlo en un momento"))
		} else if err == nil && e.Estado != "listo" {
			return documentos.ErrSinConversor() // antes de subir nada
		}
	}
	for _, ruta := range archivos {
		carga, err := c.SubirCarga(ctx, inst, ruta)
		if err != nil {
			return err
		}
		s.Cargas = append(s.Cargas, carga.ID)
	}
	t, err := c.Crear(ctx, inst, s)
	if err != nil {
		return err
	}
	errOut, out := cmd.ErrOrStderr(), cmd.OutOrStdout()
	fmt.Fprintf(errOut, idioma.T("tramtai %s · %s · espacio %s · agente %s · modelo %s\n\n"), version.Texto(), t.ID, inst, t.Solicitud.Agente, vacio(t.ModeloEnviado))

	interrupcion, parar := signal.NotifyContext(context.Background(), os.Interrupt)
	defer parar()
	terminado := make(chan struct{})
	defer close(terminado)
	go func() {
		select {
		case <-terminado:
		case <-interrupcion.Done():
			select {
			case <-terminado:
			default:
				fmt.Fprintln(errOut, idioma.T("\ncancelando…"))
				_, _ = c.Accion(context.Background(), inst, t.ID, "cancelar")
			}
		}
	}()

	inicio := time.Now()
	var impreso strings.Builder
	if err := c.Eventos(context.Background(), inst, t.ID, 0, func(_ int, tipo, datos string) error {
		switch {
		case tipo == "texto":
			fmt.Fprint(out, datos)
			impreso.WriteString(datos)
		case tipo == "estado" && datos == string(nucleo.PreparandoEntrada):
			fmt.Fprintln(errOut, idioma.T("convirtiendo archivos…"))
		}
		return nil
	}); err != nil {
		return err
	}
	final, err := c.Trabajo(context.Background(), inst, t.ID)
	if err != nil {
		return err
	}
	return imprimirFinal(cmd, final, impreso.String(), inicio)
}

// imprimirFinal muestra la salida final si difiere de la emitida en vivo y el resumen del trabajo.
func imprimirFinal(cmd *cobra.Command, final nucleo.Trabajo, impreso string, inicio time.Time) error {
	errOut, out := cmd.ErrOrStderr(), cmd.OutOrStdout()
	// Con salida estructurada el resultado final no siempre coincide con el texto emitido en vivo.
	if final.Estado == nucleo.Completado && final.Salida != impreso {
		if impreso != "" {
			fmt.Fprint(errOut, idioma.T("\n\nsalida final:\n"))
		}
		fmt.Fprint(out, final.Salida)
	}
	efectivo := final.ModeloEfectivo
	if efectivo == "" {
		efectivo = idioma.T("no verificable")
	}
	fmt.Fprintf(errOut, idioma.T("\n\n%s · modelo efectivo %s · tokens %d entrada / %d salida"), final.Estado, efectivo, final.Uso.TokensEntrada, final.Uso.TokensSalida)
	if final.Uso.CostoUSD > 0 {
		fmt.Fprintf(errOut, " · US$ %.4f", final.Uso.CostoUSD)
	}
	fmt.Fprintf(errOut, " · %s\n", time.Since(inicio).Round(100*time.Millisecond))
	if final.Estado != nucleo.Completado {
		return nucleo.NuevoError(final.CodigoError, final.MensajeError)
	}
	return nil
}
