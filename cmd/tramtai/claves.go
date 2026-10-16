package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"tramtai/internal/credenciales"
	"tramtai/internal/idioma"
)

// comandoClaves administra las claves de API en el almacén de credenciales del sistema. Nunca las imprime.
func comandoClaves() *cobra.Command {
	grupo := &cobra.Command{Use: "claves", Short: idioma.T("Guarda las claves de API en el almacén cifrado del sistema (no en archivos ni variables de entorno)"),
		Long: idioma.Tf("Guarda las claves de API de los proveedores en el almacén cifrado del sistema (Administrador de credenciales\nen Windows, Llavero en macOS, Secret Service en Linux); Tramtai nunca las escribe en sus archivos. Una variable\nde entorno con el mismo nombre tiene prioridad. Claves admitidas: %s.\nEl servicio usa la clave nueva desde la siguiente petición, sin reiniciar.", strings.Join(credenciales.Nombres, ", "))}
	validar := func(nombre string) error {
		if !credenciales.Admitido(nombre) {
			return fmt.Errorf(idioma.T("clave no admitida %q; usa una de %s"), nombre, strings.Join(credenciales.Nombres, ", "))
		}
		return nil
	}
	guardar := &cobra.Command{Use: "guardar NOMBRE", Short: idioma.T("Pide la clave sin mostrarla y la guarda"), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validar(args[0]); err != nil {
				return err
			}
			var valor string
			if term.IsTerminal(os.Stdin.Fd()) {
				fmt.Fprintf(cmd.ErrOrStderr(), idioma.T("%s (no se muestra): "), args[0])
				datos, err := term.ReadPassword(os.Stdin.Fd())
				fmt.Fprintln(cmd.ErrOrStderr())
				if err != nil {
					return err
				}
				valor = string(datos)
			} else { // desde una tubería, para scripts: nunca como argumento, que queda en el historial
				linea, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				if err != nil && linea == "" {
					return err
				}
				valor = linea
			}
			if err := credenciales.Guardar(args[0], strings.TrimSpace(valor)); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("%s guardada en %s; el servicio la usa desde la siguiente petición\n"), args[0], credenciales.Almacen())
			return nil
		}}
	borrar := &cobra.Command{Use: "borrar NOMBRE", Short: idioma.T("Elimina la clave del almacén"), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validar(args[0]); err != nil {
				return err
			}
			if err := credenciales.Borrar(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("%s eliminada\n"), args[0])
			return nil
		}}
	listar := &cobra.Command{Use: "listar", Short: idioma.T("Indica de dónde sale cada clave, sin mostrarla"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, nombre := range credenciales.Nombres {
				origen := idioma.T("no configurada")
				if v, err := credenciales.Leer(nombre); err != nil {
					origen = idioma.Tf("el almacén no responde: %s", err)
				} else if v != "" {
					origen = credenciales.Almacen()
				}
				if os.Getenv(nombre) != "" {
					origen = idioma.T("variable de entorno (tiene prioridad)")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-18s %s\n", nombre, origen)
			}
			return nil
		}}
	grupo.AddCommand(guardar, borrar, listar)
	return grupo
}
