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

// aLaVez es un Claude simulado, con su autenticación cambiable en marcha, que anota cuántas ejecuciones lleva a la vez.
type aLaVez struct {
	modo          *adaptadores.Modo
	ahora, maximo *atomic.Int32
}

func (a aLaVez) ModoAutenticacion() *adaptadores.Modo { return a.modo }

func (a aLaVez) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, emitir func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	n := a.ahora.Add(1)
	defer a.ahora.Add(-1)
	for m := a.maximo.Load(); n > m && !a.maximo.CompareAndSwap(m, n); m = a.maximo.Load() {
	}
	time.Sleep(300 * time.Millisecond)
	return adaptadores.Simulado{}.Ejecutar(ctx, s, emitir)
}

// TestSimultaneasServidor: en el servidor, con la concurrencia de Claude en 2, dos peticiones de dos APIs distintas van a
// la vez con clave de API; tras pasarlo a suscripción en marcha (sin reiniciar), con suscripcion_simultaneas 1, una tras
// otra. Fuera del servidor la suscripción no limita nada.
func TestSimultaneasServidor(t *testing.T) {
	for _, servidor := range []bool{true, false} {
		t.Setenv("TRAMTAI_SERVIDOR", map[bool]string{true: "1", false: ""}[servidor])
		c := configuracion.Resolver(agentes.Internos(), configuracion.Capa{Origen: configuracion.OrigenGlobal, Valores: map[string]any{
			"concurrencia_servicio.adaptadores.claude": 2}})
		_, limites := agentes.Registro(c)
		alm, err := persistencia.Abrir(filepath.Join(t.TempDir(), "instancia.db"))
		if err != nil {
			t.Fatal(err)
		}
		claude := aLaVez{adaptadores.NuevoModo(adaptadores.AutenticacionClaveAPI), new(atomic.Int32), new(atomic.Int32)}
		registro := map[string]adaptadores.Adaptador{"claude": claude}
		p := planificador.Nuevo(alm, registro, planificador.NuevosSemaforos(limites).ConSuscripcion(agentes.Simultaneas(c)),
			planificador.Opciones{Instancia: "global", MaximoCola: 20, MaximoTrabajos: 4, TiempoMaximoCola: time.Hour,
				TiempoLimitePredeterminado: 30 * time.Second, Secretos: func() []string { return nil }})
		if err := p.Iniciar(context.Background()); err != nil {
			t.Fatal(err)
		}
		dos := func() int32 {
			claude.maximo.Store(0)
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
						t.Fatalf("%s: %s %v", id, tr.Estado, err)
					}
				}
			}
			return claude.maximo.Load()
		}
		if n := dos(); n != 2 {
			t.Errorf("servidor %v, clave de API: %d a la vez", servidor, n)
		}
		if err := adaptadores.CambiarAutenticacion(registro, "claude", adaptadores.AutenticacionSuscripcion); err != nil {
			t.Fatal(err)
		}
		if n, quiere := dos(), map[bool]int32{true: 1, false: 2}[servidor]; n != quiere {
			t.Errorf("servidor %v, suscripción en marcha: %d a la vez, se esperaban %d", servidor, n, quiere)
		}
		p.Detener()
		alm.Cerrar()
	}
}
