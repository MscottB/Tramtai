package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/navegador"
)

func (m *modelo) opcionesInicio() []opcion {
	var ops []opcion
	if m.o.Proyecto != "" {
		ops = append(ops, opcion{idioma.T("Espacio del proyecto actual"), func() tea.Cmd { m.inst = m.o.IDProyecto; m.ir(pInstancia); return nil }})
	}
	return append(ops,
		opcion{idioma.T("Espacio global"), func() tea.Cmd { m.inst = "global"; m.ir(pInstancia); return nil }},
		opcion{idioma.T("Ver todos los espacios"), func() tea.Cmd { m.ir(pTodas); return m.cargarEstado() }},
		opcion{idioma.T("Estado del servicio"), func() tea.Cmd { m.ir(pServicio); return m.cargarEstado() }},
		opcion{idioma.T("Salir"), m.salir},
	)
}

func (m *modelo) opcionesInstancia() []opcion {
	volver := opcion{idioma.T("Volver"), func() tea.Cmd { m.ir(pInicio); return nil }}
	if m.instancia(m.inst) == nil {
		texto := idioma.T("Activar el espacio")
		if m.cli == nil {
			texto += idioma.T(" (inicia el servicio en segundo plano)")
		}
		return []opcion{{texto, m.activar}, volver}
	}
	return []opcion{
		{idioma.T("Ejecutar"), func() tea.Cmd {
			m.ir(pEjecutar)
			m.campo, m.salida, m.resumen = 2, strings.Builder{}, ""
			return m.cargarModelos()
		}},
		{idioma.T("Trabajos"), func() tea.Cmd { m.ir(pTrabajos); return m.cargarTrabajos() }},
		{idioma.T("Puntos de acceso"), func() tea.Cmd { m.ir(pAccesos); return m.cargarEstado() }},
		{idioma.T("Configuración efectiva"), func() tea.Cmd {
			m.ir(pLineas)
			m.titulo = idioma.T("Configuración efectiva")
			return m.cargarConfiguracion()
		}},
		{idioma.T("Agentes y capacidades"), func() tea.Cmd { m.ir(pLineas); m.titulo = idioma.T("Agentes y capacidades"); return m.cargarAgentes() }},
		{idioma.T("Abrir Web UI en el navegador"), m.abrirWeb},
		{idioma.T("Detener el espacio"), func() tea.Cmd {
			return m.confirmar(idioma.Tf("¿Detener %s? Se cancela lo que esté en cola.", m.inst), m.desactivar(m.inst))
		}},
		volver,
	}
}

// activar activa la instancia elegida e inicia el servicio si no está en marcha.
func (m *modelo) activar() tea.Cmd {
	ambito, raiz := "global", ""
	if m.inst != "global" {
		ambito, raiz = "proyecto", m.o.Proyecto
	}
	cli, iniciar := m.cli, m.o.Iniciar
	m.ocupado = idioma.Tf("activando %s…", m.inst)
	return func() tea.Msg {
		var err error
		if cli == nil {
			if cli, err = iniciar(); err != nil {
				return msgAviso{err: err}
			}
		}
		ctx, cancelar := conPlazo(60 * time.Second)
		defer cancelar()
		_, err = cli.Activar(ctx, ambito, raiz)
		return msgAviso{texto: idioma.T("espacio activo"), err: err}
	}
}

// abrirWeb abre la Web UI de la instancia con un código de un solo uso, sin mostrar el token.
func (m *modelo) abrirWeb() tea.Cmd {
	cli, inst := m.cli, m.inst
	return func() tea.Msg {
		ctx, cancelar := conPlazo(10 * time.Second)
		defer cancelar()
		url, err := cli.URLWeb(ctx, inst)
		if err == nil {
			err = navegador.Abrir(url)
		}
		return msgAviso{texto: idioma.T("Web UI abierta en el navegador"), err: err}
	}
}

func (m *modelo) desactivar(id string) tea.Cmd {
	cli := m.cli
	return func() tea.Msg {
		ctx, cancelar := conPlazo(60 * time.Second)
		defer cancelar()
		return msgAviso{texto: idioma.Tf("espacio %s detenido", id), err: cli.Desactivar(ctx, id)}
	}
}

func (m *modelo) cargarConfiguracion() tea.Cmd {
	cli, inst := m.cli, m.inst
	m.ocupado, m.lineas = idioma.T("cargando…"), nil
	return func() tea.Msg {
		ctx, cancelar := conPlazo(10 * time.Second)
		defer cancelar()
		entradas, err := cli.Configuracion(ctx, inst)
		var lineas []string
		for _, e := range entradas {
			lineas = append(lineas, fmt.Sprintf("%-46s %-18v %s", e.Clave, e.Valor, estilo(tenue, e.Origen)))
		}
		return msgLineas{lineas: lineas, err: err}
	}
}

func (m *modelo) cargarAgentes() tea.Cmd {
	cli, inst := m.cli, m.inst
	m.ocupado, m.lineas = idioma.T("detectando agentes…"), nil
	return func() tea.Msg {
		ctx, cancelar := conPlazo(30 * time.Second)
		defer cancelar()
		c, err := cli.Capacidades(ctx, inst)
		var lineas []string
		lista, _ := c["adaptadores"].([]any)
		for _, a := range lista {
			datos, _ := a.(map[string]any)
			capac, _ := datos["capacidades"].(map[string]any)
			lineas = append(lineas, estilo(negrita, fmt.Sprint(datos["agente"]))+idioma.Tf("  versión %v · autenticación %v", datos["version_detectada"], datos["autenticacion"]),
				idioma.Tf("  streaming %v · salida estructurada %v · modelo efectivo %v", capac["streaming"], capac["salida_estructurada"], capac["modelo_efectivo_informado"]),
				idioma.Tf("  términos: %v", datos["terminos"]))
			if p, ok := datos["problema"]; ok {
				lineas = append(lineas, estilo(amarillo, idioma.Tf("  problema: %v", p)))
			}
			lineas = append(lineas, "")
		}
		lineas = append(lineas, idioma.Tf("formato de conversación: %v", c["formato_serializacion"]))
		return msgLineas{lineas: lineas, err: err}
	}
}

func (m *modelo) teclaTodas(tecla string) tea.Cmd {
	var lista []contrato.EstadoInstancia
	if m.estado != nil {
		lista = m.estado.Instancias
	}
	if m.mover(tecla, len(lista)) || len(lista) == 0 {
		return nil
	}
	i := lista[m.cursor]
	switch tecla {
	case "enter", "space":
		m.inst = i.ID
		m.ir(pInstancia)
	case "d":
		return m.confirmar(idioma.Tf("¿Detener %s? Se cancela lo que esté en cola.", i.ID), m.desactivar(i.ID))
	}
	return nil
}

func (m *modelo) teclaServicio(tecla string) tea.Cmd {
	switch tecla {
	case "i":
		if m.cli == nil {
			iniciar := m.o.Iniciar
			m.ocupado = idioma.T("iniciando el servicio…")
			return func() tea.Msg {
				_, err := iniciar()
				return msgAviso{texto: idioma.T("servicio iniciado"), err: err}
			}
		}
	case "d":
		if m.cli != nil {
			cli := m.cli
			return m.confirmar(idioma.T("¿Detener el servicio y todos sus espacios?"), func() tea.Msg {
				ctx, cancelar := conPlazo(30 * time.Second)
				defer cancelar()
				err := cli.Detener(ctx)
				time.Sleep(time.Second)
				return msgAviso{texto: idioma.T("servicio detenido"), err: err}
			})
		}
	case "p":
		if m.cli != nil {
			cli := m.cli
			return func() tea.Msg {
				ctx, cancelar := conPlazo(30 * time.Second)
				defer cancelar()
				_, err := cli.Recargar(ctx)
				return msgAviso{texto: idioma.T("puntos de acceso recargados"), err: err}
			}
		}
	}
	return nil
}
