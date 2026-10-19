package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/version"
)

const (
	negrita  = "\x1b[1m"
	tenue    = "\x1b[2m"
	cian     = "\x1b[36m"
	verde    = "\x1b[32m"
	rojo     = "\x1b[31m"
	amarillo = "\x1b[33m"
	fin      = "\x1b[0m"
)

func estilo(e, texto string) string { return e + texto + fin }

// View implementa tea.Model.
func (m *modelo) View() tea.View {
	var b strings.Builder
	b.WriteString(estilo(negrita+cian, "Tramtai") + " " + estilo(tenue, version.Texto()) + "\n\n")
	ayuda := ""
	switch m.pantalla {
	case pInicio:
		ayuda = m.vistaInicio(&b)
	case pInstancia:
		ayuda = m.vistaInstancia(&b)
	case pEjecutar:
		ayuda = m.vistaEjecutar(&b)
	case pTrabajos:
		ayuda = m.vistaTrabajos(&b)
	case pDetalle:
		ayuda = m.vistaDetalle(&b)
	case pAccesos:
		ayuda = m.vistaAccesos(&b)
	case pNuevoPunto:
		ayuda = m.vistaNuevoPunto(&b)
	case pLineas:
		b.WriteString(estilo(negrita, m.titulo+" · "+m.inst) + "\n\n")
		for _, l := range ventana(m.lineas, m.scroll, m.alto-8) {
			b.WriteString(l + "\n")
		}
		ayuda = idioma.T("↑↓ desplazar · Esc volver")
	case pTodas:
		ayuda = m.vistaTodas(&b)
	case pServicio:
		ayuda = m.vistaServicio(&b)
	}
	b.WriteString("\n")
	switch {
	case m.pregunta != "":
		b.WriteString(estilo(amarillo+negrita, m.pregunta+idioma.T(" (s/n)")))
	case m.ocupado != "":
		b.WriteString(estilo(amarillo, m.ocupado))
	case m.aviso != "" && m.esError:
		b.WriteString(estilo(rojo, m.aviso))
	case m.aviso != "":
		b.WriteString(estilo(verde, m.aviso))
	}
	b.WriteString("\n" + estilo(tenue, ayuda+idioma.T(" · Ctrl+C salir")))
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func ventana(lineas []string, desde, alto int) []string {
	alto = max(alto, 5)
	desde = min(desde, max(len(lineas)-1, 0))
	hasta := min(desde+alto, len(lineas))
	return lineas[desde:hasta]
}

func (m *modelo) menu(b *strings.Builder, ops []opcion) {
	for i, op := range ops {
		if i == m.cursor {
			fmt.Fprintf(b, "%s %d. %s\n", estilo(cian, "›"), i+1, estilo(negrita, op.texto))
		} else {
			fmt.Fprintf(b, "  %d. %s\n", i+1, op.texto)
		}
	}
}

func marcaActiva(activa bool) string {
	if activa {
		return estilo(verde, idioma.T("activa"))
	}
	return estilo(tenue, idioma.T("inactiva"))
}

func (m *modelo) vistaInicio(b *strings.Builder) string {
	b.WriteString(idioma.T("Seleccione el ámbito de administración:\n\n"))
	m.menu(b, m.opcionesInicio())
	b.WriteString("\n")
	if m.o.Proyecto != "" {
		b.WriteString(idioma.Tf("Proyecto actual: %s\n", m.o.Proyecto))
	} else {
		b.WriteString(estilo(tenue, idioma.T("Proyecto actual: ninguno (tramtai iniciar en un repositorio)")) + "\n")
	}
	if m.cli != nil {
		b.WriteString(idioma.Tf("Servicio: %s en %s\n", estilo(verde, idioma.T("activo")), m.cli.Base))
	} else {
		b.WriteString(idioma.Tf("Servicio: %s\n", estilo(tenue, idioma.T("inactivo"))))
	}
	if m.o.Proyecto != "" {
		fmt.Fprint(b, idioma.Tf("Instancia proyecto: %s\n", marcaActiva(m.instancia(m.o.IDProyecto) != nil)))
	}
	b.WriteString(idioma.Tf("Instancia global: %s\n", marcaActiva(m.instancia("global") != nil)))
	return idioma.T("↑↓ o número · Enter elegir · r actualizar · q salir")
}

func (m *modelo) vistaInstancia(b *strings.Builder) string {
	inst := m.instancia(m.inst)
	fmt.Fprintf(b, "%s  %s\n", estilo(negrita, idioma.Tf("Instancia %s", m.inst)), marcaActiva(inst != nil))
	if inst != nil {
		if inst.Raiz != "" {
			b.WriteString(idioma.Tf("Raíz: %s\n", inst.Raiz))
		}
		base := "—"
		if m.cli != nil {
			base = m.cli.Base
		}
		fmt.Fprintf(b, "Endpoint: %s  (OpenAI %s/openai/v1 · Anthropic %s/anthropic)\n", base, base, base)
		fmt.Fprintf(b, "Token: %s\n", inst.ArchivoToken)
		b.WriteString(idioma.Tf("Agente predeterminado: %s · perfil: %s\n", valor(inst.Agente), valor(inst.Perfil)))
		b.WriteString(idioma.Tf("Trabajos: %s\n", conteo(inst.Trabajos)))
	}
	b.WriteString("\n")
	m.menu(b, m.opcionesInstancia())
	return idioma.T("↑↓ o número · Enter elegir · Esc volver")
}

// errorPunto muestra el estado de un punto de acceso; «desactivado» es un valor del servicio que el código compara.
func errorPunto(e string) string {
	if e == "desactivado" {
		return idioma.T("desactivado")
	}
	return e
}

func valor(texto string) string {
	if texto == "" {
		return "—"
	}
	return texto
}

func conteo(c map[string]int) string {
	if len(c) == 0 {
		return idioma.T("ninguno")
	}
	var partes []string
	for _, e := range []string{"en_cola", "ejecutando", "completado", "fallido", "cancelado", "interrumpido"} {
		if n := c[e]; n > 0 {
			partes = append(partes, fmt.Sprintf("%s %d", e, n))
		}
	}
	return strings.Join(partes, " · ")
}

func (m *modelo) campoTexto(indice int, etiqueta, texto string) string {
	if m.campo == indice {
		return estilo(cian, "› ") + estilo(negrita, etiqueta) + " " + texto + estilo(cian, "▏")
	}
	return "  " + etiqueta + " " + texto
}

func (m *modelo) vistaEjecutar(b *strings.Builder) string {
	fmt.Fprintf(b, "%s\n\n", estilo(negrita, idioma.Tf("Ejecutar en %s", m.inst)))
	seleccion := "…"
	if len(m.modelos) > 0 {
		seleccion = "◀ " + m.modelos[m.iModelo] + " ▶"
	}
	b.WriteString(m.campoTexto(0, idioma.T("Modelo:  "), seleccion) + "\n")
	b.WriteString(m.campoTexto(1, idioma.T("Archivos:"), m.archivos+estilo(tenue, map[bool]string{true: idioma.T(" (rutas separadas por comas)"), false: ""}[m.archivos == ""])) + "\n")
	b.WriteString(m.campoTexto(2, idioma.T("Mensaje: "), m.mensaje) + "\n")
	b.WriteString(estilo(tenue, strings.Repeat("─", min(m.ancho, 100))) + "\n")
	lineas := strings.Split(m.salida.String(), "\n")
	for _, l := range ventana(lineas, max(len(lineas)-(m.alto-14), 0), m.alto-14) {
		b.WriteString(l + "\n")
	}
	if m.resumen != "" {
		b.WriteString(estilo(tenue, strings.Repeat("─", min(m.ancho, 100))) + "\n" + estilo(cian, m.resumen) + "\n")
	}
	if m.enCurso != "" {
		return idioma.T("Tab cambia de campo · ←→ modelo · Enter ejecuta · Ctrl+U borra el campo · Ctrl+P prepara el conversor · Esc cancela el trabajo")
	}
	return idioma.T("Tab cambia de campo · ←→ modelo · Enter ejecuta · Ctrl+U borra el campo · Ctrl+P prepara el conversor · Esc volver")
}

func (m *modelo) vistaTrabajos(b *strings.Builder) string {
	fmt.Fprintf(b, "%s\n\n", estilo(negrita, idioma.Tf("Trabajos de %s", m.inst)))
	if len(m.trabajos) == 0 {
		b.WriteString(estilo(tenue, idioma.T("sin trabajos")) + "\n")
	}
	desde := max(m.cursor-(m.alto-10)+1, 0)
	for i, t := range m.trabajos {
		if i < desde || i >= desde+max(m.alto-10, 5) {
			continue
		}
		modelo := t.ModeloEfectivo
		if modelo == "" {
			modelo = t.ModeloEnviado
		}
		linea := fmt.Sprintf("%-13s %-32s %-8s %-26s %s", colorEstado(t.Estado), t.ID, t.Solicitud.Agente, valor(modelo), t.CreadoEn.Local().Format("02-01 15:04"))
		if i == m.cursor {
			b.WriteString(estilo(cian, "› ") + linea + "\n")
		} else {
			b.WriteString("  " + linea + "\n")
		}
	}
	return idioma.T("↑↓ · Enter detalle · c cancelar · t reintentar · x eliminar · r actualizar · Esc volver")
}

func colorEstado(e nucleo.Estado) string {
	texto := fmt.Sprintf("%-13s", e)
	switch e {
	case nucleo.Completado:
		return estilo(verde, texto)
	case nucleo.Fallido, nucleo.Interrumpido:
		return estilo(rojo, texto)
	case nucleo.Cancelado:
		return estilo(tenue, texto)
	}
	return estilo(amarillo, texto)
}

func (m *modelo) vistaDetalle(b *strings.Builder) string {
	t := m.detalle
	fmt.Fprintf(b, "%s  %s\n", estilo(negrita, t.ID), colorEstado(t.Estado))
	b.WriteString(idioma.Tf("Agente: %s · modelo enviado %s · efectivo %s\n", valor(t.Solicitud.Agente), valor(t.ModeloEnviado), valor(t.ModeloEfectivo)))
	if t.PuntoAcceso != "" {
		b.WriteString(idioma.Tf("Punto de acceso: %s\n", t.PuntoAcceso))
	}
	fmt.Fprintf(b, "%s\n", resumenTrabajo(t))
	b.WriteString(idioma.Tf("Creado: %s\n", t.CreadoEn.Local().Format("02-01-2006 15:04:05")))
	if t.Disponibilidad == nucleo.SalidaExpirada {
		b.WriteString(estilo(tenue, idioma.T("Resultado expirado por la retención (retencion.horas_resultado)")) + "\n")
	}
	b.WriteString(estilo(tenue, strings.Repeat("─", min(m.ancho, 100))) + "\n")
	for _, l := range ventana(strings.Split(t.Salida, "\n"), m.scroll, m.alto-12) {
		b.WriteString(l + "\n")
	}
	return idioma.T("↑↓ desplazar · c cancelar · t reintentar · x eliminar · r actualizar · Esc volver")
}

func (m *modelo) vistaAccesos(b *strings.Builder) string {
	fmt.Fprintf(b, "%s\n\n", estilo(negrita, idioma.Tf("Puntos de acceso de %s", m.inst)))
	puntos := m.puntosDeInstancia()
	if len(puntos) == 0 {
		b.WriteString(estilo(tenue, idioma.T("ninguno; pulsa n para crear uno")) + "\n")
	}
	for i, p := range puntos {
		estado := estilo(verde, idioma.T("● activo"))
		if p.Error != "" {
			estado = estilo(rojo, "○ "+errorPunto(p.Error))
		}
		modelo := strings.Trim(p.Agente+"/"+p.Modelo, "/")
		linea := fmt.Sprintf("%-18s %-40s %-22s %-20s %s", p.Nombre, valor(p.URL), "["+strings.Join(p.Contratos, ",")+"]", valor(modelo), estado)
		if i == m.cursor {
			b.WriteString(estilo(cian, "› ") + linea + "\n")
		} else {
			b.WriteString("  " + linea + "\n")
		}
	}
	b.WriteString("\n" + estilo(tenue, idioma.T("Alias, modelos permitidos y forzado: tramtai acceso editar <nombre> --alias … --permitidos … --forzar")) + "\n")
	return idioma.T("↑↓ · Enter activar/desactivar · n nuevo · x eliminar · t regenerar clave · Esc volver")
}

func (m *modelo) vistaNuevoPunto(b *strings.Builder) string {
	fmt.Fprintf(b, "%s\n\n", estilo(negrita, idioma.Tf("Nuevo punto de acceso en %s", m.inst)))
	b.WriteString(m.campoTexto(0, idioma.T("Nombre:"), m.nombrePunto) + "\n")
	b.WriteString(m.campoTexto(1, idioma.T("Puerto:"), m.puertoPunto+estilo(tenue, map[bool]string{true: idioma.T(" (vacío = /p/<nombre> en el puerto del servicio)"), false: ""}[m.puertoPunto == ""])) + "\n")
	seleccion := "…"
	if len(m.modelos) > 0 {
		seleccion = "◀ " + m.modelos[m.iModelo] + " ▶"
	}
	b.WriteString(m.campoTexto(2, idioma.T("Modelo:"), seleccion) + "\n")
	return idioma.T("Tab cambia de campo · ←→ modelo · Enter crea · Esc volver")
}

func (m *modelo) vistaTodas(b *strings.Builder) string {
	b.WriteString(estilo(negrita, idioma.T("Todas las instancias")) + "\n\n")
	if m.estado == nil || len(m.estado.Instancias) == 0 {
		b.WriteString(estilo(tenue, idioma.T("ninguna activa")) + "\n")
		return idioma.T("Esc volver")
	}
	for i, inst := range m.estado.Instancias {
		linea := fmt.Sprintf("%-28s %-9s %s", inst.ID, inst.Ambito, conteo(inst.Trabajos))
		if inst.Raiz != "" {
			linea += estilo(tenue, "  "+inst.Raiz)
		}
		if i == m.cursor {
			b.WriteString(estilo(cian, "› ") + linea + "\n")
		} else {
			b.WriteString("  " + linea + "\n")
		}
	}
	return idioma.T("↑↓ · Enter abrir · d desactivar · r actualizar · Esc volver")
}

func (m *modelo) vistaServicio(b *strings.Builder) string {
	b.WriteString(estilo(negrita, idioma.T("Estado del servicio")) + "\n\n")
	if m.cli == nil || m.estado == nil {
		b.WriteString(idioma.Tf("Servicio: %s\n", estilo(tenue, idioma.T("inactivo"))))
		return idioma.T("i iniciar en segundo plano · Esc volver")
	}
	e := m.estado
	b.WriteString(idioma.Tf("Servicio: %s en %s\n", estilo(verde, idioma.T("activo")), e.Base))
	b.WriteString(idioma.Tf("Versión: %s · pid %d · desde %s\n", e.Version, e.PID, e.IniciadoEn.Local().Format("02-01-2006 15:04")))
	b.WriteString(idioma.Tf("Instancias activas: %d\n\nPuntos de acceso:\n", len(e.Instancias)))
	if len(e.Puntos) == 0 {
		b.WriteString(estilo(tenue, "  "+idioma.T("ninguno")) + "\n")
	}
	for _, p := range e.Puntos {
		estado := estilo(verde, idioma.T("activo"))
		if p.Error != "" {
			estado = estilo(rojo, errorPunto(p.Error))
		}
		fmt.Fprintf(b, "  %-18s %-40s %-24s %s\n", p.Nombre, valor(p.URL), p.Instancia, estado)
	}
	for _, err := range e.Errores {
		b.WriteString(estilo(amarillo, idioma.Tf("configuración: %s", err)) + "\n")
	}
	return idioma.T("d detener (pide confirmación) · p recargar puntos · r actualizar · Esc volver")
}
