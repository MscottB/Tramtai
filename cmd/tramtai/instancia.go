package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"tramtai/internal/cliente"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/servicio"
)

func mostrarTokenInstancia(cmd *cobra.Command, ambito string, rotar bool) error {
	_, id, c, err := identificar(ambito)
	if err != nil {
		return err
	}
	datos, err := nucleo.DirDatos()
	if err != nil {
		return err
	}
	if rotar {
		return rotarTokenInstancia(cmd, datos, id, c)
	}
	valor, _, err := servicio.TokenInstancia(datos, id, c)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), valor)
	return nil
}

// rotarTokenInstancia regenera la clave de la instancia: con el servicio en marcha vale al momento; si la instancia
// no está activa se reescribe el archivo y valdrá al activarla.
func rotarTokenInstancia(cmd *cobra.Command, datos, id string, c *nucleo.Configuracion) error {
	if cli, err := cliente.Conectar(datos); err == nil {
		ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelar()
		token, err := cli.RotarClave(ctx, id)
		if err == nil {
			fmt.Fprintln(cmd.OutOrStdout(), token)
			return nil
		}
		if nucleo.CodigoDe(err) != nucleo.CodigoNoEncontrado {
			return err
		}
		// No encontrado vale para una instancia inactiva; si está activa, el servicio en marcha es de una versión sin
		// esta ruta: cambiar solo el archivo mostraría una clave que no usa mientras la anterior sigue valiendo.
		if estado, err := cli.Estado(ctx); err == nil && slices.ContainsFunc(estado.Instancias, func(i servicio.EstadoInstancia) bool { return i.ID == id }) {
			return errors.New(idioma.T("el servicio en marcha es de una versión anterior: detenlo (tramtai servicio detener), vuelve a iniciarlo y repite la orden"))
		}
	}
	if _, origen, err := servicio.TokenInstancia(datos, id, c); err != nil {
		return err
	} else if strings.HasPrefix(origen, "variable ") {
		return errors.New(idioma.Tf("la clave de %s viene de la %s: cámbiala allí", id, origen))
	}
	token, err := servicio.RotarToken(servicio.RutaTokenInstancia(datos, id))
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), token)
	return nil
}

// comandoInstancia crea los comandos de la instancia global o de la del proyecto actual.
func comandoInstancia(ambito string) *cobra.Command {
	corto := map[string]string{"global": idioma.T("Espacio global: texto y archivos sin repositorio (también: tramtai espacio)"),
		"proyecto": idioma.T("Espacio del proyecto actual, .tramtai/config.yaml (también: tramtai espacio)")}[ambito]
	grupo := &cobra.Command{Use: ambito, Short: corto}
	iniciar := &cobra.Command{Use: "iniciar", Short: idioma.T("Activa el espacio (inicia el servicio si hace falta)"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			raiz, _, _, err := identificar(ambito)
			if err != nil {
				return err
			}
			c, err := asegurarServicio(cmd)
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			a, err := c.Activar(ctx, ambito, raiz)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, idioma.T("espacio %s activo en %s\n"), a.Instancia.ID, c.Base)
			fmt.Fprintf(out, "  token:              %s (tramtai %s token)\n", a.Instancia.ArchivoToken, ambito)
			fmt.Fprintf(out, "  OpenAI base_url:    %s/openai/v1\n  Anthropic base_url: %s/anthropic\n", c.Base, c.Base)
			return nil
		}}
	detener := &cobra.Command{Use: "detener", Short: idioma.T("Detiene el espacio: vacía su cola sin afectar a los demás"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, id, _, err := identificar(ambito)
			if err != nil {
				return err
			}
			c, err := conectar()
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			if err := c.Desactivar(ctx, id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("espacio %s detenido\n"), id)
			return nil
		}}
	estado := &cobra.Command{Use: "estado", Short: idioma.T("Muestra si el espacio está activo y sus trabajos"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			raiz, id, _, err := identificar(ambito)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			c, err := conectar()
			if err != nil {
				fmt.Fprintf(out, idioma.T("espacio %s: inactivo (servicio detenido)\n"), id)
				return nil
			}
			ctx, cancelar := contexto()
			defer cancelar()
			e, err := c.Estado(ctx)
			if err != nil {
				return err
			}
			for _, i := range e.Instancias {
				if i.ID == id {
					fmt.Fprintf(out, idioma.T("espacio %s: activo en %s\n  %s\n  token: %s\n"), id, e.Base, resumenTrabajos(i.Trabajos), i.ArchivoToken)
					return nil
				}
			}
			fmt.Fprintf(out, idioma.T("espacio %s: inactivo%s (tramtai %s iniciar)\n"), id, map[bool]string{true: " · " + raiz}[raiz != ""], ambito)
			return nil
		}}
	var rotarToken bool
	token := &cobra.Command{Use: "token", Short: idioma.T("Muestra el token del espacio para configurar clientes"),
		RunE: func(cmd *cobra.Command, _ []string) error { return mostrarTokenInstancia(cmd, ambito, rotarToken) }}
	token.Flags().BoolVar(&rotarToken, "rotar", false, idioma.T("genera un token nuevo; el anterior deja de valer"))
	var enNavegador bool
	abrirWeb := &cobra.Command{Use: "abrir", Short: idioma.T("Abre el espacio en la app de escritorio o, con --navegador, en el navegador"),
		RunE: func(cmd *cobra.Command, _ []string) error { return abrirWebUI(cmd, ambito, enNavegador) }}
	abrirWeb.Flags().BoolVar(&enNavegador, "navegador", false, idioma.T("abre la Web UI en el navegador en lugar de la app de escritorio"))
	grupo.AddCommand(iniciar, detener, estado, token, abrirWeb)
	return grupo
}
