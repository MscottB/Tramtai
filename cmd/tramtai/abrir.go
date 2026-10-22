package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"tramtai/internal/autoarranque"
	"tramtai/internal/carpetas"
	"tramtai/internal/cliente"
	"tramtai/internal/idioma"
	"tramtai/internal/navegador"
	"tramtai/internal/servicio"
)

// abrirCuandoResponda espera a que el servicio en primer plano acepte conexiones y abre la interfaz global.
func abrirCuandoResponda(cmd *cobra.Command, enNavegador bool) {
	datos, err := carpetas.DirDatos()
	if err != nil {
		return
	}
	for range 100 {
		if c, err := cliente.Conectar(datos); err == nil {
			if err := abrirInterfaz(cmd, c, "global", "", enNavegador); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), idioma.T("no se pudo abrir la interfaz:"), err)
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// abrirWebUI inicia el servicio si hace falta, deja claro que sigue en marcha en segundo plano y abre la
// interfaz: la app de escritorio si está junto a tramtai, o la Web UI en el navegador con un enlace de un solo uso.
// (en Windows y Linux el mismo usuario entra igualmente sin él).
func abrirWebUI(cmd *cobra.Command, ambito string, enNavegador bool) error {
	raiz, _, _, err := identificar(ambito)
	if err != nil {
		return err
	}
	datos, err := carpetas.DirDatos()
	if err != nil {
		return err
	}
	_, errPrevio := cliente.Conectar(datos)
	c, err := asegurarServicio(cmd)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if errPrevio == nil {
		fmt.Fprintf(out, idioma.T("Tramtai ya estaba en marcha en %s.\n"), c.Base)
	} else {
		fmt.Fprintf(out, idioma.T("Tramtai se inició en segundo plano en %s.\n"), c.Base)
	}
	fmt.Fprintln(out, idioma.T("Sigue funcionando aunque cierres esta terminal o la ventana."))
	if err := abrirInterfaz(cmd, c, ambito, raiz, enNavegador); err != nil {
		return err
	}
	fmt.Fprint(out, idioma.T(`
  Ver lo que procesa:  tramtai servicio registro -f
  Ver su estado:       tramtai servicio estado
  Detenerlo:           tramtai servicio detener
`))
	return nil
}

// abrirInterfaz abre la app de escritorio (si está junto a tramtai y no se pidió el navegador) o la Web UI en el
// navegador, ya dentro y en la instancia del ámbito.
func abrirInterfaz(cmd *cobra.Command, c *cliente.Cliente, ambito, raiz string, enNavegador bool) error {
	out := cmd.OutOrStdout()
	if escritorio, err := autoarranque.Escritorio(); err == nil && !enNavegador {
		args := []string{}
		if ambito == "proyecto" {
			args = append(args, "-proyecto", raiz)
		}
		app := exec.Command(escritorio, args...)
		if err := app.Start(); err == nil {
			_ = app.Process.Release()
			fmt.Fprintln(out, idioma.T("Abriendo la aplicación de escritorio… (con --navegador se abre en el navegador)"))
			return nil
		}
	}
	ctx, cancelar := contexto()
	defer cancelar()
	a, err := activarEspacio(ctx, cmd, c, ambito, raiz)
	if err != nil {
		return err
	}
	url, err := c.URLWeb(ctx, a.Instancia.ID)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, idioma.T("Abriendo la Web UI en el navegador…"))
	return navegador.Abrir(url)
}

func comandoIniciar() *cobra.Command {
	return &cobra.Command{Use: "iniciar", Short: idioma.T("Prepara el directorio actual como proyecto de Tramtai (.tramtai/ con un perfil de ejemplo)"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			ruta := filepath.Join(dir, ".tramtai", "config.yaml")
			if _, err := os.Stat(ruta); err == nil {
				return fmt.Errorf(idioma.T("%s ya existe"), ruta)
			}
			if err := servicio.PrepararProyecto(dir); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("creado %s\nsiguiente: tramtai proyecto iniciar\n"), ruta)
			return nil
		}}
}
