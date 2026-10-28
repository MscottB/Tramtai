package agentes_test // usa el planificador, que importa agentes

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/configuracion"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/planificador"
)

// aLaVez es un Claude simulado que cuenta cuántas ejecuciones lleva a la vez y anota el máximo.
type aLaVez struct{ ahora, maximo *atomic.Int32 }

func (a aLaVez) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, emitir func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	n := a.ahora.Add(1)
	defer a.ahora.Add(-1)
	for m := a.maximo.Load(); n > m && !a.maximo.CompareAndSwap(m, n); m = a.maximo.Load() {
	}
	time.Sleep(300 * time.Millisecond)
	return adaptadores.Simulado{}.Ejecutar(ctx, s, emitir)
}

// TestSimultaneasServidor: en el servidor, con Claude en suscripción y suscripcion_simultaneas 1, dos peticiones de dos
// APIs distintas van una tras otra aunque la concurrencia del agente sea 2; con clave de API, a la vez.
func TestSimultaneasServidor(t *testing.T) {
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	for modo, quiere := range map[string]int32{adaptadores.AutenticacionSuscripcion: 1, adaptadores.AutenticacionClaveAPI: 2} {
		c := configuracion.Resolver(agentes.Internos(), configuracion.Capa{Origen: configuracion.OrigenGlobal, Valores: map[string]any{
			"adaptadores.claude.autenticacion": modo, "concurrencia_servicio.adaptadores.claude": 2}})
		_, limites := agentes.Registro(c)
		alm, err := persistencia.Abrir(filepath.Join(t.TempDir(), "instancia.db"))
		if err != nil {
			t.Fatal(err)
		}
		cuenta := aLaVez{new(atomic.Int32), new(atomic.Int32)}
		p := planificador.Nuevo(alm, map[string]adaptadores.Adaptador{"claude": cuenta}, planificador.NuevosSemaforos(limites),
			planificador.Opciones{Instancia: "global", MaximoCola: 20, MaximoTrabajos: 4, TiempoMaximoCola: time.Hour,
				TiempoLimitePredeterminado: 30 * time.Second, Secretos: func() []string { return nil }})
		if err := p.Iniciar(context.Background()); err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, api := range []string{"facturas", "correo"} {
			tr, _, err := p.EnviarPorPunto(context.Background(), nucleo.SolicitudEjecucion{Agente: "claude",
				Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "hola"}}}, "", api)
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, tr.ID)
		}
		for _, id := range ids {
			for limite := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
				if tr, err := alm.Obtener(context.Background(), id); err == nil && tr.Estado == nucleo.Completado {
					break
				} else if time.Now().After(limite) {
					t.Fatalf("%s: %s %v", modo, tr.Estado, err)
				}
			}
		}
		p.Detener()
		alm.Cerrar()
		if cuenta.maximo.Load() != quiere {
			t.Errorf("%s: %d a la vez, se esperaban %d", modo, cuenta.maximo.Load(), quiere)
		}
	}
}
