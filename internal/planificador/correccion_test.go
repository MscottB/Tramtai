package planificador

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

// enOrden responde por turnos (el último se repite) y guarda las solicitudes que recibe.
type enOrden struct {
	mu         *sync.Mutex
	respuestas []string
	recibidas  *[]nucleo.SolicitudEjecucion
}

func nuevoEnOrden(respuestas ...string) enOrden {
	return enOrden{mu: &sync.Mutex{}, respuestas: respuestas, recibidas: &[]nucleo.SolicitudEjecucion{}}
}

func (e enOrden) Ejecutar(_ context.Context, s nucleo.SolicitudEjecucion, _ func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	*e.recibidas = append(*e.recibidas, s)
	r := e.respuestas[min(len(*e.recibidas), len(e.respuestas))-1]
	return adaptadores.Resultado{Salida: r, Uso: nucleo.Uso{TokensEntrada: 10, TokensSalida: 1}}, nil
}

func (e enOrden) llamadas() []nucleo.SolicitudEjecucion {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]nucleo.SolicitudEjecucion(nil), *e.recibidas...)
}

const esquemaTotal = `{"type":"object","required":["total"],"properties":{"total":{"type":"number"}}}`

func conEsquema(reintentos int) nucleo.SolicitudEjecucion {
	s := solicitud("¿cuánto es?")
	s.Salida = nucleo.ConfigSalida{Formato: "json", Esquema: json.RawMessage(esquemaTotal), Reintentos: reintentos, ConservarSalidaInvalida: true}
	return s
}

func eventosDe(t *testing.T, p *Planificador, id, tipo string) []string {
	t.Helper()
	eventos, err := p.almacen.Eventos(ctx, id, 0)
	if err != nil {
		t.Fatal(err)
	}
	var datos []string
	for _, e := range eventos {
		if e.Tipo == tipo {
			datos = append(datos, e.Datos)
		}
	}
	return datos
}

func TestCorrigeLaSalida(t *testing.T) {
	alm := abrir(t)
	agente := nuevoEnOrden(`{"total":"diez"}`, `{"total":10}`)
	p := iniciar(t, alm, agente, 1, opciones())
	tr := esperar(t, alm, enviar(t, p, conEsquema(2)), nucleo.Completado)
	if tr.Salida != `{"total":10}` || tr.Uso.TokensEntrada != 20 || tr.Uso.TokensSalida != 2 {
		t.Fatalf("corregida, con el uso de los dos intentos: %+v", tr)
	}
	llamadas := agente.llamadas()
	if len(llamadas) != 2 {
		t.Fatalf("una corrección: %d llamadas", len(llamadas))
	}
	m := llamadas[1].Mensajes
	if len(m) != 3 || m[1].Rol != "asistente" || m[1].Contenido != `{"total":"diez"}` || m[2].Rol != "usuario" ||
		!strings.Contains(m[2].Contenido, "`total` debería ser un número y llegó un texto") || !strings.Contains(m[2].Contenido, "JSON corregido") {
		t.Fatalf("la corrección lleva la respuesta y los problemas: %+v", m)
	}
	if v := eventosDe(t, p, tr.ID, "validacion"); len(v) != 1 || !strings.Contains(v[0], `"problema":"tipo"`) || !strings.Contains(v[0], `"correccion":0`) {
		t.Fatalf("evento validacion: %v", v)
	}
	if i := eventosDe(t, p, tr.ID, "intento"); len(i) != 2 || !strings.Contains(i[1], `"correccion":1,"correcciones":2`) {
		t.Fatalf("la corrección es un intento más: %v", i)
	}
}

func TestSinReintentosNoCorrige(t *testing.T) {
	alm := abrir(t)
	agente := nuevoEnOrden(`{"total":"diez"}`, `{"total":10}`)
	p := iniciar(t, alm, agente, 1, opciones())
	tr := esperar(t, alm, enviar(t, p, conEsquema(0)), nucleo.Fallido)
	if len(agente.llamadas()) != 1 || tr.CodigoError != nucleo.CodigoSalidaNoCumpleEsquema || tr.Salida != `{"total":"diez"}` {
		t.Fatalf("con 0 reintentos no se corrige: %+v", tr)
	}
	if tr.MensajeError != "`total` debería ser un número y llegó un texto" {
		t.Fatalf("el mensaje es el primer problema: %q", tr.MensajeError)
	}
	if pr, err := p.ProblemasSalida(ctx, tr.ID); err != nil || len(pr) != 1 || pr[0].Campo != "total" {
		t.Fatalf("problemas guardados: %+v %v", pr, err)
	}
}

func TestCorreccionesAgotadasYSinRespaldo(t *testing.T) {
	alm := abrir(t)
	malo := nuevoEnOrden(`{}`)
	respaldo := nuevoEnOrden(`{"total":1}`)
	p := iniciarVarios(t, alm, map[string]adaptadores.Adaptador{"simulado": malo, "respaldo": respaldo}, 2)
	s := conEsquema(2)
	s.Respaldo = []string{"respaldo"}
	tr := esperar(t, alm, enviar(t, p, s), nucleo.Fallido)
	if len(malo.llamadas()) != 3 || len(respaldo.llamadas()) != 0 {
		t.Fatalf("con 2 reintentos, 3 intentos y nunca el respaldo: %d y %d", len(malo.llamadas()), len(respaldo.llamadas()))
	}
	if tr.CodigoError != nucleo.CodigoSalidaNoCumpleEsquema || tr.Uso.TokensEntrada != 30 {
		t.Fatalf("falla con el uso de los tres: %+v", tr)
	}
	if v := eventosDe(t, p, tr.ID, "validacion"); len(v) != 3 || !strings.Contains(v[2], `"correccion":2`) {
		t.Fatalf("una validación por intento: %v", v)
	}
}

func TestCorregirAMano(t *testing.T) {
	alm := abrir(t)
	agente := nuevoEnOrden(`{"total":"diez"}`, `{"total":10}`)
	p := iniciar(t, alm, agente, 1, opciones())
	tr := esperar(t, alm, enviar(t, p, conEsquema(0)), nucleo.Fallido)
	nuevo, err := p.Corregir(ctx, tr.ID)
	if err != nil {
		t.Fatal(err)
	}
	final := esperar(t, alm, nuevo.ID, nucleo.Completado)
	if final.IntentoDe != tr.ID || final.Salida != `{"total":10}` || len(final.Solicitud.Mensajes) != 3 {
		t.Fatalf("corrección vinculada con la conversación: %+v", final)
	}
	if m := agente.llamadas()[1].Mensajes; m[1].Contenido != `{"total":"diez"}` || !strings.Contains(m[2].Contenido, "`total`") {
		t.Fatalf("la corrección manual lleva la respuesta y los problemas: %+v", m)
	}
	// Un trabajo completado no se corrige.
	if _, err := p.Corregir(ctx, final.ID); nucleo.CodigoDe(err) != nucleo.CodigoCorreccionNoDisponible {
		t.Fatalf("sin fallo de esquema: %v", err)
	}
}

func TestCorregirSinSalidaConservada(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, nuevoEnOrden(`{}`), 1, opciones())
	s := conEsquema(0)
	s.Salida.ConservarSalidaInvalida = false
	tr := esperar(t, alm, enviar(t, p, s), nucleo.Fallido)
	if _, err := p.Corregir(ctx, tr.ID); nucleo.CodigoDe(err) != nucleo.CodigoCorreccionNoDisponible || !strings.Contains(err.Error(), "conservar_salida_invalida") {
		t.Fatalf("sin salida conservada no hay corrección: %v", err)
	}
}
