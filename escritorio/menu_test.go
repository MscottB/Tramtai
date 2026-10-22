package main

import (
	"strings"
	"testing"

	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
)

// acciones devuelve las acciones del menú en orden, con «-» por cada separador y «[» … «]» alrededor del submenú.
func acciones(entradas []entrada) []accion {
	var a []accion
	for _, n := range entradas {
		switch {
		case n.separador:
			a = append(a, -1)
		case n.submenu != nil:
			a = append(a, -2)
			a = append(a, acciones(n.submenu)...)
			a = append(a, -3)
		default:
			a = append(a, n.accion)
		}
	}
	return a
}

func iguales(a, b []accion) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMenuConElServicioDetenido(t *testing.T) {
	m := entradasMenu(estadoMenu{autoarranque: true, windows: true, conWindows: true})
	quiero := []accion{accionMostrar, accionAcercaDe, -1, accionMostrar, -1, accionIniciar, accionAutoarranque, -1, accionSalir}
	if got := acciones(m); !iguales(got, quiero) {
		t.Fatalf("acciones %v, quería %v", got, quiero)
	}
	if m[0].texto != idioma.T("Servicio detenido") || m[0].icono != "rojo" {
		t.Errorf("primera línea: %+v", m[0])
	}
	if c := m[6]; !c.casilla || !c.marcada || c.texto != idioma.T("Iniciar con Windows") {
		t.Errorf("casilla del arranque: %+v", c)
	}
}

func TestMenuConElServicioEnMarcha(t *testing.T) {
	puntos := []contrato.EstadoPunto{
		{Nombre: "uno", Instancia: "global", Activo: true},
		{Nombre: "dos", Instancia: "proy", Grupo: "equipo", Error: "puerto ocupado"},
		{Nombre: "tres", Instancia: "global", Error: "desactivado"},
	}
	m := entradasMenu(estadoMenu{base: "http://127.0.0.1:7777", puntos: puntos, proyectos: map[string]string{"proy": "web"},
		sinPreguntar: true, windows: false, autoarranque: true})
	quiero := []accion{accionMostrar, accionAcercaDe, -1, accionMostrar, accionBuscarActualizaciones,
		-2, accionAlternarApi, accionAlternarApi, accionAlternarApi, -1, accionIrMisApis, -3, accionCopiarDireccion,
		-1, accionReiniciar, accionAvisarDetener, accionAutoarranque, -1, accionDetenerYSalir}
	if got := acciones(m); !iguales(got, quiero) {
		t.Fatalf("acciones %v, quería %v", got, quiero)
	}
	if !strings.Contains(m[0].texto, "127.0.0.1:7777") || strings.Contains(m[0].texto, "http://") || m[0].icono != "verde" {
		t.Errorf("primera línea: %+v", m[0])
	}
	apis := m[5]
	if apis.texto != idioma.Tf("Mis APIs (%d de %d activas)", 1, 3) {
		t.Errorf("submenú: %q", apis.texto)
	}
	// Primero la que tiene error, con su grupo y su proyecto; después la activa y al final la desactivada.
	conError, activa, desactivada := apis.submenu[0], apis.submenu[1], apis.submenu[2]
	if conError.texto != "dos · equipo · web"+idioma.T(" (con error)") || conError.icono != "rojo" || !conError.habilitada {
		t.Errorf("con error: %+v", conError)
	}
	if activa.punto.Nombre != "uno" || activa.icono != "verde" || !activa.habilitada {
		t.Errorf("activa: %+v", activa)
	}
	if desactivada.punto.Nombre != "tres" || desactivada.icono != "rojo" || desactivada.habilitada {
		t.Errorf("desactivada: %+v", desactivada)
	}
	if m[10].texto != idioma.T("Iniciar al entrar en la sesión") {
		t.Errorf("fuera de Windows: %q", m[10].texto)
	}
}

func TestMenuConVersionNuevaYSinApis(t *testing.T) {
	m := entradasMenu(estadoMenu{base: "http://127.0.0.1:7777", nueva: "v9.9.9"})
	quiero := []accion{accionMostrar, accionAcercaDe, -1, accionMostrar, accionActualizar, -2, accionIrMisApis, -3,
		accionCopiarDireccion, -1, accionReiniciar, accionAvisarDetener, -1, accionAvisarSalir}
	if got := acciones(m); !iguales(got, quiero) {
		t.Fatalf("acciones %v, quería %v", got, quiero)
	}
	if m[1].texto != idioma.Tf("Acerca de Tramtai · hay una versión nueva: %s", "v9.9.9") {
		t.Errorf("acerca de: %q", m[1].texto)
	}
	if m[5].submenu[0].texto != idioma.T("Crear una API…") {
		t.Errorf("sin APIs: %+v", m[5].submenu)
	}
}

func TestMenuConMuchasApis(t *testing.T) {
	var puntos []contrato.EstadoPunto
	for i := range maximoApisMenu + 4 {
		puntos = append(puntos, contrato.EstadoPunto{Nombre: string(rune('a' + i)), Activo: true})
	}
	sub := entradasMenu(estadoMenu{base: "http://127.0.0.1:7777", puntos: puntos})[5].submenu
	if len(sub) != maximoApisMenu+3 || sub[maximoApisMenu].texto != idioma.Tf("Ver las %d restantes en Mis APIs…", 4) {
		t.Fatalf("submenú con %d entradas: %+v", len(sub), sub[maximoApisMenu])
	}
}
