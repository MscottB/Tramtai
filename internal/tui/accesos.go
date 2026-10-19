package tui

import (
	"context"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
)

func (m *modelo) puntosDeInstancia() []contrato.EstadoPunto {
	var lista []contrato.EstadoPunto
	if m.estado != nil {
		for _, p := range m.estado.Puntos {
			if p.Instancia == m.inst {
				lista = append(lista, p)
			}
		}
	}
	return lista
}

// conServicio ejecuta f con el cliente y un plazo, y lo convierte en el aviso de la pantalla: las APIs se editan por
// la API del servicio, con sus reglas, como en la Web UI y la CLI.
func (m *modelo) conServicio(f func(ctx context.Context, cli *cliente.Cliente) (string, error)) tea.Cmd {
	cli := m.cli
	return func() tea.Msg {
		ctx, cancelar := conPlazo(30 * time.Second)
		defer cancelar()
		texto, err := f(ctx, cli)
		return msgAviso{texto: texto, err: err}
	}
}

func (m *modelo) teclaAccesos(tecla string) tea.Cmd {
	puntos := m.puntosDeInstancia()
	if tecla == "n" {
		m.ir(pNuevoPunto)
		m.campo, m.nombrePunto, m.puertoPunto = 0, "", ""
		return m.cargarModelos()
	}
	if m.mover(tecla, len(puntos)) || len(puntos) == 0 {
		return nil
	}
	p := puntos[m.cursor]
	switch tecla {
	case "enter", "space":
		activo := !(p.Activo || p.Error != "desactivado")
		return m.conServicio(func(ctx context.Context, cli *cliente.Cliente) (string, error) {
			return map[bool]string{true: idioma.T("punto activado"), false: idioma.T("punto desactivado")}[activo],
				cli.ActivarPunto(ctx, p.Instancia, p.Nombre, activo)
		})
	case "x":
		return m.confirmar(idioma.Tf("¿Eliminar el punto de acceso %s?", p.Nombre),
			m.conServicio(func(ctx context.Context, cli *cliente.Cliente) (string, error) {
				// El servicio borra el punto, su clave y su consumo: otro con este nombre no hereda nada.
				return idioma.T("punto eliminado"), cli.EliminarPunto(ctx, p.Instancia, p.Nombre)
			}))
	case "t":
		return m.confirmar(idioma.Tf("¿Regenerar la clave de %s? La anterior deja de valer.", p.Nombre), m.regenerarClave(p.Instancia, p.Nombre))
	}
	return nil
}

// regenerarClave crea una clave nueva para el punto y la muestra esta única vez: solo se guarda su huella.
func (m *modelo) regenerarClave(instancia, nombre string) tea.Cmd {
	return m.conServicio(func(ctx context.Context, cli *cliente.Cliente) (string, error) {
		token, err := cli.RotarTokenPunto(ctx, instancia, nombre)
		return idioma.Tf("clave nueva de %s (guárdala: no se volverá a mostrar): %s", nombre, token), err
	})
}

func (m *modelo) teclaNuevoPunto(k tea.KeyPressMsg) tea.Cmd {
	switch tecla := k.String(); tecla {
	case "esc":
		return m.volver()
	case "tab", "down":
		m.campo = (m.campo + 1) % 3
	case "shift+tab", "up":
		m.campo = (m.campo + 2) % 3
	case "left", "right":
		if m.campo == 2 && len(m.modelos) > 0 {
			paso := map[string]int{"left": -1, "right": 1}[tecla]
			m.iModelo = (m.iModelo + paso + len(m.modelos)) % len(m.modelos)
		}
	case "backspace":
		if m.campo == 0 {
			m.nombrePunto = borrarUltimo(m.nombrePunto)
		} else if m.campo == 1 {
			m.puertoPunto = borrarUltimo(m.puertoPunto)
		}
	case "enter":
		if m.campo < 2 {
			m.campo++
			return nil
		}
		return m.crearPunto()
	default:
		if k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
			m.escribir(k.Text)
		}
	}
	return nil
}

// crearPunto crea la API en la instancia de la pantalla: el servicio la guarda donde la Web UI (la configuración
// global para la global, .tramtai/local.yaml para un proyecto) y devuelve su clave, que se muestra esta única vez.
func (m *modelo) crearPunto() tea.Cmd {
	p := contrato.ConfigPunto{Nombre: m.nombrePunto}
	if m.puertoPunto != "" {
		puerto, err := strconv.Atoi(m.puertoPunto)
		if err != nil {
			m.aviso, m.esError = idioma.T("el puerto debe ser un número"), true
			return nil
		}
		p.Puerto = puerto
	}
	if len(m.modelos) > 0 {
		s := solicitudDesde(m.modelos[m.iModelo], "")
		p.Agente, p.Modelo, p.PerfilModelo = s.Agente, s.Modelo, s.PerfilModelo
	}
	m.ir(pAccesos)
	instancia := m.inst
	return m.conServicio(func(ctx context.Context, cli *cliente.Cliente) (string, error) {
		_, token, err := cli.CrearPunto(ctx, instancia, p)
		return idioma.Tf("clave nueva de %s (guárdala: no se volverá a mostrar): %s", p.Nombre, token), err
	})
}
