package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/carpetas"
	"tramtai/internal/configuracion"
	"tramtai/internal/idioma"
	"tramtai/internal/servidor"
	"tramtai/internal/terminos"
)

// aceptado dice si lo escrito es «acepto» (o su traducción).
func aceptado(linea string) bool {
	return slices.Contains([]string{"acepto", idioma.T("acepto")}, strings.ToLower(strings.TrimSpace(linea)))
}

// adminAgentesServidor exige permisos de administrador a las órdenes de agentes que cambian el servidor.
func adminAgentesServidor() error {
	if carpetas.Servidor() && !esAdministrador() {
		return errors.New(idioma.T("los agentes del servidor necesitan permisos de administrador: en Linux, con sudo (sudo tramtai-servidor agentes …); en Windows, desde una consola de administrador"))
	}
	return nil
}

// comandoEntrar lanza el inicio de sesión del CLI del agente con la cuenta del servicio, con la terminal conectada: el
// CLI hace su propio flujo y guarda su sesión (spec §5). Solo en modo servidor.
func comandoEntrar() *cobra.Command {
	return &cobra.Command{Use: "entrar <agente>", Aliases: []string{"login"}, Args: cobra.ExactArgs(1),
		Short: idioma.T("Inicia sesión en el CLI del agente con la cuenta del servicio (modo servidor)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !carpetas.Servidor() {
				return errors.New(idioma.T("el inicio de sesión de los agentes es del servidor: usa «tramtai --servidor agentes entrar <agente>» (o tramtai-servidor agentes entrar <agente>)"))
			}
			if err := adminAgentesServidor(); err != nil {
				return err
			}
			f, ok := agentes.Buscar(args[0])
			if !ok || len(f.InicioSesion) == 0 {
				return fmt.Errorf(idioma.T("%s no inicia sesión con una suscripción: usa su clave de API (tramtai-servidor claves guardar)"), args[0])
			}
			c, err := cargarConfiguracion()
			if err != nil {
				return err
			}
			orden, err := servidor.OrdenEntrar(f.Nombre, c.Texto("comandos_agentes."+f.Nombre), f.InicioSesion)
			if err != nil {
				return err
			}
			orden.Stdin, orden.Stdout, orden.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
			return orden.Run()
		}}
}

// comandoSuscripcion activa la suscripción de un agente tras enseñar los riesgos 1 y 2 y pedir «acepto». Con --personal
// (solo en el servidor) enseña además sus condiciones, escribe las tres claves en la global del servidor y lo reinicia;
// sin ella, la suscripción local de siempre, como el botón de Agentes.
func comandoSuscripcion() *cobra.Command {
	var personal bool
	var simultaneas int
	cmd := &cobra.Command{Use: "suscripcion <agente>", Aliases: []string{"subscription"}, Args: cobra.ExactArgs(1),
		Short: idioma.T("Usa tu suscripción con un agente (--personal: en el servidor abierto a la red, con sus condiciones)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			agente := args[0]
			if !agentes.AdmiteSuscripcion(agente) {
				return fmt.Errorf(idioma.T("%s no admite la suscripción"), agente)
			}
			if simultaneas < 1 || simultaneas > 3 {
				return errors.New(idioma.T("--simultaneas: se admite de 1 a 3"))
			}
			if !personal && cmd.Flags().Changed("simultaneas") {
				return errors.New(idioma.T("--simultaneas va con --personal"))
			}
			if personal && !carpetas.Servidor() {
				return errors.New(idioma.T("la suscripción personal es del servidor: usa «tramtai --servidor agentes suscripcion <agente> --personal» (o tramtai-servidor agentes suscripcion <agente> --personal)"))
			}
			if err := adminAgentesServidor(); err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, idioma.Tf("Antes de usar tu suscripción de %s, lee los riesgos:", agentes.NombreVisible(agente)))
			for i, riesgo := range terminos.Riesgos(true)[0:2] {
				fmt.Fprintf(out, "  %d. %s\n", i+1, riesgo)
			}
			if personal {
				f, _ := agentes.Buscar(agente)
				fmt.Fprintf(out, idioma.T("Condiciones de la suscripción personal en el servidor:\n  - solo la usas tú, el titular de la cuenta del proveedor;\n  - no repartes las claves de las APIs que usan este agente;\n  - uso ordinario, como el tuyo en tu propio equipo.\nTérminos del proveedor: %s\nTérminos de Tramtai: %s\n"), f.URLTerminos, terminos.URL)
			}
			leer := lector(cmd)
			if linea, _ := leer(idioma.T("Escribe «acepto» para activarla: "), false); !aceptado(linea) {
				return errors.New(idioma.T("no se aceptaron las condiciones: no se ha cambiado nada"))
			}
			if simultaneas > 1 {
				fmt.Fprintln(out, idioma.Tf("aviso: con %d peticiones a la vez, el uso se aleja del ordinario y sube el riesgo de que el proveedor limite o suspenda tu cuenta", simultaneas))
				if linea, _ := leer(idioma.T("Escribe «acepto» para confirmarlo: "), false); !aceptado(linea) {
					return errors.New(idioma.T("no se confirmó: no se ha cambiado nada"))
				}
			}
			if !personal {
				return suscripcionLocal(cmd, agente)
			}
			ruta, err := configuracion.RutaGlobal()
			if err != nil {
				return err
			}
			for _, v := range []struct {
				clave string
				valor any
			}{{"autenticacion", adaptadores.AutenticacionSuscripcion}, {"suscripcion_personal", true}, {"suscripcion_simultaneas", simultaneas}} {
				if err := configuracion.FijarValor(ruta, configuracion.OrigenGlobal, "adaptadores."+agente+"."+v.clave, v.valor); err != nil {
					return err
				}
			}
			fmt.Fprintln(out, idioma.Tf("Servidor: %s usa tu suscripción personal, con %d peticiones a la vez como mucho; se reinicia el servicio para aplicarlo", agentes.NombreVisible(agente), simultaneas))
			return reiniciarServidor()
		}}
	cmd.Flags().BoolVar(&personal, "personal", false, idioma.T("tu suscripción personal en el servidor abierto a la red (exige TOTP del administrador)"))
	cmd.Flags().IntVar(&simultaneas, "simultaneas", 1, idioma.T("peticiones a la vez con la suscripción personal, de 1 a 3"))
	return cmd
}

// suscripcionLocal pasa el agente a suscripción como el botón de Agentes: por el servicio si está en marcha (lo aplica
// al momento y no edita config.yaml a la vez que él), o en la configuración global si no.
func suscripcionLocal(cmd *cobra.Command, agente string) error {
	if activo, err := conectar(); err == nil {
		ctx, cancelar := contexto()
		defer cancelar()
		if err := activo.CambiarAutenticacion(ctx, agente, adaptadores.AutenticacionSuscripcion); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), idioma.Tf("%s usa tu suscripción", agentes.NombreVisible(agente)))
		return nil
	}
	ruta, err := configuracion.RutaGlobal()
	if err != nil {
		return err
	}
	if err := configuracion.FijarValor(ruta, configuracion.OrigenGlobal, "adaptadores."+agente+".autenticacion", adaptadores.AutenticacionSuscripcion); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), idioma.Tf("%s usa tu suscripción desde que se inicie el servicio (guardado en %s)", agentes.NombreVisible(agente), ruta))
	return nil
}
