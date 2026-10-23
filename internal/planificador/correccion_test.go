package planificador

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

// porFuncion es un adaptador hecho con una función, para guiones de prueba.
type porFuncion func(context.Context, nucleo.SolicitudEjecucion) (adaptadores.Resultado, error)

func (f porFuncion) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, _ func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	return f(ctx, s)
}

// esperarQue sondea hasta que se cumple la condición.
func esperarQue(t *testing.T, que string, condicion func() bool) {
	t.Helper()
	for limite := time.Now().Add(10 * time.Second); !condicion(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(limite) {
			t.Fatalf("no ocurrió: %s", que)
		}
	}
}

// Un drenaje justo antes de pedir la corrección no devuelve el trabajo a la cola: ya ejecutó y se cerraría dos veces.
func TestDrenajeAntesDeCorregir(t *testing.T) {
	alm := abrir(t)
	var p *Planificador
	var llamadas atomic.Int32
	p = iniciar(t, alm, porFuncion(func(context.Context, nucleo.SolicitudEjecucion) (adaptadores.Resultado, error) {
		llamadas.Add(1)
		go p.Drenar(time.Minute)
		esperarQue(t, "el drenaje empieza", func() bool { return p.ctx.Err() != nil })
		return adaptadores.Resultado{Salida: `{}`, ModeloEfectivo: "m1", Uso: nucleo.Uso{TokensEntrada: 10}}, nil
	}), 1, opciones())
	tr := esperar(t, alm, enviar(t, p, conEsquema(2)), nucleo.Fallido)
	if llamadas.Load() != 1 || tr.CodigoError != nucleo.CodigoSalidaNoCumpleEsquema || tr.Uso.TokensEntrada != 10 || tr.Salida != `{}` || tr.ModeloEfectivo != "m1" {
		t.Fatalf("se cierra con los problemas, el uso y la salida: %d %+v", llamadas.Load(), tr)
	}
}

// lentoORapido: «lento» ocupa el agente hasta que lo detienen; el resto responde `{}` (no cumple) tras una pausa.
func lentoORapido() porFuncion {
	return func(ctx context.Context, s nucleo.SolicitudEjecucion) (adaptadores.Resultado, error) {
		if s.Mensajes[0].Contenido == "lento" {
			<-ctx.Done()
			return adaptadores.Resultado{}, ctx.Err()
		}
		time.Sleep(200 * time.Millisecond)
		return adaptadores.Resultado{Salida: `{}`, Uso: nucleo.Uso{TokensEntrada: 10}}, nil
	}
}

// corrigiendoEnEspera deja un trabajo esperando al agente para su corrección, con otro trabajo ocupándolo.
func corrigiendoEnEspera(t *testing.T) (*Planificador, string) {
	alm := abrir(t)
	p := iniciar(t, alm, lentoORapido(), 1, opciones())
	id := enviar(t, p, conEsquema(1))
	esperar(t, alm, id, nucleo.Ejecutando)
	lento := enviar(t, p, solicitud("lento")) // espera el agente y lo toma antes que la corrección
	esperar(t, alm, lento, nucleo.Ejecutando)
	esperarQue(t, "la corrección espera al agente", func() bool {
		tr, _ := alm.Obtener(ctx, id)
		return tr.Estado == nucleo.EsperandoAgente
	})
	return p, id
}

func TestDrenajeConLaCorreccionEnEspera(t *testing.T) {
	p, id := corrigiendoEnEspera(t)
	p.Drenar(100 * time.Millisecond)
	tr, _ := p.almacen.Obtener(ctx, id)
	if tr.Estado != nucleo.Fallido || tr.CodigoError != nucleo.CodigoSalidaNoCumpleEsquema || tr.Uso.TokensEntrada != 10 || tr.Salida != `{}` {
		t.Fatalf("no vuelve a la cola: se cierra con los problemas y el uso: %+v", tr)
	}
}

func TestCancelarConLaCorreccionEnEspera(t *testing.T) {
	p, id := corrigiendoEnEspera(t)
	if _, err := p.Cancelar(ctx, id); err != nil {
		t.Fatal(err)
	}
	if tr := esperar(t, p.almacen, id, nucleo.Cancelado); tr.Uso.TokensEntrada != 10 {
		t.Fatalf("cancelado, con el uso del intento: %+v", tr.Uso)
	}
}

// Si respondió el respaldo, la corrección se le pide al respaldo.
func TestCorreccionAlRespaldoQueRespondio(t *testing.T) {
	alm := abrir(t)
	var principal atomic.Int32
	respaldo := nuevoEnOrden(`{}`, `{"total":1}`)
	p := iniciarVarios(t, alm, map[string]adaptadores.Adaptador{
		"simulado": porFuncion(func(context.Context, nucleo.SolicitudEjecucion) (adaptadores.Resultado, error) {
			principal.Add(1)
			return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoLimiteUsoProveedor, "límite")
		}),
		"respaldo": respaldo,
	}, 2)
	s := conEsquema(1)
	s.Respaldo = []string{"respaldo"}
	esperar(t, alm, enviar(t, p, s), nucleo.Completado)
	if principal.Load() != 1 || len(respaldo.llamadas()) != 2 {
		t.Fatalf("la corrección va al respaldo: principal %d, respaldo %d", principal.Load(), len(respaldo.llamadas()))
	}
}

// Corregir a mano una respuesta del respaldo sigue con el respaldo y conserva su intento.
func TestCorregirConservaElIntento(t *testing.T) {
	alm := abrir(t)
	respaldo := nuevoEnOrden(`{}`, `{"total":1}`)
	p := iniciarVarios(t, alm, map[string]adaptadores.Adaptador{
		"simulado": adaptadores.Simulado{Error: nucleo.NuevoError(nucleo.CodigoLimiteUsoProveedor, "límite")},
		"respaldo": respaldo,
	}, 2)
	s := conEsquema(0)
	s.Respaldo = []string{"respaldo"}
	tr := esperar(t, alm, enviar(t, p, s), nucleo.Fallido)
	if tr.Intento != 2 || tr.Solicitud.Agente != "respaldo" {
		t.Fatalf("falló el esquema en el respaldo, intento 2: %+v", tr)
	}
	nuevo, err := p.Corregir(ctx, tr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final := esperar(t, alm, nuevo.ID, nucleo.Completado); final.Intento != 2 || final.Solicitud.Agente != "respaldo" || len(respaldo.llamadas()) != 2 {
		t.Fatalf("la corrección conserva el intento del respaldo: %+v", final)
	}
}

// Un error del agente durante la corrección cierra con ese error, el uso de los dos intentos y el modelo que respondió.
func TestErrorDelAgenteEnLaCorreccion(t *testing.T) {
	alm := abrir(t)
	var n atomic.Int32
	p := iniciar(t, alm, porFuncion(func(context.Context, nucleo.SolicitudEjecucion) (adaptadores.Resultado, error) {
		if n.Add(1) == 1 {
			return adaptadores.Resultado{Salida: `{}`, ModeloEfectivo: "m1", Uso: nucleo.Uso{TokensEntrada: 10}}, nil
		}
		return adaptadores.Resultado{Uso: nucleo.Uso{TokensEntrada: 5}}, nucleo.NuevoError(nucleo.CodigoErrorAgente, "se cayó")
	}), 1, opciones())
	tr := esperar(t, alm, enviar(t, p, conEsquema(1)), nucleo.Fallido)
	if tr.CodigoError != nucleo.CodigoErrorAgente || tr.Uso.TokensEntrada != 15 || tr.ModeloEfectivo != "m1" {
		t.Fatalf("error, uso sumado y modelo: %+v", tr)
	}
}

// En una corrección a mano los documentos van con la pregunta, no con el pedido de corrección.
func TestDocumentosConLaPreguntaAlCorregir(t *testing.T) {
	s := conCorreccion(solicitud("¿cuánto es?"), `{}`, nil, "no cumple")
	m := conDocumentos(s, "[documento]").Mensajes
	if !strings.HasPrefix(m[0].Contenido, "[documento]") || strings.Contains(m[2].Contenido, "[documento]") {
		t.Fatalf("documentos en el mensaje equivocado: %+v", m)
	}
}

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
