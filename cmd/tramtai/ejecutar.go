package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/planificador"
	"tramtai/internal/version"
)

func comandoAgentes() *cobra.Command {
	return &cobra.Command{Use: "agentes", Short: "Diagnostica cada agente: versión, autenticación y términos (sin consumir cuota)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := cargarConfiguracion()
			if err != nil {
				return err
			}
			registro, _ := adaptadores.Registro(c)
			nombres := make([]string, 0, len(registro))
			for nombre := range registro {
				nombres = append(nombres, nombre)
			}
			sort.Strings(nombres)
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "tramtai", version.Texto())
			for _, nombre := range nombres {
				detectable, ok := registro[nombre].(adaptadores.Detectable)
				if !ok {
					continue
				}
				ctx, cancelar := context.WithTimeout(cmd.Context(), 15*time.Second)
				e := detectable.Detectar(ctx)
				cancelar()
				fmt.Fprintf(out, "\n%s\n  versión:       %s\n  ejecutable:    %s\n  autenticación: %s\n  términos:      %s\n",
					e.Agente, vacio(e.Version), vacio(e.Ruta), e.Autenticacion, e.Terminos)
				if e.Problema != "" {
					fmt.Fprintf(out, "  problema:      %s\n", e.Problema)
				}
			}
			return nil
		}}
}

func comandoEjecutar() *cobra.Command {
	var agente, modelo, perfil, sistema string
	var tiempo int
	cmd := &cobra.Command{Use: "ejecutar [texto]", Short: "Ejecuta un trabajo con un agente y muestra la respuesta en vivo",
		Long: "Ejecuta un trabajo directamente, sin servicio. Si no se pasa texto, se lee de la entrada estándar.\nCtrl+C cancela el trabajo y termina el proceso del agente.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			texto := strings.Join(args, " ")
			if texto == "" {
				datos, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return err
				}
				texto = strings.TrimSpace(string(datos))
			}
			c, err := cargarConfiguracion()
			if err != nil {
				return err
			}
			s := nucleo.SolicitudEjecucion{Agente: agente, Modelo: modelo, PerfilModelo: perfil, Instrucciones: sistema,
				Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}}
			s.Ejecucion.TiempoLimiteSegundos = tiempo
			return ejecutar(cmd, c, c.AplicarPredeterminados(s))
		}}
	f := cmd.Flags()
	f.StringVarP(&agente, "agente", "a", "", "agente: claude o codex (predeterminados.agente)")
	f.StringVarP(&modelo, "modelo", "m", "", "identificador o alias del modelo, por ejemplo haiku")
	f.StringVarP(&perfil, "perfil", "p", "", "perfil de modelo: rapido, equilibrado, profundo (modelos.perfiles)")
	f.StringVarP(&sistema, "sistema", "s", "", "instrucciones de sistema")
	f.IntVarP(&tiempo, "tiempo", "t", 0, "tiempo límite en segundos (300 por defecto)")
	return cmd
}

func ejecutar(cmd *cobra.Command, c *nucleo.Configuracion, s nucleo.SolicitudEjecucion) error {
	dirConfig, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	datos := filepath.Join(dirConfig, "tramtai", "datos")
	if err := os.MkdirAll(datos, 0o700); err != nil {
		return err
	}
	alm, err := persistencia.Abrir(filepath.Join(datos, "cli.db"))
	if err != nil {
		return err
	}
	defer alm.Cerrar()

	registro, limites := adaptadores.Registro(c)
	// shortcut: cada ejecución directa usa su propia instancia para no relanzar trabajos de otra ejecución
	// interrumpida; el servicio (M4) tendrá instancias persistentes con su recuperación.
	opc := planificador.OpcionesDesde(c, "cli-"+nucleo.NuevoID()[len("trabajo_"):])
	p := planificador.Nuevo(alm, registro, planificador.NuevosSemaforos(limites), opc)
	if err := p.Iniciar(cmd.Context()); err != nil {
		return err
	}
	defer p.Detener()

	t, _, err := p.Enviar(cmd.Context(), s, "")
	if err != nil {
		return err
	}
	errOut, out := cmd.ErrOrStderr(), cmd.OutOrStdout()
	fmt.Fprintf(errOut, "tramtai %s · %s · agente %s · modelo %s\n\n", version.Texto(), t.ID, s.Agente, vacio(t.ModeloEnviado))

	interrupcion, parar := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer parar()
	inicio, ultimo, cancelado := time.Now(), 0, false
	for {
		if interrupcion.Err() != nil && !cancelado {
			cancelado = true
			fmt.Fprintln(errOut, "\ncancelando…")
			if _, err := p.Cancelar(context.Background(), t.ID); err != nil {
				return err
			}
		}
		eventos, err := alm.Eventos(context.Background(), t.ID, ultimo)
		if err != nil {
			return err
		}
		terminado := false
		for _, e := range eventos {
			ultimo = e.Numero
			switch {
			case e.Tipo == "texto":
				fmt.Fprint(out, e.Datos)
			case e.Tipo == "estado" && nucleo.Estado(e.Datos).Terminal():
				terminado = true
			}
		}
		if terminado {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	final, err := alm.Obtener(context.Background(), t.ID)
	if err != nil {
		return err
	}
	efectivo := final.ModeloEfectivo
	if efectivo == "" {
		efectivo = "no verificable"
	}
	fmt.Fprintf(errOut, "\n\n%s · modelo efectivo %s · tokens %d entrada / %d salida", final.Estado, efectivo, final.Uso.TokensEntrada, final.Uso.TokensSalida)
	if final.Uso.CostoUSD > 0 {
		fmt.Fprintf(errOut, " · US$ %.4f", final.Uso.CostoUSD)
	}
	fmt.Fprintf(errOut, " · %s\n", time.Since(inicio).Round(100*time.Millisecond))
	if final.Estado != nucleo.Completado {
		return nucleo.NuevoError(final.CodigoError, final.MensajeError)
	}
	return nil
}

func vacio(texto string) string {
	if texto == "" {
		return "—"
	}
	return texto
}
