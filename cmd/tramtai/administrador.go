package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"tramtai/internal/agentes"
	"tramtai/internal/carpetas"
	"tramtai/internal/configuracion"
	"tramtai/internal/idioma"
	"tramtai/internal/servicio"
	"tramtai/internal/servidor"
	"tramtai/internal/totp"
)

// Lo que «tramtai administrador» toca fuera de datos/; las pruebas lo sustituyen. En Linux el secreto va a systemd-creds
// (y se reinicia el servicio); en Windows, al almacén del servicio por su API de administración.
var (
	fijarTOTPServidor  = servidor.GuardarTOTP
	quitarTOTPServidor = servidor.BorrarTOTP
	generarTOTP        = totp.Generar
)

// datosAdministrador es la carpeta datos/ del servidor, si la orden va en modo servidor y como administrador.
func datosAdministrador() (string, error) {
	if !carpetas.Servidor() {
		return "", errors.New(idioma.T("el administrador es del servidor: usa «tramtai --servidor administrador …» (o tramtai-servidor administrador …)"))
	}
	if !esAdministrador() {
		return "", errors.New(idioma.T("el administrador del servidor necesita permisos de administrador: en Linux, con sudo (sudo tramtai-servidor administrador …); en Windows, desde una consola de administrador"))
	}
	return dirDatos()
}

// lector pide datos por la entrada: lo secreto, sin eco en una terminal; desde una tubería (scripts y pruebas), línea a
// línea.
func lector(cmd *cobra.Command) func(pregunta string, secreto bool) (string, error) {
	entrada := bufio.NewReader(cmd.InOrStdin())
	return func(pregunta string, secreto bool) (string, error) {
		fmt.Fprint(cmd.ErrOrStderr(), pregunta)
		if secreto && term.IsTerminal(os.Stdin.Fd()) {
			datos, err := term.ReadPassword(os.Stdin.Fd())
			fmt.Fprintln(cmd.ErrOrStderr())
			return string(datos), err
		}
		linea, err := entrada.ReadString('\n')
		if err != nil && linea == "" {
			return "", err
		}
		return strings.TrimRight(linea, "\r\n"), nil
	}
}

// comandoAdministrador es el administrador de la Web UI remota del servidor: su contraseña y su TOTP.
func comandoAdministrador() *cobra.Command {
	grupo := &cobra.Command{Use: "administrador", Aliases: []string{"admin"}, Short: idioma.T("El administrador de la Web UI remota del servidor: contraseña y TOTP"),
		Long: idioma.T("Con contraseña (y TOTP, opcional) la Web UI del servidor se abre desde cualquier navegador, solo por HTTPS.\nSin contraseña solo se abre con el código de un solo uso de «tramtai --servidor abrir».")}
	contrasena := &cobra.Command{Use: "contraseña", Aliases: []string{"password"}, Short: idioma.T("Pone o cambia la contraseña del administrador (la pide dos veces, sin mostrarla)"),
		Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			datos, err := datosAdministrador()
			if err != nil {
				return err
			}
			leer := lector(cmd)
			nueva, err := leer(idioma.T("Contraseña nueva, de 12 caracteres o más (no se muestra): "), true)
			if err != nil {
				return err
			}
			repetida, err := leer(idioma.T("Repítela: "), true)
			if err != nil {
				return err
			}
			if nueva != repetida {
				return errors.New(idioma.T("las contraseñas no coinciden: no se ha cambiado nada"))
			}
			if err := servicio.FijarContrasena(datos, nueva); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), idioma.T("Servidor: contraseña del administrador guardada; las sesiones abiertas con la anterior se cierran"))
			return nil
		}}
	grupoTOTP := &cobra.Command{Use: "totp", Short: idioma.T("Activa o quita el segundo factor (TOTP) del administrador")}
	activar := &cobra.Command{Use: "activar", Aliases: []string{"enable"}, Short: idioma.T("Activa el TOTP: enseña el secreto y lo guarda al confirmarlo con un código"),
		Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			datos, err := datosAdministrador()
			if err != nil {
				return err
			}
			if _, err := os.Stat(filepath.Join(datos, "administrador")); err != nil {
				return errors.New(idioma.T("primero pon la contraseña del administrador: tramtai --servidor administrador contraseña"))
			}
			secreto, err := generarTOTP()
			if err != nil {
				return err
			}
			var dominios []string // los de la Web UI: normalizados como al arrancar
			if c, err := configuracion.CargarConfiguracion("", agentes.Internos()); err == nil {
				if red, err := entradaServidor(c); err == nil {
					dominios = red.Dominios
				}
			}
			cuenta := totp.Cuenta(dominios)
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("Añade este secreto a tu app de autenticación:\n  %s\no abre este enlace en ella:\n  %s\n"), secreto, totp.URI(secreto, cuenta))
			codigo, err := lector(cmd)(idioma.T("Escribe el código de 6 dígitos que muestra la app: "), false)
			if err != nil {
				return err
			}
			if _, ok := totp.Valido(secreto, strings.TrimSpace(codigo), time.Now(), 0); !ok {
				return errors.New(idioma.T("el código no coincide: no se ha activado nada (revisa que el equipo y el móvil estén en hora y vuelve a probar)"))
			}
			if err := fijarTOTPServidor(secreto); errors.Is(err, servidor.ErrClaveEnClaro) {
				fmt.Fprintln(cmd.ErrOrStderr(), idioma.Tf("aviso: %s", err.Error()))
			} else if err != nil {
				return err
			}
			codigos, err := servicio.NuevosCodigosRecuperacion(datos)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("Servidor: TOTP activado. Guarda estos códigos de recuperación en un sitio seguro; cada uno entra una vez si pierdes el móvil:\n  %s\n"),
				strings.Join(codigos, "\n  "))
			return nil
		}}
	quitar := &cobra.Command{Use: "quitar", Aliases: []string{"disable"}, Short: idioma.T("Quita el TOTP (no se permite mientras haya suscripción personal)"),
		Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			datos, err := datosAdministrador()
			if err != nil {
				return err
			}
			c, err := configuracion.CargarConfiguracion("", agentes.Internos())
			if err != nil {
				return err
			}
			if err := servicio.ComprobarQuitarTOTP(c); err != nil {
				return err
			}
			if err := quitarTOTPServidor(); err != nil {
				return err
			}
			if err := servicio.BorrarCodigosRecuperacion(datos); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), idioma.T("Servidor: TOTP quitado; basta la contraseña para entrar"))
			return nil
		}}
	grupoTOTP.AddCommand(activar, quitar)
	grupo.AddCommand(contrasena, grupoTOTP)
	return grupo
}
