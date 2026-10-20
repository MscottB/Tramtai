package planificador

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
)

const (
	sobreMalo  = `{"content":"","tool_calls":[{"name":"clima","arguments":"{\"ciudad\":7}"}]}`
	sobreBueno = `{"content":"","tool_calls":[{"name":"clima","arguments":"{\"ciudad\":\"Santiago\"}"}]}`
)

func conClima(reintentos int) nucleo.SolicitudEjecucion {
	s := solicitud("¿Qué tiempo hace en Santiago?")
	s.Herramientas = []nucleo.Herramienta{{Nombre: "clima", Parametros: json.RawMessage(
		`{"type":"object","properties":{"ciudad":{"type":"string"}},"required":["ciudad"]}`)}}
	s.Salida.Reintentos = reintentos
	return s
}

// guion responde, en orden, las salidas dadas (repite la última) y guarda lo que recibió el agente en cada intento.
type guion struct {
	mu        sync.Mutex
	salidas   []string
	recibidas []nucleo.SolicitudEjecucion
	texto     bool // emite la salida también como texto en vivo
}

func (g *guion) Ejecutar(_ context.Context, s nucleo.SolicitudEjecucion, emitir func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	g.mu.Lock()
	n := len(g.recibidas)
	g.recibidas = append(g.recibidas, s)
	salida := g.salidas[min(n, len(g.salidas)-1)]
	g.mu.Unlock()
	if g.texto {
		_ = emitir(adaptadores.Evento{Tipo: "texto", Datos: salida})
	}
	return adaptadores.Resultado{Salida: salida}, nil
}

func (g *guion) recibida(i int) nucleo.SolicitudEjecucion {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.recibidas[i]
}

func TestHerramientasInvalidasAlAceptar(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, &guion{salidas: []string{sobreBueno}}, 1, opciones())
	s := conClima(0)
	s.Herramientas[0].Nombre = ""
	if _, _, err := p.Enviar(ctx, s, ""); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("una herramienta sin nombre se rechaza al aceptar: %v", err)
	}
	if lista, _ := alm.Listar(ctx, "global", persistencia.FiltroTrabajos{}, 10); len(lista) != 0 {
		t.Fatalf("no se encola nada: %d", len(lista))
	}
}

func TestHerramientasCorreccionLuegoBien(t *testing.T) {
	alm := abrir(t)
	agente := &guion{salidas: []string{sobreMalo, sobreBueno}}
	p := iniciar(t, alm, agente, 1, opciones())
	tr := esperar(t, alm, enviar(t, p, conClima(1)), nucleo.Completado)
	if tr.Salida != sobreBueno {
		t.Fatalf("salida %q", tr.Salida)
	}
	ultimo := agente.recibida(1).Mensajes
	if correccion := ultimo[len(ultimo)-1].Contenido; !strings.Contains(correccion, "clima") {
		t.Fatalf("la corrección le dice al agente qué falló: %q", correccion)
	}
}

func TestHerramientasSinCorreccionesFalla(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, &guion{salidas: []string{sobreMalo}}, 1, opciones())
	if tr := esperar(t, alm, enviar(t, p, conClima(0)), nucleo.Fallido); tr.CodigoError != nucleo.CodigoSalidaNoCumpleEsquema {
		t.Fatalf("código %s", tr.CodigoError)
	}
}

func TestHerramientasRespaldoRecibeElSobre(t *testing.T) {
	alm := abrir(t)
	respaldo := &guion{salidas: []string{sobreBueno}}
	p := iniciarVarios(t, alm, map[string]adaptadores.Adaptador{
		"simulado": adaptadores.Simulado{Error: errorDe(nucleo.CodigoLimiteUsoProveedor)}, "otro": respaldo}, 2)
	s := conClima(0)
	s.Respaldo = []string{"otro"}
	esperar(t, alm, enviar(t, p, s), nucleo.Completado)
	if r := respaldo.recibida(0); !strings.Contains(r.Instrucciones, "Available tools") || !strings.Contains(string(r.Salida.Esquema), "tool_calls") {
		t.Fatalf("el respaldo recibe la descripción y el sobre: %q %s", r.Instrucciones, r.Salida.Esquema)
	}
}

func TestSolicitudGuardadaSinPreparar(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, &guion{salidas: []string{sobreBueno}}, 1, opciones())
	tr := esperar(t, alm, enviar(t, p, conClima(0)), nucleo.Completado)
	if len(tr.Solicitud.Herramientas) != 1 || strings.Contains(tr.Solicitud.Instrucciones, "Available tools") || len(tr.Solicitud.Salida.Esquema) > 0 {
		t.Fatalf("se guarda la solicitud del cliente, no la preparada: %+v", tr.Solicitud)
	}
}

func TestHerramientasSinTextoEnVivo(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, &guion{salidas: []string{sobreBueno}, texto: true}, 1, opciones())
	id := enviar(t, p, conClima(0))
	esperar(t, alm, id, nucleo.Completado)
	eventos, _ := alm.Eventos(ctx, id, 0)
	for _, e := range eventos {
		if e.Tipo == "texto" {
			t.Fatalf("con herramientas no se ve el sobre interno en vivo: %q", e.Datos)
		}
	}
	// Sin herramientas, el texto en vivo sigue llegando.
	id = enviar(t, p, solicitud("hola"))
	esperar(t, alm, id, nucleo.Completado)
	eventos, _ = alm.Eventos(ctx, id, 0)
	hayTexto := false
	for _, e := range eventos {
		hayTexto = hayTexto || e.Tipo == "texto"
	}
	if !hayTexto {
		t.Fatal("sin herramientas hay texto en vivo")
	}
}
