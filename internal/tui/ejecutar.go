package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/documentos"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

func escuchar(flujo chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-flujo
		if !ok {
			return nil
		}
		return msg
	}
}

func (m *modelo) cargarModelos() tea.Cmd {
	cli, inst := m.cli, m.inst
	return func() tea.Msg {
		ctx, cancelar := conPlazo(10 * time.Second)
		defer cancelar()
		lista, err := cli.Modelos(ctx, inst)
		return msgModelos{lista: lista, err: err}
	}
}

// solicitudDesde traduce la selección de modelo del catálogo a la solicitud.
func solicitudDesde(seleccion, texto string) nucleo.SolicitudEjecucion {
	s := nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}}
	switch {
	case seleccion == "" || strings.HasPrefix(seleccion, "("):
	case strings.HasPrefix(seleccion, "perfil/"):
		s.PerfilModelo = strings.TrimPrefix(seleccion, "perfil/")
	default:
		s.Agente, s.Modelo, _ = strings.Cut(seleccion, "/")
	}
	return s
}

func (m *modelo) teclaEjecutar(k tea.KeyPressMsg) tea.Cmd {
	switch tecla := k.String(); tecla {
	case "esc":
		if m.enCurso != "" {
			return m.accionTrabajo(m.enCurso, "cancelar", nil)
		}
		return m.volver()
	case "tab", "down":
		m.campo = (m.campo + 1) % 3
		if m.campo == 2 {
			return m.comprobarConversor()
		}
	case "shift+tab", "up":
		m.campo = (m.campo + 2) % 3
	case "left", "right":
		if m.campo == 0 && len(m.modelos) > 0 {
			paso := map[string]int{"left": -1, "right": 1}[tecla]
			m.iModelo = (m.iModelo + paso + len(m.modelos)) % len(m.modelos)
		}
	case "enter":
		if m.campo < 2 {
			m.campo++
			if m.campo == 2 {
				return m.comprobarConversor()
			}
			return nil
		}
		return m.lanzar()
	case "backspace":
		if m.campo == 1 {
			m.archivos = borrarUltimo(m.archivos)
		} else if m.campo == 2 {
			m.mensaje = borrarUltimo(m.mensaje)
		}
	case "ctrl+p":
		return m.prepararConversor()
	case "ctrl+u":
		if m.campo == 1 {
			m.archivos = ""
		} else if m.campo == 2 {
			m.mensaje = ""
		}
	default:
		if k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
			m.escribir(k.Text)
		}
	}
	return nil
}

// comprobarConversor avisa, al dejar el campo de archivos, si alguno necesita el conversor y no está listo.
func (m *modelo) comprobarConversor() tea.Cmd {
	archivos := strings.FieldsFunc(m.archivos, func(r rune) bool { return r == ',' || r == ';' })
	necesita := slices.ContainsFunc(archivos, func(ruta string) bool {
		ruta = strings.Trim(strings.TrimSpace(ruta), `"`)
		return documentos.NecesitaConversor(filepath.Base(ruta), ruta)
	})
	if !necesita || m.cli == nil {
		return nil
	}
	cli, inst := m.cli, m.inst
	return func() tea.Msg {
		ctx, cancelar := conPlazo(10 * time.Second)
		defer cancelar()
		if e, err := cli.Conversor(ctx, inst); err == nil && e.Estado != "listo" {
			return msgAviso{err: errors.New(idioma.Tf("el conversor de archivos no está listo (%s): ctrl+p lo prepara", e.Estado))}
		}
		return nil
	}
}

// prepararConversor lanza la preparación del conversor de archivos en el servicio.
func (m *modelo) prepararConversor() tea.Cmd {
	cli, inst := m.cli, m.inst
	return func() tea.Msg {
		ctx, cancelar := conPlazo(15 * time.Second)
		defer cancelar()
		e, err := cli.PrepararConversor(ctx, inst)
		return msgAviso{texto: idioma.Tf("preparando el conversor de archivos (%s): sigue en Ajustes → Archivos de la Web UI o con «tramtai conversor estado»", e.Estado), err: err}
	}
}

// rutaPermitida aplica instancia_global.raices_permitidas con la configuración de la carpeta actual, como la CLI.
func rutaPermitida(ruta string) (string, error) {
	raiz := ""
	if dir, err := os.Getwd(); err == nil {
		raiz, _ = nucleo.BuscarProyecto(dir)
	}
	c, err := nucleo.CargarConfiguracion(raiz)
	if err != nil {
		return "", err
	}
	return c.RutaPermitida(ruta)
}

// lanzar sube los archivos, crea el trabajo y sigue sus eventos en vivo.
func (m *modelo) lanzar() tea.Cmd {
	if m.enCurso != "" || m.ocupado != "" { // ocupado: un envío que aún no ha vuelto (dos Enter seguidos)
		return nil
	}
	if strings.TrimSpace(m.mensaje) == "" {
		m.aviso, m.esError = idioma.T("escribe un mensaje"), true
		return nil
	}
	seleccion := ""
	if len(m.modelos) > 0 {
		seleccion = m.modelos[m.iModelo]
	}
	sol := solicitudDesde(seleccion, m.mensaje)
	archivos := strings.FieldsFunc(m.archivos, func(r rune) bool { return r == ',' || r == ';' })
	cli, inst := m.cli, m.inst
	m.salida.Reset()
	m.resumen, m.aviso, m.ocupado = "", "", idioma.T("enviando…")
	return func() tea.Msg {
		ctx, cancelar := conPlazo(10 * time.Minute)
		defer cancelar()
		// Sin conversor listo, un archivo que lo necesita se avisa antes de subir nada (ctrl+p lo prepara).
		necesita := slices.ContainsFunc(archivos, func(ruta string) bool {
			ruta = strings.Trim(strings.TrimSpace(ruta), `"`)
			return documentos.NecesitaConversor(filepath.Base(ruta), ruta)
		})
		if necesita {
			if e, err := cli.Conversor(ctx, inst); err == nil && e.Estado != "listo" {
				return msgCreado{err: errors.New(idioma.Tf("el conversor de archivos no está listo (%s): ctrl+p lo prepara", e.Estado))}
			}
		}
		for _, ruta := range archivos {
			ruta, err := rutaPermitida(strings.Trim(strings.TrimSpace(ruta), `"`))
			if err != nil {
				return msgCreado{err: err}
			}
			carga, err := cli.SubirCarga(ctx, inst, ruta)
			if err != nil {
				return msgCreado{err: err}
			}
			sol.Cargas = append(sol.Cargas, carga.ID)
		}
		t, err := cli.Crear(ctx, inst, sol)
		if err != nil {
			return msgCreado{err: err}
		}
		flujo := make(chan tea.Msg, 256)
		seguir, parar := context.WithCancel(context.Background())
		go func() {
			defer close(flujo)
			err := cli.Eventos(seguir, inst, t.ID, 0, func(_ int, tipo, datos string) error {
				select {
				case flujo <- msgEvento{tipo: tipo, datos: datos}:
					return nil
				case <-seguir.Done():
					return seguir.Err()
				}
			})
			if seguir.Err() != nil {
				return // se dejó de seguir (salir o un trabajo nuevo): nadie espera el final
			}
			ctx, cancelar := conPlazo(10 * time.Second)
			defer cancelar()
			final, err2 := cli.Trabajo(ctx, inst, t.ID)
			if err == nil {
				err = err2
			}
			select {
			case flujo <- msgFin{t: final, err: err}:
			case <-seguir.Done():
			}
		}()
		return msgCreado{t: t, flujo: flujo, parar: parar}
	}
}
