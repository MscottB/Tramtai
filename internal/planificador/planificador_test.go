package planificador

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
)

var ctx = context.Background()

func abrir(t *testing.T) *persistencia.Almacen {
	t.Helper()
	a, err := persistencia.Abrir(filepath.Join(t.TempDir(), "instancia.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Cerrar() })
	return a
}

func opciones() Opciones {
	return Opciones{Instancia: "global", MaximoCola: 20, MaximoTrabajos: 2, TiempoMaximoCola: time.Hour, TiempoLimitePredeterminado: 30 * time.Second}
}

func iniciar(t *testing.T, alm *persistencia.Almacen, adaptador adaptadores.Adaptador, limiteAdaptador int, opc Opciones) *Planificador {
	t.Helper()
	p := Nuevo(alm, map[string]adaptadores.Adaptador{"simulado": adaptador}, NuevosSemaforos(map[string]int{"simulado": limiteAdaptador}), opc)
	if err := p.Iniciar(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Detener)
	return p
}

func solicitud(texto string) nucleo.SolicitudEjecucion {
	return nucleo.SolicitudEjecucion{Agente: "simulado", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}}
}

func enviar(t *testing.T, p *Planificador, s nucleo.SolicitudEjecucion) string {
	t.Helper()
	tr, _, err := p.Enviar(ctx, s, "")
	if err != nil {
		t.Fatal(err)
	}
	return tr.ID
}

// esperar sondea hasta que el trabajo llega al estado indicado.
func esperar(t *testing.T, alm *persistencia.Almacen, id string, objetivo nucleo.Estado) nucleo.Trabajo {
	t.Helper()
	limite := time.Now().Add(10 * time.Second)
	for {
		tr, err := alm.Obtener(ctx, id)
		if err == nil && tr.Estado == objetivo {
			return tr
		}
		if time.Now().After(limite) {
			t.Fatalf("%s: estado %s, se esperaba %s (%v)", id, tr.Estado, objetivo, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCompletaConAgenteSimulado(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, adaptadores.Simulado{}, 2, opciones())
	tr := esperar(t, alm, enviar(t, p, solicitud("hola")), nucleo.Completado)
	if tr.Salida != "hola" || tr.TerminadoEn == nil {
		t.Fatalf("trabajo completado incorrecto: %+v", tr)
	}
	eventos, _ := alm.Eventos(ctx, tr.ID, 0)
	var recorrido []string
	for _, e := range eventos {
		recorrido = append(recorrido, e.Datos)
	}
	esperado := []string{"en_cola", "esperando_agente", "ejecutando", "hola", "validando_salida", "completado"}
	if len(recorrido) != len(esperado) {
		t.Fatalf("recorrido de eventos %v, se esperaba %v", recorrido, esperado)
	}
	for i := range esperado {
		if recorrido[i] != esperado[i] {
			t.Fatalf("recorrido de eventos %v, se esperaba %v", recorrido, esperado)
		}
	}
}

func TestSalidaJSONInvalida(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, adaptadores.Simulado{Respuesta: "no es json"}, 2, opciones())
	s := solicitud("x")
	s.Salida.Formato = "json"
	if tr := esperar(t, alm, enviar(t, p, s), nucleo.Fallido); tr.CodigoError != nucleo.CodigoSalidaNoCumpleEsquema || tr.Salida != "" {
		t.Fatalf("se esperaba salida_no_cumple_esquema sin conservar la salida: %+v", tr)
	}
}

func TestTiempoEjecucionAgotado(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, adaptadores.Simulado{Demora: 5 * time.Second}, 2, opciones())
	s := solicitud("x")
	s.Ejecucion.TiempoLimiteSegundos = 1
	if tr := esperar(t, alm, enviar(t, p, s), nucleo.Fallido); tr.CodigoError != nucleo.CodigoTiempoEjecucionAgotado {
		t.Fatalf("se esperaba tiempo_ejecucion_agotado: %+v", tr)
	}
}

func TestCancelarEnEjecucionEsIdempotente(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, adaptadores.Simulado{Demora: time.Minute}, 2, opciones())
	id := enviar(t, p, solicitud("x"))
	esperar(t, alm, id, nucleo.Ejecutando)
	if _, err := p.Cancelar(ctx, id); err != nil {
		t.Fatal(err)
	}
	esperar(t, alm, id, nucleo.Cancelado)
	if tr, err := p.Cancelar(ctx, id); err != nil || tr.Estado != nucleo.Cancelado {
		t.Fatalf("cancelar de nuevo debe devolver el mismo estado: %+v %v", tr, err)
	}
}

func TestCancelarEnColaYColaLlena(t *testing.T) {
	alm := abrir(t)
	opc := opciones()
	opc.MaximoCola, opc.MaximoTrabajos = 1, 1
	p := iniciar(t, alm, adaptadores.Simulado{Demora: time.Minute}, 1, opc)
	primero := enviar(t, p, solicitud("1"))
	esperar(t, alm, primero, nucleo.Ejecutando)
	segundo := enviar(t, p, solicitud("2"))
	if _, _, err := p.Enviar(ctx, solicitud("3"), ""); nucleo.CodigoDe(err) != nucleo.CodigoColaLlena {
		t.Fatalf("con la cola llena se esperaba cola_llena: %v", err)
	}
	if tr, err := p.Cancelar(ctx, segundo); err != nil || tr.Estado != nucleo.Cancelado {
		t.Fatalf("un trabajo en cola se cancela de inmediato: %+v %v", tr, err)
	}
	inicio := time.Now()
	if _, _, err := p.Enviar(ctx, solicitud("4"), ""); err != nil {
		t.Fatalf("tras cancelar hay sitio en la cola: %v", err)
	}
	if espera := time.Since(inicio); espera > time.Second {
		t.Fatalf("Enviar no debe bloquearse por un trabajo cancelado que sigue en el canal: tardó %s", espera)
	}
}

func TestIdempotenciaEnPlanificador(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, adaptadores.Simulado{}, 2, opciones())
	a, _, err := p.Enviar(ctx, solicitud("hola"), "clave")
	if err != nil {
		t.Fatal(err)
	}
	b, reutilizado, err := p.Enviar(ctx, solicitud("hola"), "clave")
	if err != nil || !reutilizado || b.ID != a.ID {
		t.Fatalf("la repetición debe devolver el mismo trabajo: %v %v", reutilizado, err)
	}
	if _, _, err := p.Enviar(ctx, solicitud("otra"), "clave"); nucleo.CodigoDe(err) != nucleo.CodigoConflictoIdempotencia {
		t.Fatalf("se esperaba conflicto_idempotencia: %v", err)
	}
	esperar(t, alm, a.ID, nucleo.Completado)
}

// contador mide cuántas ejecuciones simultáneas recibe el adaptador.
type contador struct {
	actual, maximo atomic.Int32
}

func (c *contador) Ejecutar(ctx context.Context, _ nucleo.SolicitudEjecucion, _ func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	n := c.actual.Add(1)
	defer c.actual.Add(-1)
	for {
		m := c.maximo.Load()
		if n <= m || c.maximo.CompareAndSwap(m, n) {
			break
		}
	}
	select {
	case <-time.After(150 * time.Millisecond):
		return adaptadores.Resultado{Salida: "ok"}, nil
	case <-ctx.Done():
		return adaptadores.Resultado{}, ctx.Err()
	}
}

func TestSemaforoPorAdaptador(t *testing.T) {
	alm := abrir(t)
	c := &contador{}
	opc := opciones()
	opc.MaximoTrabajos = 3
	p := iniciar(t, alm, c, 1, opc)
	var ids []string
	for i := 0; i < 3; i++ {
		ids = append(ids, enviar(t, p, solicitud("x")))
	}
	for _, id := range ids {
		esperar(t, alm, id, nucleo.Completado)
	}
	if c.maximo.Load() != 1 {
		t.Fatalf("el semáforo del adaptador debía limitar a 1 ejecución simultánea, hubo %d", c.maximo.Load())
	}
}

func TestTiempoEnColaYAgenteNoRegistrado(t *testing.T) {
	alm := abrir(t)
	opc := opciones()
	opc.TiempoMaximoCola = time.Nanosecond
	p := iniciar(t, alm, adaptadores.Simulado{}, 2, opc)
	if tr := esperar(t, alm, enviar(t, p, solicitud("x")), nucleo.Fallido); tr.CodigoError != nucleo.CodigoTiempoColaAgotado {
		t.Fatalf("se esperaba tiempo_cola_agotado: %+v", tr)
	}
	otro := iniciar(t, abrir(t), adaptadores.Simulado{}, 2, opciones())
	s := solicitud("x")
	s.Agente = "inexistente"
	tr, _, err := otro.Enviar(ctx, s, "")
	if err != nil {
		t.Fatal(err)
	}
	if final := esperar(t, otro.almacen, tr.ID, nucleo.Fallido); final.CodigoError != nucleo.CodigoAgenteNoDisponible {
		t.Fatalf("se esperaba agente_no_disponible: %+v", final)
	}
}

func TestSolicitudInvalidaNoSeAcepta(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, adaptadores.Simulado{}, 2, opciones())
	if _, _, err := p.Enviar(ctx, nucleo.SolicitudEjecucion{Agente: "simulado"}, ""); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("se esperaba solicitud_invalida: %v", err)
	}
	if ids, _ := alm.ListarPorEstado(ctx, "global", nucleo.EnCola); len(ids) != 0 {
		t.Fatal("una solicitud inválida no debe registrarse")
	}
}

func TestReinicioNoRelanzaYReencola(t *testing.T) {
	alm := abrir(t)
	opc := opciones()
	opc.MaximoTrabajos = 1
	p1 := Nuevo(alm, map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{Demora: time.Minute}}, NuevosSemaforos(map[string]int{"simulado": 1}), opc)
	if err := p1.Iniciar(ctx); err != nil {
		t.Fatal(err)
	}
	enCurso := enviar(t, p1, solicitud("1"))
	esperar(t, alm, enCurso, nucleo.Ejecutando)
	pendiente := enviar(t, p1, solicitud("2"))
	p1.Detener()
	esperar(t, alm, enCurso, nucleo.Interrumpido)
	esperar(t, alm, pendiente, nucleo.EnCola)
	if _, _, err := p1.Enviar(ctx, solicitud("3"), ""); nucleo.CodigoDe(err) != nucleo.CodigoServicioDetenido {
		t.Fatalf("un planificador detenido no acepta trabajos: %v", err)
	}

	iniciar(t, alm, adaptadores.Simulado{}, 1, opc)
	esperar(t, alm, pendiente, nucleo.Completado)
	if tr, _ := alm.Obtener(ctx, enCurso); tr.Estado != nucleo.Interrumpido {
		t.Fatalf("un trabajo interrumpido no se relanza: %s", tr.Estado)
	}
}
