package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"tramtai/internal/agentes"
	"tramtai/internal/carpetas"
	"tramtai/internal/configuracion"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/servidor"
	"tramtai/internal/terminos"
)

// Lo que toca el sistema en «tramtai servidor»; las pruebas lo sustituyen.
var (
	esAdministrador     = servidor.EsAdministrador
	instalarServidor    = servidor.Instalar
	desinstalarServidor = servidor.Desinstalar
	estadoServidor      = servidor.Estado
)

// avisarExpuesto recuerda el riesgo 3 si la configuración del servidor lo abre a la red; si no se puede leer, lo dice.
func avisarExpuesto(out io.Writer) {
	c, err := configuracion.CargarConfiguracion("", agentes.Internos())
	var red servidor.Entrada
	if err == nil {
		red, err = entradaServidor(c)
	}
	switch {
	case err != nil:
		fmt.Fprintln(out, idioma.Tf("aviso: %s", nucleo.MensajeDe(err)))
	case servidor.Expuesto(red):
		fmt.Fprintln(out, idioma.Tf("aviso: %s", terminos.Riesgos(true)[2]))
	}
}

// esDeServidor dice si la orden es «tramtai servidor» o cuelga de ella: trabaja siempre con la raíz del servidor.
func esDeServidor(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "servidor" && c.Parent() != nil && !c.Parent().HasParent() {
			return true
		}
	}
	return false
}

// ajustarDuenoServidor deja a nombre de la cuenta del servicio lo que una orden, como administrador en modo servidor,
// haya escrito en la raíz del servidor (config.yaml, terminos.json…): si no, el servicio no podría leerlo.
func ajustarDuenoServidor(errores io.Writer) {
	if !carpetas.Servidor() || !esAdministrador() {
		return
	}
	if err := servidor.AjustarDueno(carpetas.RaizServidor()); err != nil {
		fmt.Fprintln(errores, idioma.Tf("aviso: no se pudo dejar %s a nombre de la cuenta del servicio: %s", carpetas.RaizServidor(), err.Error()))
	}
}

// ordenDeServidor deja escribir las órdenes de «servidor» sin «servidor» tras --servidor, que es como las lanza el
// envoltorio: «tramtai-servidor instalar» llega como «tramtai --servidor instalar» y se ejecuta «servidor instalar».
func ordenDeServidor(raiz *cobra.Command, args []string) []string {
	if len(args) < 2 || args[0] != "--servidor" && args[0] != "--server" {
		return args
	}
	if grupo, _, err := raiz.Find([]string{"servidor"}); err == nil {
		for _, c := range grupo.Commands() {
			if c.Name() == args[1] || c.HasAlias(args[1]) {
				return slices.Insert(slices.Clone(args), 1, "servidor")
			}
		}
	}
	return args
}

func comandoServidor() *cobra.Command {
	grupo := &cobra.Command{Use: "servidor", Aliases: []string{"server"},
		Short: idioma.T("Servicio del sistema (modo servidor): instalar, desinstalar, estado, reiniciar y registro")}

	var acepto bool
	var conversor string
	instalar := &cobra.Command{Use: "instalar", Aliases: []string{"install"}, Args: cobra.NoArgs,
		Short: idioma.T("Instala el servicio del sistema y lo arranca, tras aceptar los términos de uso"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, idioma.T("Antes de instalar el servidor, lee los riesgos:"))
			for i, riesgo := range terminos.Riesgos(true) {
				fmt.Fprintf(out, "  %d. %s\n", i+1, riesgo)
			}
			fmt.Fprintln(out, idioma.Tf("Términos de uso: %s", terminos.URL))
			como := terminos.ComoOpcion
			if !acepto {
				fmt.Fprint(cmd.ErrOrStderr(), idioma.T("Escribe «acepto» para aceptarlos e instalar: "))
				linea, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				if !slices.Contains([]string{"acepto", idioma.T("acepto")}, strings.ToLower(strings.TrimSpace(linea))) {
					return errors.New(idioma.T("instalación cancelada: no se aceptaron los términos de uso"))
				}
				como = terminos.ComoServidorInstalar
			}
			ejecutable, err := os.Executable()
			if err != nil {
				return err
			}
			if real, err := filepath.EvalSymlinks(ejecutable); err == nil {
				ejecutable = real
			}
			if conversor != "" {
				if conversor, err = filepath.Abs(conversor); err != nil {
					return err
				}
			}
			if err := instalarServidor(servidor.Opciones{Origen: filepath.Dir(ejecutable), AceptoTerminos: true, Como: como, Conversor: conversor}); err != nil {
				return err
			}
			if runtime.GOOS == "windows" {
				fmt.Fprintln(out, idioma.T("servidor instalado y en marcha (servicio Tramtai)\n"+
					"Siguientes pasos: la contraseña del administrador, HTTPS o un proxy, y las claves de los agentes (guía del servidor: docs/servidor.md).\n"+
					"Adminístralo con «tramtai-servidor …» desde una consola de administrador nueva (esta aún no ve el PATH)."))
				return nil
			}
			fmt.Fprintln(out, idioma.T("servidor instalado y en marcha (servicio tramtai)\n"+
				"Siguientes pasos: la contraseña del administrador, HTTPS o un proxy, y las claves de los agentes (guía del servidor: docs/servidor.md).\n"+
				"Adminístralo con «sudo tramtai-servidor …»."))
			return nil
		}}
	instalar.Flags().BoolVar(&acepto, "acepto-terminos", false, idioma.T("acepta los términos de uso sin preguntar"))
	instalar.Flags().BoolVar(&acepto, "accept-terms", false, "")
	_ = instalar.Flags().MarkHidden("accept-terms") // alias en inglés
	instalar.Flags().StringVar(&conversor, "conversor", "", idioma.T("prepara el conversor de archivos sin red desde su paquete (.zip)"))
	instalar.Flags().StringVar(&conversor, "converter", "", "")
	_ = instalar.Flags().MarkHidden("converter") // alias en inglés

	var borrar bool
	desinstalar := &cobra.Command{Use: "desinstalar", Aliases: []string{"uninstall"}, Args: cobra.NoArgs,
		Short: idioma.T("Quita el servicio del sistema y el programa; conserva los datos salvo con --borrar-datos"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			pregunta := idioma.Tf("¿Borrar también los datos del servidor (%s) y la cuenta %s? No se puede deshacer.", servidor.Rutas().Datos, servidor.Cuenta)
			if runtime.GOOS == "windows" {
				pregunta = idioma.Tf("¿Borrar también los datos del servidor (%s) y el perfil de la cuenta %s, con las sesiones de los agentes? No se puede deshacer.", servidor.Rutas().Datos, servidor.CuentaWindows)
			}
			if borrar && !confirmar(cmd, pregunta) {
				fmt.Fprintln(cmd.OutOrStdout(), idioma.T("desinstalación cancelada"))
				return nil
			}
			var aviso servidor.Aviso
			if err := desinstalarServidor(borrar); errors.As(err, &aviso) {
				fmt.Fprintln(cmd.ErrOrStderr(), idioma.Tf("aviso: %s", aviso.Error()))
			} else if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), idioma.T("servidor desinstalado"))
			return nil
		}}
	desinstalar.Flags().BoolVar(&borrar, "borrar-datos", false, idioma.T("borra también la carpeta de datos y la cuenta del servicio"))
	desinstalar.Flags().BoolVar(&borrar, "delete-data", false, "")
	_ = desinstalar.Flags().MarkHidden("delete-data") // alias en inglés

	estado := &cobra.Command{Use: "estado", Aliases: []string{"status"}, Args: cobra.NoArgs,
		Short: idioma.T("Muestra si el servicio del sistema está en marcha, con sus espacios y sus APIs"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancelar := contexto()
			defer cancelar()
			e, err := estadoServidor(ctx)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if !e.Instalado && runtime.GOOS == "windows" {
				fmt.Fprintln(out, idioma.T("servicio del sistema: no instalado (tramtai servidor instalar, desde una consola de administrador)"))
				return nil
			}
			if !e.Instalado {
				fmt.Fprintln(out, idioma.T("servicio del sistema: no instalado (sudo tramtai servidor instalar)"))
				return nil
			}
			fmt.Fprintln(out, idioma.Tf("servicio del sistema: %s", e.Activo))
			if e.ClavesEnClaro {
				fmt.Fprintln(out, idioma.Tf("aviso: %s", servidor.ErrClaveEnClaro.Error()))
			}
			avisarExpuesto(out)
			c, err := conectar()
			if err != nil {
				fmt.Fprintln(out, idioma.Tf("no responde: %s", err.Error()))
				return nil
			}
			estadoServicio, err := c.Estado(ctx)
			if err != nil {
				return err
			}
			imprimirEstado(out, estadoServicio)
			return nil
		}}

	reiniciar := &cobra.Command{Use: "reiniciar", Aliases: []string{"restart"}, Args: cobra.NoArgs,
		Short: idioma.T("Reinicia el servicio del sistema (deja terminar las peticiones en curso)"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := servidor.Reiniciar(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), idioma.T("servidor reiniciado"))
			return nil
		}}

	var lineas int
	var seguir bool
	registro := &cobra.Command{Use: "registro", Aliases: []string{"log"}, Args: cobra.NoArgs,
		Short: idioma.T("Muestra el registro del servicio del sistema (servicio.log); -f lo sigue en vivo"),
		RunE:  func(cmd *cobra.Command, _ []string) error { return seguirRegistro(cmd, lineas, seguir) }}
	registro.Flags().IntVarP(&lineas, "lineas", "n", 30, idioma.T("líneas finales que se muestran"))
	registro.Flags().BoolVarP(&seguir, "seguir", "f", false, idioma.T("sigue mostrando las líneas nuevas hasta Ctrl+C"))

	grupo.AddCommand(instalar, desinstalar, estado, reiniciar, registro)
	for _, c := range grupo.Commands() {
		c.PreRunE = func(*cobra.Command, []string) error {
			if !esAdministrador() {
				return errors.New(idioma.T("tramtai servidor necesita permisos de administrador: en Linux, con sudo (sudo tramtai-servidor …); en Windows, desde una consola de administrador"))
			}
			return nil
		}
	}
	return grupo
}
