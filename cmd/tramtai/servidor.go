package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"tramtai/internal/idioma"
	"tramtai/internal/servidor"
	"tramtai/internal/terminos"
)

// Lo que toca el sistema en «tramtai servidor»; las pruebas lo sustituyen.
var (
	esAdministrador     = servidor.EsAdministrador
	instalarServidor    = servidor.Instalar
	desinstalarServidor = servidor.Desinstalar
)

// esDeServidor dice si la orden es «tramtai servidor» o cuelga de ella: trabaja siempre con la raíz del servidor.
func esDeServidor(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "servidor" && c.Parent() != nil && !c.Parent().HasParent() {
			return true
		}
	}
	return false
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
			if err := instalarServidor(servidor.Opciones{Origen: filepath.Dir(ejecutable), AceptoTerminos: true, Como: como}); err != nil {
				return err
			}
			fmt.Fprintln(out, idioma.T("servidor instalado y en marcha (servicio tramtai)\n"+
				"Siguientes pasos: la contraseña del administrador, HTTPS o un proxy, y las claves de los agentes (guía del servidor: docs/servidor.md).\n"+
				"Adminístralo con «sudo tramtai-servidor …»."))
			return nil
		}}
	instalar.Flags().BoolVar(&acepto, "acepto-terminos", false, idioma.T("acepta los términos de uso sin preguntar"))
	instalar.Flags().BoolVar(&acepto, "accept-terms", false, "")
	_ = instalar.Flags().MarkHidden("accept-terms") // alias en inglés

	var borrar bool
	desinstalar := &cobra.Command{Use: "desinstalar", Aliases: []string{"uninstall"}, Args: cobra.NoArgs,
		Short: idioma.T("Quita el servicio del sistema y el programa; conserva los datos salvo con --borrar-datos"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if borrar && !confirmar(cmd, idioma.Tf("¿Borrar también los datos del servidor (%s) y la cuenta %s? No se puede deshacer.", servidor.Rutas().Datos, servidor.Cuenta)) {
				return nil
			}
			if err := desinstalarServidor(borrar); err != nil {
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
			e, err := servidor.Estado(ctx)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if !e.Instalado {
				fmt.Fprintln(out, idioma.T("servicio del sistema: no instalado (sudo tramtai servidor instalar)"))
				return nil
			}
			fmt.Fprintln(out, idioma.Tf("servicio del sistema: %s", e.Activo))
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
