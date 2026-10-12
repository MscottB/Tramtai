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
	"tramtai/internal/documentos"
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
	var agente, modelo, perfil, sistema, esquema, formato string
	var archivos []string
	var tiempo int
	var directo bool
	cmd := &cobra.Command{Use: "ejecutar [texto]", Short: "Ejecuta un trabajo con un agente y muestra la respuesta en vivo",
		Long: "Ejecuta un trabajo y muestra la respuesta en vivo. Si el servicio está en marcha, el trabajo va a la instancia del\nproyecto actual (o a la global) y se ve también en la terminal interactiva y en la Web UI; si no, se ejecuta\ndirectamente. Sin texto, se lee de la entrada estándar. Ctrl+C cancela el trabajo.",
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
			s.Salida.Formato = formato
			if esquema != "" {
				if s.Salida.Esquema, err = os.ReadFile(esquema); err != nil {
					return err
				}
			}
			if !directo {
				if cli, err := conectar(); err == nil {
					return ejecutarEnServicio(cmd, cli, s, archivos)
				}
			}
			return ejecutar(cmd, c, c.AplicarPredeterminados(s), archivos)
		}}
	f := cmd.Flags()
	f.StringVarP(&agente, "agente", "a", "", "agente: claude o codex (predeterminados.agente)")
	f.StringVarP(&modelo, "modelo", "m", "", "identificador o alias del modelo, por ejemplo haiku")
	f.StringVarP(&perfil, "perfil", "p", "", "perfil de modelo: rapido, equilibrado, profundo (modelos.perfiles)")
	f.StringVarP(&sistema, "sistema", "s", "", "instrucciones de sistema")
	f.IntVarP(&tiempo, "tiempo", "t", 0, "tiempo límite en segundos (300 por defecto)")
	f.StringArrayVarP(&archivos, "archivo", "f", nil, "archivo de entrada, repetible: PDF, DOCX, PPTX, XLSX, HTML, texto o ZIP")
	f.StringVarP(&esquema, "esquema", "e", "", "archivo JSON Schema que debe cumplir la salida (implica formato json)")
	f.StringVar(&formato, "formato", "", "formato de salida: texto, markdown o json")
	f.BoolVar(&directo, "directo", false, "ejecuta sin el servicio aunque esté en marcha (base cli.db propia)")
	return cmd
}

func comandoConvertir() *cobra.Command {
	return &cobra.Command{Use: "convertir <archivo>...", Short: "Muestra el texto que recibiría el agente para esos archivos (sin ejecutar ningún agente)",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cargarConfiguracion()
			if err != nil {
				return err
			}
			entradas := make([]documentos.Entrada, len(args))
			for i, ruta := range args {
				entradas[i] = documentos.Entrada{Ruta: ruta, Nombre: filepath.Base(ruta)}
			}
			texto, err := documentos.Desde(c).Convertir(cmd.Context(), entradas)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), texto)
			return nil
		}}
}

func ejecutar(cmd *cobra.Command, c *nucleo.Configuracion, s nucleo.SolicitudEjecucion, archivos []string) error {
	datos, err := dirDatos()
	if err != nil {
		return err
	}
	alm, err := persistencia.Abrir(filepath.Join(datos, "cli.db"))
	if err != nil {
		return err
	}
	defer alm.Cerrar()

	cargas, err := documentos.AbrirCargas(filepath.Join(datos, "cargas", "cli"),
		time.Duration(c.Entero("retencion.horas_carga_huerfana"))*time.Hour, int64(c.Entero("conversion.maximo_carga_mb"))<<20)
	if err != nil {
		return err
	}
	for _, ruta := range archivos {
		archivo, err := os.Open(ruta)
		if err != nil {
			return err
		}
		carga, err := cargas.Guardar(filepath.Base(ruta), archivo)
		archivo.Close()
		if err != nil {
			return err
		}
		s.Cargas = append(s.Cargas, carga.ID)
	}

	registro, limites := adaptadores.Registro(c)
	// shortcut: cada ejecución directa usa su propia instancia para no relanzar trabajos de otra ejecución
	// interrumpida; el servicio (M4) tendrá instancias persistentes con su recuperación.
	opc := planificador.OpcionesDesde(c, "cli-"+nucleo.NuevoID()[len("trabajo_"):])
	opc.Preparador = documentos.Preparador{Cargas: cargas, Conversor: documentos.Desde(c)}
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
	var impreso strings.Builder
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
				impreso.WriteString(e.Datos)
			case e.Tipo == "estado" && e.Datos == string(nucleo.PreparandoEntrada):
				fmt.Fprintln(errOut, "convirtiendo archivos…")
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
	return imprimirFinal(cmd, final, impreso.String(), inicio)
}

func vacio(texto string) string {
	if texto == "" {
		return "—"
	}
	return texto
}
