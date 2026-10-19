package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

func resumenTrabajo(t nucleo.Trabajo) string {
	modelo := t.ModeloEfectivo
	if modelo == "" {
		modelo = idioma.T("no verificable")
	}
	texto := idioma.Tf("%s · modelo %s · tokens %d/%d", t.Estado, modelo, t.Uso.TokensEntrada, t.Uso.TokensSalida)
	if t.Uso.CostoUSD > 0 {
		texto += fmt.Sprintf(" · US$ %.4f", t.Uso.CostoUSD)
	}
	if t.CodigoError != "" {
		texto += " · " + string(t.CodigoError) + ": " + t.MensajeError
	}
	return texto
}

func (m *modelo) cargarTrabajos() tea.Cmd {
	cli, inst := m.cli, m.inst
	m.ocupado = idioma.T("cargando trabajos…")
	return func() tea.Msg {
		ctx, cancelar := conPlazo(10 * time.Second)
		defer cancelar()
		lista, err := cli.Trabajos(ctx, inst, 50)
		return msgTrabajos{lista: lista, err: err}
	}
}

func (m *modelo) cargarDetalle(id string) tea.Cmd {
	cli, inst := m.cli, m.inst
	return func() tea.Msg {
		ctx, cancelar := conPlazo(10 * time.Second)
		defer cancelar()
		t, err := cli.Trabajo(ctx, inst, id)
		return msgDetalle{t: t, err: err}
	}
}

func (m *modelo) accionTrabajo(id, accion string, siguiente tea.Cmd) tea.Cmd {
	cli, inst := m.cli, m.inst
	return func() tea.Msg {
		ctx, cancelar := conPlazo(30 * time.Second)
		defer cancelar()
		var err error
		if accion == "eliminar" {
			err = cli.Eliminar(ctx, inst, id)
		} else {
			_, err = cli.Accion(ctx, inst, id, accion)
		}
		etiqueta := map[string]string{"cancelar": idioma.T("cancelar"), "reintentar": idioma.T("reintentar"), "eliminar": idioma.T("eliminar")}[accion]
		return msgAviso{texto: etiqueta + ": " + id, err: err, siguiente: siguiente}
	}
}

func (m *modelo) teclaTrabajos(tecla string) tea.Cmd {
	if m.mover(tecla, len(m.trabajos)) || len(m.trabajos) == 0 {
		return nil
	}
	t := m.trabajos[m.cursor]
	switch tecla {
	case "enter", "space":
		return m.cargarDetalle(t.ID)
	case "c":
		return m.accionTrabajo(t.ID, "cancelar", m.cargarTrabajos())
	case "t":
		return m.accionTrabajo(t.ID, "reintentar", m.cargarTrabajos())
	case "x":
		return m.confirmar(idioma.Tf("¿Eliminar %s?", t.ID), m.accionTrabajo(t.ID, "eliminar", m.cargarTrabajos()))
	}
	return nil
}

func (m *modelo) teclaDetalle(tecla string) tea.Cmd {
	id := m.detalle.ID
	switch tecla {
	case "c":
		return m.accionTrabajo(id, "cancelar", m.cargarDetalle(id))
	case "t":
		return m.accionTrabajo(id, "reintentar", nil)
	case "x":
		return m.confirmar(idioma.Tf("¿Eliminar %s?", id), m.accionTrabajo(id, "eliminar", func() tea.Msg { return msgTrabajos{} }))
	case "r":
		return m.cargarDetalle(id)
	}
	m.desplazar(tecla, len(strings.Split(m.detalle.Salida, "\n")))
	return nil
}
