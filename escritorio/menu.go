package main

import (
	"strings"

	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
)

// accion es lo que hace una entrada del menú de la bandeja al pulsarla; main la conecta con Wails.
type accion int

const (
	accionNinguna accion = iota
	accionMostrar
	accionAcercaDe
	accionActualizar
	accionBuscarActualizaciones
	accionAlternarApi // activa o desactiva entrada.punto según entrada.habilitada
	accionIrMisApis
	accionCopiarDireccion
	accionIniciar
	accionReiniciar
	accionAvisarDetener
	accionAutoarranque
	accionSalir
	accionDetenerYSalir
	accionAvisarSalir
)

// entrada es una línea del menú de la bandeja. icono es el nombre de un icono de iconosMenu, o «verde» y «rojo» para
// los círculos de estado.
type entrada struct {
	texto      string
	icono      string
	accion     accion
	separador  bool
	casilla    bool // con marca; marcada dice si está puesta
	marcada    bool
	submenu    []entrada
	punto      contrato.EstadoPunto // la API de accionAlternarApi
	habilitada bool
}

// estadoMenu es lo que decide qué sale en el menú. base es la dirección del servicio en marcha, vacía si está
// detenido; nueva, la versión más nueva si la hay.
type estadoMenu struct {
	base                     string
	puntos                   []contrato.EstadoPunto
	proyectos                map[string]string // id del espacio de proyecto → nombre de su carpeta
	conWindows, sinPreguntar bool
	nueva                    string
	autoarranque             bool // si el sistema permite iniciar con la sesión
	windows                  bool
}

// entradasMenu calcula el menú de la bandeja sin tocar Wails, para poder probarlo.
func entradasMenu(e estadoMenu) []entrada {
	enMarcha := e.base != ""
	var menu []entrada
	add := func(n entrada) { menu = append(menu, n) }
	separador := entrada{separador: true}
	if !enMarcha {
		add(entrada{texto: idioma.T("Servicio detenido"), icono: "rojo", accion: accionMostrar})
	} else {
		add(entrada{texto: idioma.Tf("Servicio en marcha · %s", strings.TrimPrefix(e.base, "http://")), icono: "verde", accion: accionMostrar})
	}
	// «Acerca de» (donde se ve la versión), y si hay una más nueva (la opción para instalarla va justo debajo).
	if e.nueva != "" {
		add(entrada{texto: idioma.Tf("Acerca de Tramtai · hay una versión nueva: %s", e.nueva), accion: accionAcercaDe})
	} else {
		add(entrada{texto: idioma.T("Acerca de Tramtai"), accion: accionAcercaDe})
	}
	add(separador)
	add(entrada{texto: idioma.T("Abrir Tramtai"), icono: "abrir", accion: accionMostrar})
	if e.nueva != "" {
		// La ventana pide confirmación y muestra el avance con el mismo diálogo que la Web UI.
		add(entrada{texto: idioma.Tf("Instalar la versión %s…", e.nueva), icono: "reiniciar", accion: accionActualizar})
	} else if enMarcha {
		// Consulta GitHub en el momento; la ventana muestra el resultado o el diálogo de la versión nueva.
		add(entrada{texto: idioma.T("Buscar actualizaciones"), icono: "reiniciar", accion: accionBuscarActualizaciones})
	}
	if enMarcha {
		activas := 0
		for _, p := range e.puntos {
			if p.Activo {
				activas++
			}
		}
		var apis []entrada
		visibles, resto := menuApis(e.puntos, maximoApisMenu)
		for _, p := range visibles {
			habilitada := p.Error != "desactivado"
			etiqueta := p.Nombre
			for _, extra := range []string{p.Grupo, e.proyectos[p.Instancia]} {
				if extra != "" {
					etiqueta += " · " + extra
				}
			}
			if habilitada && !p.Activo {
				etiqueta += idioma.T(" (con error)")
			}
			circ := "rojo"
			if p.Activo {
				circ = "verde"
			}
			// Clic: desactivarla si está habilitada, activarla si no. Se guarda y se aplica sin reiniciar.
			apis = append(apis, entrada{texto: etiqueta, icono: circ, accion: accionAlternarApi, punto: p, habilitada: habilitada})
		}
		if resto > 0 {
			apis = append(apis, entrada{texto: idioma.Tf("Ver las %d restantes en Mis APIs…", resto), accion: accionIrMisApis})
		}
		if len(e.puntos) > 0 {
			apis = append(apis, separador, entrada{texto: idioma.T("Gestionar en Mis APIs"), icono: "apis", accion: accionIrMisApis})
		} else {
			apis = append(apis, entrada{texto: idioma.T("Crear una API…"), icono: "apis", accion: accionIrMisApis})
		}
		add(entrada{texto: idioma.Tf("Mis APIs (%d de %d activas)", activas, len(e.puntos)), submenu: apis})
		add(entrada{texto: idioma.T("Copiar la dirección para tus apps"), icono: "copiar", accion: accionCopiarDireccion})
	}
	add(separador)
	if !enMarcha {
		add(entrada{texto: idioma.T("Iniciar el servicio"), icono: "iniciar", accion: accionIniciar})
	} else {
		add(entrada{texto: idioma.T("Reiniciar el servicio"), icono: "reiniciar", accion: accionReiniciar})
		add(entrada{texto: idioma.T("Detener el servicio…"), icono: "detener", accion: accionAvisarDetener})
	}
	// El arranque con el sistema también se cambia en Ajustes; el repaso mantiene la marca al día.
	if e.autoarranque {
		etiqueta := idioma.T("Iniciar con Windows")
		if !e.windows {
			etiqueta = idioma.T("Iniciar al entrar en la sesión")
		}
		add(entrada{texto: etiqueta, casilla: true, marcada: e.conWindows, accion: accionAutoarranque})
	}
	add(separador)
	switch {
	case !enMarcha:
		add(entrada{texto: idioma.T("Salir"), icono: "salir", accion: accionSalir})
	case e.sinPreguntar:
		add(entrada{texto: idioma.T("Salir y detener el servicio"), icono: "salir", accion: accionDetenerYSalir})
	default:
		add(entrada{texto: idioma.T("Salir y detener el servicio…"), icono: "salir", accion: accionAvisarSalir})
	}
	return menu
}
