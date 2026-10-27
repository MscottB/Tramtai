package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"tramtai/internal/carpetas"
	"tramtai/internal/credenciales"
	"tramtai/internal/idioma"
	"tramtai/internal/servidor"
)

// comandoClaves administra las claves de API en el almacén de credenciales del sistema. Nunca las imprime. Con
// --servidor, las del servicio del sistema: en Linux, cifradas con systemd-creds; en Windows, en el almacén de la
// cuenta del servicio, que las recibe por la API de administración.
func comandoClaves() *cobra.Command {
	grupo := &cobra.Command{Use: "claves", Aliases: []string{"keys"}, Short: idioma.T("Guarda las claves de API en el almacén cifrado del sistema (no en archivos ni variables de entorno)"),
		Long: idioma.Tf("Guarda las claves de API de los proveedores en el almacén cifrado del sistema (Administrador de credenciales\nen Windows, Llavero en macOS, Secret Service en Linux); Tramtai nunca las escribe en sus archivos. Una variable\nde entorno con el mismo nombre tiene prioridad. Claves admitidas: %s.\nEl servicio usa la clave nueva desde la siguiente petición, sin reiniciar.", strings.Join(credenciales.Nombres, ", "))}
	// enServidor dice si son las claves del servicio del sistema, que solo administra root o un administrador: en Linux
	// se cifran en /etc; en Windows, el token de servicio solo lo leen los administradores.
	enServidor := func() (bool, error) {
		if !carpetas.Servidor() {
			return false, nil
		}
		if !esAdministrador() {
			return true, errors.New(idioma.T("las claves del servidor necesitan permisos de administrador: en Linux, con sudo (sudo tramtai-servidor claves …); en Windows, desde una consola de administrador"))
		}
		return true, nil
	}
	guardar := &cobra.Command{Use: "guardar NOMBRE", Aliases: []string{"set"}, Short: idioma.T("Pide la clave sin mostrarla y la guarda"), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := credenciales.Rechazo(args[0]); err != nil {
				return err
			}
			delServidor, err := enServidor()
			if err != nil {
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
			valor = strings.TrimSpace(valor)
			if delServidor {
				err := servidor.GuardarClave(args[0], valor)
				if errors.Is(err, servidor.ErrClaveEnClaro) {
					fmt.Fprintln(cmd.ErrOrStderr(), idioma.Tf("aviso: %s", err.Error()))
				} else if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), idioma.T("Servidor: %s guardada para el servicio del sistema\n"), args[0])
				return nil
			}
			if err := credenciales.Guardar(args[0], valor); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("%s guardada en %s; el servicio la usa desde la siguiente petición\n"), args[0], credenciales.Almacen())
			return nil
		}}
	borrar := &cobra.Command{Use: "borrar NOMBRE", Aliases: []string{"delete"}, Short: idioma.T("Elimina la clave del almacén"), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := credenciales.Rechazo(args[0]); err != nil {
				return err
			}
			delServidor, err := enServidor()
			if err != nil {
				return err
			}
			if delServidor {
				if err := servidor.BorrarClave(args[0]); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), idioma.T("Servidor: %s eliminada\n"), args[0])
				return nil
			}
			if err := credenciales.Borrar(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("%s eliminada\n"), args[0])
			return nil
		}}
	listar := &cobra.Command{Use: "listar", Aliases: []string{"list"}, Short: idioma.T("Indica de dónde sale cada clave, sin mostrarla"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			delServidor, err := enServidor()
			if err != nil {
				return err
			}
			if delServidor { // solo el servicio sabe qué claves recibió (de systemd o de su almacén)
				c, err := conectar()
				if err != nil {
					return err
				}
				ctx, cancelar := contexto()
				defer cancelar()
				lista, err := c.Claves(ctx)
				if err != nil {
					return err
				}
				for _, clave := range lista {
					origen := idioma.T("no configurada")
					if clave.Guardada {
						origen = idioma.T("la tiene el servicio del sistema")
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%-20s %s\n", clave.Nombre, origen)
				}
				return nil
			}
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
