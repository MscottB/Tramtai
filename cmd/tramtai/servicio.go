package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"tramtai/internal/adaptadores"
	"tramtai/internal/api"
	"tramtai/internal/documentos"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/planificador"
	"tramtai/internal/version"
)

func comandoServicio() *cobra.Command {
	servicio := &cobra.Command{Use: "servicio", Short: "Servicio HTTP local: API nativa y fachadas OpenAI y Anthropic"}
	iniciar := &cobra.Command{Use: "iniciar", Short: "Inicia el servicio en primer plano (Ctrl+C lo detiene)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := cargarConfiguracion()
			if err != nil {
				return err
			}
			return iniciarServicio(cmd, c)
		}}
	token := &cobra.Command{Use: "token", Short: "Muestra el token de la instancia global para configurar clientes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := cargarConfiguracion()
			if err != nil {
				return err
			}
			datos, err := dirDatos()
			if err != nil {
				return err
			}
			valor, _, err := tokenGlobal(c, datos)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), valor)
			return nil
		}}
	servicio.AddCommand(iniciar, token)
	return servicio
}

// dirDatos es el directorio de datos locales del usuario (sección 12.1).
func dirDatos() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	datos := filepath.Join(dir, "tramtai", "datos")
	return datos, os.MkdirAll(datos, 0o700)
}

// tokenGlobal toma el token de la variable configurada o, si no existe, del archivo token-global del directorio
// de datos, que se crea con un valor aleatorio la primera vez (sección 14.1).
func tokenGlobal(c *nucleo.Configuracion, datos string) (valor, origen string, err error) {
	variable := c.Texto("instancia_global.token_variable_entorno")
	if v := os.Getenv(variable); variable != "" && v != "" {
		return v, "variable " + variable, nil
	}
	ruta := filepath.Join(datos, "token-global")
	if contenido, err := os.ReadFile(ruta); err == nil && len(strings.TrimSpace(string(contenido))) >= 32 {
		return strings.TrimSpace(string(contenido)), ruta, nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	valor = "tt_" + hex.EncodeToString(b)
	return valor, ruta, os.WriteFile(ruta, []byte(valor+"\n"), 0o600)
}

func esLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// iniciarServicio arranca la instancia global y el servidor HTTP en primer plano.
// shortcut: sin descriptor ni segundo plano; el ciclo de vida completo (sección 14.2) llega en M5.
func iniciarServicio(cmd *cobra.Command, c *nucleo.Configuracion) error {
	host, puerto := c.Texto("servicio.host"), c.Entero("servicio.puerto")
	remoto := !esLoopback(host)
	if remoto {
		if !c.Booleano("servicio.permitir_remoto") {
			return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, "servicio.host "+host+" no es loopback: activa servicio.permitir_remoto (mejor, usa un túnel SSH)")
		}
		if !c.Booleano("servicio.requerir_token") {
			return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, "el acceso remoto exige servicio.requerir_token")
		}
		for _, agente := range []string{"claude", "codex"} {
			if c.Texto("adaptadores."+agente+".autenticacion") == adaptadores.AutenticacionSuscripcion {
				return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, "el acceso remoto no se permite con "+agente+" en modo suscripción (sección 2.4)")
			}
		}
	}
	datos, err := dirDatos()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(datos, "instancias"), 0o700); err != nil {
		return err
	}
	alm, err := persistencia.Abrir(filepath.Join(datos, "instancias", "global.db"))
	if err != nil {
		return err
	}
	defer alm.Cerrar()
	cargas, err := documentos.AbrirCargas(filepath.Join(datos, "cargas", "global"),
		time.Duration(c.Entero("retencion.horas_carga_huerfana"))*time.Hour, int64(c.Entero("conversion.maximo_carga_mb"))<<20)
	if err != nil {
		return err
	}
	registro, limites := adaptadores.Registro(c)
	opc := planificador.OpcionesDesde(c, "global")
	opc.Preparador = documentos.Preparador{Cargas: cargas, Conversor: documentos.Desde(c)}
	p := planificador.Nuevo(alm, registro, planificador.NuevosSemaforos(limites), opc)

	var token, origenToken string
	if c.Booleano("servicio.requerir_token") {
		if token, origenToken, err = tokenGlobal(c, datos); err != nil {
			return err
		}
	}
	hosts := []string{"127.0.0.1", "localhost", "::1"}
	if remoto {
		hosts = append(hosts, strings.ToLower(host))
	}
	direccion := net.JoinHostPort(host, strconv.Itoa(puerto))
	oyente, err := net.Listen("tcp", direccion)
	if err != nil {
		return fmt.Errorf("no se pudo escuchar en %s (¿el puerto está en uso?): %w", direccion, err)
	}
	if err := p.Iniciar(cmd.Context()); err != nil {
		oyente.Close()
		return err
	}
	s := &api.Servidor{Planificador: p, Cargas: cargas, Registro: registro, Config: c, Token: token, Hosts: hosts}
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}

	base := "http://" + direccion
	salida := cmd.ErrOrStderr()
	fmt.Fprintf(salida, "tramtai %s escuchando en %s\n", version.Texto(), base)
	fmt.Fprintf(salida, "  API nativa:         %s/api/v1\n  OpenAI base_url:    %s/openai/v1\n  Anthropic base_url: %s/anthropic\n", base, base, base)
	if token != "" {
		fmt.Fprintf(salida, "  token:              %s (tramtai servicio token)\n", origenToken)
	} else {
		fmt.Fprintln(salida, "  token:              no requerido (servicio.requerir_token: false)")
	}
	fmt.Fprintln(salida, "Ctrl+C para detener.")

	ctx, parar := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer parar()
	fallo := make(chan error, 1)
	go func() { fallo <- srv.Serve(oyente) }()
	select {
	case err = <-fallo:
	case <-ctx.Done():
		fmt.Fprintln(salida, "deteniendo…")
	}
	cierre, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if srv.Shutdown(cierre) != nil {
		srv.Close() // los flujos SSE abiertos no terminan solos
	}
	p.Detener()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
