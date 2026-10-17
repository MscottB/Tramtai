package planificador

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
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

// bloqueado no termina hasta que se cierra soltar.
type bloqueado struct{ soltar chan struct{} }

func (b bloqueado) Ejecutar(ctx context.Context, _ nucleo.SolicitudEjecucion, _ func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	select {
	case <-b.soltar:
		return adaptadores.Resultado{Salida: "ok"}, nil
	case <-ctx.Done():
		return adaptadores.Resultado{}, ctx.Err()
	}
}

func TestAgenteOcupadoNoBloqueaAOtro(t *testing.T) {
	alm := abrir(t)
	opc := opciones() // MaximoTrabajos 2
	lento := bloqueado{soltar: make(chan struct{})}
	p := Nuevo(alm, map[string]adaptadores.Adaptador{"lento": lento, "simulado": adaptadores.Simulado{}},
		NuevosSemaforos(map[string]int{"lento": 1, "simulado": 1}), opc)
	if err := p.Iniciar(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Detener)
	s := solicitud("x")
	s.Agente = "lento"
	primero := enviar(t, p, s)
	esperar(t, alm, primero, nucleo.Ejecutando)
	segundo := enviar(t, p, s)
	esperar(t, alm, segundo, nucleo.EsperandoAgente)
	// El segundo trabajo lento espera a su agente sin ocupar plaza: el del otro agente se ejecuta ya.
	esperar(t, alm, enviar(t, p, solicitud("rápido")), nucleo.Completado)
	if tr, _ := alm.Obtener(ctx, segundo); tr.Estado != nucleo.EsperandoAgente {
		t.Fatalf("el segundo trabajo lento sigue esperando a su agente: %s", tr.Estado)
	}
	close(lento.soltar)
	esperar(t, alm, primero, nucleo.Completado)
	esperar(t, alm, segundo, nucleo.Completado)
}

func TestOrdenDeLaColaPorAgente(t *testing.T) {
	alm := abrir(t)
	opc := opciones()
	opc.MaximoTrabajos = 3
	agente := registraSolicitud{recibida: make(chan nucleo.SolicitudEjecucion, 8)}
	p := iniciar(t, alm, agente, 1, opc)
	var ids []string
	for i := range 8 {
		ids = append(ids, enviar(t, p, solicitud(string(rune('a'+i)))))
	}
	for i := range 8 {
		if s := <-agente.recibida; s.Mensajes[0].Contenido != string(rune('a'+i)) {
			t.Fatalf("el agente recibió %q en el turno %d", s.Mensajes[0].Contenido, i)
		}
	}
	for _, id := range ids {
		esperar(t, alm, id, nucleo.Completado)
	}
}

func TestLimiteDeLaInstancia(t *testing.T) {
	alm := abrir(t)
	c := &contador{}
	p := iniciar(t, alm, c, 5, opciones()) // el adaptador admite 5; la instancia, 2
	var ids []string
	for range 4 {
		ids = append(ids, enviar(t, p, solicitud("x")))
	}
	for _, id := range ids {
		esperar(t, alm, id, nucleo.Completado)
	}
	if c.maximo.Load() != 2 {
		t.Fatalf("la instancia debía limitar a 2 ejecuciones simultáneas, hubo %d", c.maximo.Load())
	}
}

// goteo emite muchos fragmentos diminutos con un aviso en medio.
type goteo struct{}

func (goteo) Ejecutar(_ context.Context, _ nucleo.SolicitudEjecucion, emitir func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	var salida strings.Builder
	for i := range 600 {
		if i == 300 {
			if err := emitir(adaptadores.Evento{Tipo: "aviso", Datos: "mitad"}); err != nil {
				return adaptadores.Resultado{}, err
			}
		}
		fragmento := string(rune('a' + i%26))
		salida.WriteString(fragmento)
		if err := emitir(adaptadores.Evento{Tipo: "texto", Datos: fragmento}); err != nil {
			return adaptadores.Resultado{}, err
		}
	}
	return adaptadores.Resultado{Salida: salida.String()}, nil
}

func TestFragmentosDeTextoSeAgrupan(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, goteo{}, 1, opciones())
	final := esperar(t, alm, enviar(t, p, solicitud("x")), nucleo.Completado)
	eventos, _ := alm.Eventos(ctx, final.ID, 0)
	var texto strings.Builder
	textos, aviso := 0, -1
	for i, e := range eventos {
		switch e.Tipo {
		case "texto":
			if aviso >= 0 && texto.Len() < 300 {
				t.Fatalf("el aviso quedó delante de texto anterior a él: %+v", eventos)
			}
			textos++
			texto.WriteString(e.Datos)
		case "aviso":
			aviso = i
			if texto.Len() != 300 {
				t.Fatalf("antes del aviso debían guardarse sus 300 fragmentos, hay %d", texto.Len())
			}
		case "estado":
			if e.Datos == string(nucleo.ValidandoSalida) && texto.String() != final.Salida {
				t.Fatal("todo el texto se guarda antes del cambio de estado")
			}
		}
	}
	if texto.String() != final.Salida || textos == 0 || textos >= 20 {
		t.Fatalf("%d eventos de texto para 600 fragmentos; texto igual a la salida: %v", textos, texto.String() == final.Salida)
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

// registrador guarda el modelo recibido y devuelve uso y modelo efectivo.
type registrador struct{ modelo chan string }

func (r registrador) Ejecutar(_ context.Context, s nucleo.SolicitudEjecucion, _ func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	r.modelo <- s.Modelo
	return adaptadores.Resultado{Salida: "ok", ModeloEfectivo: "efectivo-1", Uso: nucleo.Uso{TokensEntrada: 10, TokensSalida: 2}}, nil
}

func TestPerfilSeResuelveAlAceptar(t *testing.T) {
	alm := abrir(t)
	opc := opciones()
	opc.Perfiles = map[string]map[string]string{"rapido": {"simulado": "modelo-rapido"}}
	r := registrador{modelo: make(chan string, 1)}
	p := iniciar(t, alm, r, 1, opc)
	s := solicitud("x")
	s.PerfilModelo = "rapido"
	tr, _, err := p.Enviar(ctx, s, "")
	if err != nil || tr.ModeloEnviado != "modelo-rapido" {
		t.Fatalf("perfil no resuelto: %+v %v", tr, err)
	}
	if recibido := <-r.modelo; recibido != "modelo-rapido" {
		t.Fatalf("el adaptador recibió %q", recibido)
	}
	final := esperar(t, alm, tr.ID, nucleo.Completado)
	if final.ModeloEfectivo != "efectivo-1" || final.Uso.TokensEntrada != 10 {
		t.Fatalf("uso o modelo efectivo no guardados: %+v", final)
	}

	s.PerfilModelo = "profundo"
	if _, _, err := p.Enviar(ctx, s, ""); nucleo.CodigoDe(err) != nucleo.CodigoModeloNoConfigurado {
		t.Fatalf("un perfil sin modelo para el agente se rechaza: %v", err)
	}
}

// preparadorFalso devuelve un bloque fijo o un error.
type preparadorFalso struct{ err error }

func (p preparadorFalso) Preparar(_ context.Context, cargas []string) (string, error) {
	return "<<<archivo nombre=\"a.txt\">>>\ncontenido\n<<<fin archivo>>>", p.err
}

// eco devuelve el último mensaje recibido, para ver qué llegó al agente.
type eco struct{}

func (eco) Ejecutar(_ context.Context, s nucleo.SolicitudEjecucion, _ func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	return adaptadores.Resultado{Salida: s.Mensajes[len(s.Mensajes)-1].Contenido}, nil
}

const cargaPrueba = "carga_0123456789abcdef0123456789abcdef"

func TestCargasSePreparanAntesDelAgente(t *testing.T) {
	alm := abrir(t)
	opc := opciones()
	opc.Preparador = preparadorFalso{}
	p := iniciar(t, alm, eco{}, 1, opc)
	s := solicitud("resume")
	s.Cargas = []string{cargaPrueba}
	final := esperar(t, alm, enviar(t, p, s), nucleo.Completado)
	if final.Salida != "<<<archivo nombre=\"a.txt\">>>\ncontenido\n<<<fin archivo>>>\nresume" {
		t.Fatalf("el agente recibió %q", final.Salida)
	}
	eventos, _ := alm.Eventos(ctx, final.ID, 0)
	if eventos[1].Datos != string(nucleo.PreparandoEntrada) {
		t.Fatalf("el segundo estado debe ser preparando_entrada: %+v", eventos)
	}
}

func TestCargaNoDisponibleFalla(t *testing.T) {
	alm := abrir(t)
	opc := opciones()
	opc.Preparador = preparadorFalso{err: nucleo.NuevoError(nucleo.CodigoEntradaNoDisponible, "venció")}
	p := iniciar(t, alm, eco{}, 1, opc)
	s := solicitud("x")
	s.Cargas = []string{cargaPrueba}
	if tr := esperar(t, alm, enviar(t, p, s), nucleo.Fallido); tr.CodigoError != nucleo.CodigoEntradaNoDisponible || tr.MensajeError != "venció" {
		t.Fatalf("se esperaba entrada_no_disponible: %+v", tr)
	}
}

func TestEsquemaDeSalida(t *testing.T) {
	alm := abrir(t)
	esquema := []byte(`{"type":"object","required":["ok"],"properties":{"ok":{"type":"boolean"}}}`)
	cumple := iniciar(t, alm, adaptadores.Simulado{Respuesta: `{"ok":true}`}, 1, opciones())
	s := solicitud("x")
	s.Salida.Formato, s.Salida.Esquema = "json", esquema
	esperar(t, alm, enviar(t, cumple, s), nucleo.Completado)

	noCumple := iniciar(t, abrir(t), adaptadores.Simulado{Respuesta: `{"ok":"sí"}`}, 1, opciones())
	tr, _, err := noCumple.Enviar(ctx, s, "")
	if err != nil {
		t.Fatal(err)
	}
	if final := esperar(t, noCumple.almacen, tr.ID, nucleo.Fallido); final.CodigoError != nucleo.CodigoSalidaNoCumpleEsquema || final.Salida != "" {
		t.Fatalf("se esperaba salida_no_cumple_esquema sin conservar la salida: %+v", final)
	}

	s.Salida.Esquema = []byte(`{"type":"objeto"}`)
	if _, _, err := cumple.Enviar(ctx, s, ""); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("un esquema inválido se rechaza al aceptar: %v", err)
	}
}

func TestDrenajeEsperaYLuegoInterrumpe(t *testing.T) {
	alm := abrir(t)
	opc := opciones()
	opc.MaximoTrabajos = 1
	p := Nuevo(alm, map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{Demora: 300 * time.Millisecond}},
		NuevosSemaforos(map[string]int{"simulado": 1}), opc)
	if err := p.Iniciar(ctx); err != nil {
		t.Fatal(err)
	}
	corto := enviar(t, p, solicitud("corto"))
	enCola := enviar(t, p, solicitud("en cola"))
	esperar(t, alm, corto, nucleo.Ejecutando)
	p.Drenar(5 * time.Second)
	if tr, _ := alm.Obtener(ctx, corto); tr.Estado != nucleo.Completado {
		t.Fatalf("el drenaje deja terminar lo que se ejecuta: %s", tr.Estado)
	}
	if tr, _ := alm.Obtener(ctx, enCola); tr.Estado != nucleo.EnCola {
		t.Fatalf("lo que estaba en cola se conserva para el reinicio: %s", tr.Estado)
	}
	if _, _, err := p.Enviar(ctx, solicitud("tarde"), ""); nucleo.CodigoDe(err) != nucleo.CodigoServicioDetenido {
		t.Fatalf("en drenaje no se aceptan trabajos: %v", err)
	}

	alm2 := abrir(t)
	p2 := Nuevo(alm2, map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{Demora: time.Minute}},
		NuevosSemaforos(map[string]int{"simulado": 1}), opc)
	if err := p2.Iniciar(ctx); err != nil {
		t.Fatal(err)
	}
	largo := enviar(t, p2, solicitud("largo"))
	esperar(t, alm2, largo, nucleo.Ejecutando)
	inicio := time.Now()
	p2.Drenar(200 * time.Millisecond)
	if tr, _ := alm2.Obtener(ctx, largo); tr.Estado != nucleo.Interrumpido || time.Since(inicio) > 5*time.Second {
		t.Fatalf("agotado el plazo se interrumpe: %s tras %s", tr.Estado, time.Since(inicio))
	}
}

func TestReintentarSinEntradas(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, adaptadores.Simulado{}, 1, opciones())
	id := enviar(t, p, solicitud("hola"))
	esperar(t, alm, id, nucleo.Completado)
	if _, _, err := alm.Purgar(ctx, "global", persistencia.Retencion{Resultado: time.Hour, Metadatos: time.Hour, EliminarEntradas: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Reintentar(ctx, id); nucleo.CodigoDe(err) != nucleo.CodigoEntradaNoDisponible {
		t.Fatalf("sin entradas el reintento pide reenviarlas: %v", err)
	}
}

func TestLimitePorPuntoDeAcceso(t *testing.T) {
	alm := abrir(t)
	opc := opciones()
	opc.MaximoTrabajos = 3
	p := Nuevo(alm, map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{Demora: 300 * time.Millisecond}},
		NuevosSemaforos(map[string]int{"simulado": 3}), opc)
	if err := p.Iniciar(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Detener)
	p.LimitarPuntos(map[string]int{"app": 1})
	var ids []string
	for range 2 {
		tr, _, err := p.EnviarPorPunto(ctx, solicitud("x"), "", "app")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, tr.ID)
	}
	libre := enviar(t, p, solicitud("sin punto"))
	esperar(t, alm, libre, nucleo.Ejecutando)
	time.Sleep(100 * time.Millisecond)
	ejecutando := 0
	for _, id := range ids {
		if tr, _ := alm.Obtener(ctx, id); tr.Estado == nucleo.Ejecutando {
			ejecutando++
		}
	}
	if ejecutando != 1 {
		t.Fatalf("el punto con límite 1 ejecuta %d a la vez", ejecutando)
	}
	for _, id := range ids {
		esperar(t, alm, id, nucleo.Completado)
	}
}

func TestConservarSalidaInvalida(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, adaptadores.Simulado{Respuesta: "no es json"}, 1, opciones())
	s := solicitud("x")
	s.Salida = nucleo.ConfigSalida{Formato: "json", ConservarSalidaInvalida: true}
	tr := esperar(t, alm, enviar(t, p, s), nucleo.Fallido)
	if tr.CodigoError != nucleo.CodigoSalidaNoCumpleEsquema || tr.Salida != "no es json" {
		t.Fatalf("salida bruta conservada: %+v", tr)
	}
}

// registraSolicitud guarda la solicitud que recibe el agente.
type registraSolicitud struct {
	recibida chan nucleo.SolicitudEjecucion
}

func (r registraSolicitud) Ejecutar(_ context.Context, s nucleo.SolicitudEjecucion, _ func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	r.recibida <- s
	return adaptadores.Resultado{Salida: "ok"}, nil
}

func TestModoConscienteSoloEnProyectoConfiable(t *testing.T) {
	s := solicitud("x")
	s.ModoContexto = "consciente_proyecto"
	p := iniciar(t, abrir(t), adaptadores.Simulado{}, 1, opciones())
	if _, _, err := p.Enviar(ctx, s, ""); nucleo.CodigoDe(err) != nucleo.CodigoContextoNoPermitido {
		t.Fatalf("sin proyecto confiable se rechaza: %v", err)
	}
	opc := opciones()
	opc.RaizConfiable = t.TempDir()
	alm := abrir(t)
	agente := registraSolicitud{recibida: make(chan nucleo.SolicitudEjecucion, 1)}
	p = iniciar(t, alm, agente, 1, opc)
	id := enviar(t, p, s)
	if recibida := <-agente.recibida; recibida.RaizProyecto != opc.RaizConfiable {
		t.Fatalf("el agente recibe la raíz: %q", recibida.RaizProyecto)
	}
	esperar(t, alm, id, nucleo.Completado)
	eventos, _ := alm.Eventos(ctx, id, 0)
	avisado := false
	for _, e := range eventos {
		avisado = avisado || (e.Tipo == "aviso" && strings.Contains(e.Datos, opc.RaizConfiable))
	}
	if !avisado {
		t.Fatal("se declara el contexto cargado antes de ejecutar")
	}
}

// lineas es un io.Writer seguro entre goroutines para leer el registro del planificador.
type lineas struct {
	mu    sync.Mutex
	texto strings.Builder
}

func (l *lineas) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.texto.Write(b)
}

func (l *lineas) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.texto.String()
}

func TestRegistroDeTrabajos(t *testing.T) {
	alm := abrir(t)
	registro := &lineas{}
	opc := opciones()
	opc.Registros = registro
	p := iniciar(t, alm, adaptadores.Simulado{}, 2, opc)
	id := enviar(t, p, solicitud("hola"))
	esperar(t, alm, id, nucleo.Completado)
	limite := time.Now().Add(5 * time.Second)
	for !strings.Contains(registro.String(), "completado") && time.Now().Before(limite) {
		time.Sleep(10 * time.Millisecond)
	}
	texto := registro.String()
	if !strings.Contains(texto, "global recibido    "+id+" simulado") || !strings.Contains(texto, "global completado  "+id+" simulado") {
		t.Fatalf("registro:\n%s", texto)
	}
}

// iniciarVarios es iniciar con varios agentes, cada uno con el mismo límite de trabajos simultáneos.
func iniciarVarios(t *testing.T, alm *persistencia.Almacen, lista map[string]adaptadores.Adaptador, limite int) *Planificador {
	t.Helper()
	limites := map[string]int{}
	for nombre := range lista {
		limites[nombre] = limite
	}
	p := Nuevo(alm, lista, NuevosSemaforos(limites), opciones())
	if err := p.Iniciar(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Detener)
	return p
}

// trasTexto emite texto y después falla: el respaldo ya no puede saltar.
type trasTexto struct{ err error }

func (a trasTexto) Ejecutar(_ context.Context, _ nucleo.SolicitudEjecucion, emitir func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	_ = emitir(adaptadores.Evento{Tipo: "texto", Datos: "parcial"})
	return adaptadores.Resultado{}, a.err
}

// contado cuenta cuántas veces se ejecuta el agente.
type contado struct {
	n *atomic.Int32
	adaptadores.Simulado
}

func (c contado) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, emitir func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	c.n.Add(1)
	return c.Simulado.Ejecutar(ctx, s, emitir)
}

func errorDe(codigo nucleo.CodigoError) error { return nucleo.NuevoError(codigo, string(codigo)) }

func conRespaldo(respaldo ...string) nucleo.SolicitudEjecucion {
	s := solicitud("x")
	s.Respaldo = respaldo
	return s
}

func avisos(t *testing.T, alm *persistencia.Almacen, id string) []string {
	t.Helper()
	eventos, err := alm.Eventos(ctx, id, 0)
	if err != nil {
		t.Fatal(err)
	}
	var lista []string
	for _, e := range eventos {
		if e.Tipo == "aviso" {
			lista = append(lista, e.Datos)
		}
	}
	return lista
}

// primeroBien responde la primera vez y después falla por límite, contando sus ejecuciones.
type primeroBien struct{ n *atomic.Int32 }

func (a primeroBien) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, emitir func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	if a.n.Add(1) == 1 {
		return adaptadores.Simulado{Respuesta: "ok"}.Ejecutar(ctx, s, emitir)
	}
	return adaptadores.Resultado{}, errorDe(nucleo.CodigoLimiteUsoProveedor)
}

func TestRespaldoSaltaPorLimite(t *testing.T) {
	alm := abrir(t)
	var n atomic.Int32
	p := iniciarVarios(t, alm, map[string]adaptadores.Adaptador{
		"simulado": adaptadores.Simulado{Error: errorDe(nucleo.CodigoLimiteUsoProveedor)}, "otro": primeroBien{&n}}, 2)
	tr := esperar(t, alm, enviar(t, p, conRespaldo("otro/m2")), nucleo.Completado)
	if tr.Salida != "ok" || tr.Solicitud.Agente != "otro" || tr.ModeloEnviado != "m2" {
		t.Fatalf("debía responder el respaldo: %+v", tr)
	}
	if a := avisos(t, alm, tr.ID); len(a) != 1 || !strings.Contains(a[0], "simulado") || !strings.Contains(a[0], "otro") {
		t.Fatalf("falta el aviso del salto: %v", a)
	}

	// El reintento empieza por el agente que respondió y omite su entrada repetida en el respaldo: si otro falla,
	// no se vuelve a ejecutar.
	nuevo, err := p.Reintentar(ctx, tr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r := esperar(t, alm, nuevo.ID, nucleo.Fallido); r.Solicitud.Agente != "otro" || len(avisos(t, alm, r.ID)) != 0 || n.Load() != 2 {
		t.Fatalf("el reintento debía ir directo a otro y ejecutarlo una vez (%d en total): %+v", n.Load(), r)
	}
}

func TestRespaldoNoSaltaTrasTexto(t *testing.T) {
	alm := abrir(t)
	p := iniciarVarios(t, alm, map[string]adaptadores.Adaptador{
		"simulado": trasTexto{errorDe(nucleo.CodigoLimiteUsoProveedor)}, "otro": adaptadores.Simulado{Respuesta: "ok"}}, 2)
	tr := esperar(t, alm, enviar(t, p, conRespaldo("otro")), nucleo.Fallido)
	if tr.CodigoError != nucleo.CodigoLimiteUsoProveedor || tr.Solicitud.Agente != "simulado" {
		t.Fatalf("con texto ya emitido no se salta: %+v", tr)
	}
}

func TestRespaldoSinCandidatosQuedaElUltimo(t *testing.T) {
	alm := abrir(t)
	p := iniciarVarios(t, alm, map[string]adaptadores.Adaptador{
		"simulado": adaptadores.Simulado{Error: errorDe(nucleo.CodigoErrorAgente)},
		"otro":     adaptadores.Simulado{Error: errorDe(nucleo.CodigoAgenteNoDisponible)}}, 2)
	if tr := esperar(t, alm, enviar(t, p, conRespaldo("otro")), nucleo.Fallido); tr.CodigoError != nucleo.CodigoAgenteNoDisponible {
		t.Fatalf("debía quedar el error del último candidato: %+v", tr)
	}
}

func TestRespaldoOmiteRepetidos(t *testing.T) {
	alm := abrir(t)
	var n atomic.Int32
	p := iniciarVarios(t, alm, map[string]adaptadores.Adaptador{
		"simulado": contado{&n, adaptadores.Simulado{Error: errorDe(nucleo.CodigoLimiteUsoProveedor)}}, "otro": adaptadores.Simulado{}}, 2)
	esperar(t, alm, enviar(t, p, conRespaldo("simulado", "otro")), nucleo.Completado)
	if n.Load() != 1 {
		t.Fatalf("el principal repetido en el respaldo corrió %d veces", n.Load())
	}
}

func TestRespaldoCanceladoEnEspera(t *testing.T) {
	alm := abrir(t)
	p := iniciarVarios(t, alm, map[string]adaptadores.Adaptador{
		"simulado": adaptadores.Simulado{Error: errorDe(nucleo.CodigoLimiteUsoProveedor)}, "otro": adaptadores.Simulado{Demora: time.Minute}}, 1)
	ocupa := solicitud("largo")
	ocupa.Agente = "otro"
	esperar(t, alm, enviar(t, p, ocupa), nucleo.Ejecutando)
	id := enviar(t, p, conRespaldo("otro"))
	for limite := time.Now().Add(10 * time.Second); len(avisos(t, alm, id)) == 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(limite) {
			t.Fatal("el trabajo no llegó a saltar")
		}
	}
	esperar(t, alm, id, nucleo.EsperandoAgente)
	if _, err := p.Cancelar(ctx, id); err != nil {
		t.Fatal(err)
	}
	esperar(t, alm, id, nucleo.Cancelado)
}

func TestRespaldoAgenteDesconocido(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, adaptadores.Simulado{}, 1, opciones())
	if _, _, err := p.Enviar(ctx, conRespaldo("nadie"), ""); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("un agente de respaldo desconocido se rechaza al aceptar: %v", err)
	}
}

func TestRespaldoNoSaltaAlCancelar(t *testing.T) {
	alm := abrir(t)
	p := iniciarVarios(t, alm, map[string]adaptadores.Adaptador{
		"simulado": adaptadores.Simulado{Demora: time.Minute}, "otro": adaptadores.Simulado{Respuesta: "ok"}}, 2)
	id := enviar(t, p, conRespaldo("otro"))
	esperar(t, alm, id, nucleo.Ejecutando)
	if _, err := p.Cancelar(ctx, id); err != nil {
		t.Fatal(err)
	}
	if tr := esperar(t, alm, id, nucleo.Cancelado); tr.Solicitud.Agente != "simulado" || len(avisos(t, alm, id)) != 0 {
		t.Fatalf("cancelar no debe pasar al respaldo: %+v", tr)
	}
}

func TestRespaldoSaltaPorTiempoAgotado(t *testing.T) {
	alm := abrir(t)
	p := iniciarVarios(t, alm, map[string]adaptadores.Adaptador{
		"simulado": adaptadores.Simulado{Demora: 5 * time.Second}, "otro": adaptadores.Simulado{Respuesta: "ok"}}, 2)
	s := conRespaldo("otro")
	s.Ejecucion.TiempoLimiteSegundos = 1
	tr := esperar(t, alm, enviar(t, p, s), nucleo.Completado)
	if a := avisos(t, alm, tr.ID); tr.Solicitud.Agente != "otro" || len(a) != 1 || !strings.Contains(a[0], string(nucleo.CodigoTiempoEjecucionAgotado)) {
		t.Fatalf("un tiempo agotado sin texto debe pasar al respaldo: %+v %v", tr, a)
	}
}

func TestRespaldoGuardaLosRestantes(t *testing.T) {
	alm := abrir(t)
	p := iniciarVarios(t, alm, map[string]adaptadores.Adaptador{
		"simulado": adaptadores.Simulado{Error: errorDe(nucleo.CodigoLimiteUsoProveedor)}, "otro": adaptadores.Simulado{Respuesta: "ok"},
		"tercero": adaptadores.Simulado{}}, 2)
	// Tras el salto la solicitud guardada solo conserva el respaldo que queda: si el trabajo se reencola o se
	// reintenta, no vuelve a probar al que ya falló.
	tr := esperar(t, alm, enviar(t, p, conRespaldo("otro", "tercero")), nucleo.Completado)
	if tr.Solicitud.Agente != "otro" || len(tr.Solicitud.Respaldo) != 1 || tr.Solicitud.Respaldo[0] != "tercero" {
		t.Fatalf("respaldo restante incorrecto: %+v", tr.Solicitud)
	}
}

func TestOrigenDelTrabajo(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, adaptadores.Simulado{}, 2, opciones())
	desdeProbar := nucleo.ConOrigenPeticion(ctx, nucleo.DesdeProbar)
	tr, _, err := p.Enviar(desdeProbar, solicitud("hola"), "")
	if err != nil || tr.Origen != "probar" {
		t.Fatalf("el origen del contexto se guarda: %q %v", tr.Origen, err)
	}
	if porPunto, _, err := p.EnviarPorPunto(nucleo.ConOrigenPeticion(ctx, nucleo.DesdeCLI), solicitud("hola"), "", "tienda"); err != nil || porPunto.Origen != "api" {
		t.Fatalf("por una API extra el origen es api: %q %v", porPunto.Origen, err)
	}
	esperar(t, alm, tr.ID, nucleo.Completado)
	if nuevo, err := p.Reintentar(ctx, tr.ID); err != nil || nuevo.Origen != "probar" {
		t.Fatalf("el reintento conserva el origen: %q %v", nuevo.Origen, err)
	}
}

// conUso responde (o falla) informando el uso que dio el agente.
type conUso struct {
	uso nucleo.Uso
	err error
}

func (c conUso) Ejecutar(context.Context, nucleo.SolicitudEjecucion, func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	return adaptadores.Resultado{Salida: "ok", Uso: c.uso}, c.err
}

func TestUsoSumaIntentosFallidos(t *testing.T) {
	alm := abrir(t)
	p := iniciarVarios(t, alm, map[string]adaptadores.Adaptador{
		"simulado": conUso{uso: nucleo.Uso{TokensEntrada: 10}, err: nucleo.NuevoError(nucleo.CodigoLimiteUsoProveedor, "límite")},
		"respaldo": conUso{uso: nucleo.Uso{TokensEntrada: 5, TokensSalida: 3}},
	}, 2)
	s := solicitud("hola")
	s.Respaldo = []string{"respaldo"}
	tr := esperar(t, alm, enviar(t, p, s), nucleo.Completado)
	if tr.Uso.TokensEntrada != 15 || tr.Uso.TokensSalida != 3 {
		t.Fatalf("el uso suma el intento que falló antes del salto: %+v", tr.Uso)
	}
}

func TestUsoPrevioAlCancelarElRespaldo(t *testing.T) {
	alm := abrir(t)
	p := iniciarVarios(t, alm, map[string]adaptadores.Adaptador{
		"simulado": conUso{uso: nucleo.Uso{TokensEntrada: 10}, err: nucleo.NuevoError(nucleo.CodigoLimiteUsoProveedor, "límite")},
		"respaldo": adaptadores.Simulado{Demora: time.Hour},
	}, 2)
	s := solicitud("hola")
	s.Respaldo = []string{"respaldo"}
	id := enviar(t, p, s)
	for range 200 { // espera a que el respaldo esté ejecutando
		if tr, _ := alm.Obtener(ctx, id); tr.Estado == nucleo.Ejecutando && tr.Solicitud.Agente == "respaldo" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := p.Cancelar(ctx, id); err != nil {
		t.Fatal(err)
	}
	tr := esperar(t, alm, id, nucleo.Cancelado)
	dia, _, _ := alm.Consumo(ctx, "global", "")
	if tr.Uso.TokensEntrada != 10 || dia.Tokens != 10 {
		t.Fatalf("el uso del intento fallido se conserva al cancelar: trabajo %+v, consumo %+v", tr.Uso, dia)
	}
}
