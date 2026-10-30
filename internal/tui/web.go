package tui

import (
	"context"
	"net/url"
	"os"
	"runtime"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/idioma"
	"tramtai/internal/navegador"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
)

// abrir abre una URL en el navegador del sistema; las pruebas lo cambian para no abrir uno de verdad.
var abrir = navegador.Abrir

// hayNavegador dice si tiene sentido abrir uno: no por SSH y, en Linux, solo con una pantalla (X11 o Wayland). Las
// pruebas lo cambian.
var hayNavegador = func() bool {
	if os.Getenv("SSH_CONNECTION") != "" {
		return false
	}
	return runtime.GOOS != "linux" || os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

// msgWeb es el resultado de W: la dirección con el código de un solo uso (o, en el servidor con administrador, la
// pública, sin código: conContrasena) y si se abrió en el navegador.
type msgWeb struct {
	url           string
	abierta       bool
	conContrasena bool
	err           error
}

// abrirWeb (tecla W) pide al servicio una dirección de la Web UI de un solo uso para el espacio de ahora y la abre en el
// navegador; sin navegador, o si no se pudo abrir, la enseña en una ventana para copiarla a mano. Sin navegador, en el
// servidor donde se entra con la contraseña del administrador (spec §8), basta la dirección pública, sin código.
func (m *modelo) abrirWeb() tea.Cmd {
	if m.pidiendoWeb { // ya hay una petición en vuelo: una segunda W no lanza otra
		return nil
	}
	if m.cli == nil || m.caido != nil {
		return comp.Aviso(idioma.T("El servicio no responde · r reintenta"), comp.Fallo)
	}
	if !hayNavegador() && m.entraConContrasena() {
		base := m.cli.BasePublica()
		return func() tea.Msg { return msgWeb{url: base, conContrasena: true} }
	}
	m.pidiendoWeb = true
	cli, instancia := m.cli, m.instancia
	pedir := func() tea.Msg {
		ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelar()
		direccion, err := cli.URLWeb(ctx, instancia)
		if err != nil {
			return msgWeb{err: err}
		}
		return msgWeb{url: direccion, abierta: hayNavegador() && abrir(direccion) == nil}
	}
	return tea.Batch(comp.Aviso(idioma.T("Pidiendo el enlace de la Web UI…"), comp.Info), pedir)
}

// entraConContrasena dice si en este servidor se entra con la contraseña del administrador: hay administrador y llega
// HTTPS (TLS propio o un proxy de confianza); sin HTTPS, la contraseña no se acepta y hace falta el código.
func (m *modelo) entraConContrasena() bool {
	s := m.servidor
	return s != nil && s.Administrador && (s.TLS != "ninguno" && s.TLS != "" || len(s.Confiables) > 0)
}

// webAbierta atiende el resultado de W: un aviso si se abrió o falló, o la ventana con la dirección y el código.
func (m *modelo) webAbierta(msg msgWeb) tea.Cmd {
	m.pidiendoWeb = false
	switch {
	case msg.err != nil:
		return comp.Aviso(nucleo.MensajeDe(msg.err), comp.Fallo)
	case msg.abierta:
		return comp.Aviso(idioma.T("Abierto en el navegador"), comp.Info)
	}
	if m.pregunta != nil { // ya hay otra pregunta abierta: no se pisa; la dirección queda en la línea de estado
		return comp.Aviso(msg.url, comp.Info)
	}
	if msg.conContrasena {
		m.pregunta = &comp.Confirmacion{Texto: idioma.T("Abre esta dirección en un navegador y entra con la contraseña del administrador:") + "\n\n" + msg.url, Leer: true}
		return nil
	}
	texto := idioma.T("Abre esta dirección en un navegador; vale un minuto y una sola vez:") + "\n\n" + msg.url
	if u, err := url.Parse(msg.url); err == nil && u.Query().Get("codigo") != "" {
		texto += "\n\n" + idioma.T("Código") + ": " + u.Query().Get("codigo")
	}
	// Tras un proxy de confianza, el enlace con código va al oyente local (R35): desde fuera solo se entra con contraseña.
	if s := m.servidor; s != nil && !s.Administrador && len(s.Confiables) > 0 {
		texto += "\n\n" + idioma.T("Para entrar desde otro equipo, crea antes la contraseña del administrador: tramtai-servidor administrador contraseña")
	}
	m.pregunta = &comp.Confirmacion{Texto: texto, Leer: true} // sin pregunta abierta, saliendo y cambiarA ya están vacíos
	return nil
}
