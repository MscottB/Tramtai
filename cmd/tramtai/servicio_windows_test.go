//go:build windows

package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc"

	"tramtai/internal/nucleo"
)

// TestManejadorDrena: el manejador del servicio de Windows informa StartPending y Running, y un svc.Stop, svc.Shutdown
// o svc.PreShutdown cancela el contexto del servicio (que así drena como «tramtai servicio detener») e informa
// StopPending hasta que termina. Un reinicio pedido desde la Web UI sale con su código propio, para la recuperación; con la
// configuración inválida sale limpio, para que la recuperación no lo arranque una y otra vez (fallaría igual).
func TestManejadorDrena(t *testing.T) {
	for _, c := range []struct {
		nombre    string
		orden     svc.Cmd
		resultado error
		propio    bool
		codigo    uint32
	}{
		{"detener", svc.Stop, nil, false, 0},
		{"apagar", svc.Shutdown, nil, false, 0},
		{"antes de apagar", svc.PreShutdown, nil, false, 0},
		{"reinicio pedido", svc.Stop, errReinicioPedido, true, codigoReinicio},
		{"otro fallo", svc.Stop, errors.New("otro fallo"), true, 1},
		{"configuración inválida", svc.Stop, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, "config.yaml: YAML no válido"), false, 0},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			cancelado := make(chan struct{})
			m := &manejador{ejecutar: func(ctx context.Context) error {
				<-ctx.Done()
				close(cancelado)
				time.Sleep(50 * time.Millisecond) // el drenaje
				return c.resultado
			}}
			peticiones, estados := make(chan svc.ChangeRequest), make(chan svc.Status, 16)
			type salida struct {
				propio bool
				codigo uint32
			}
			fin := make(chan salida, 1)
			go func() {
				propio, codigo := m.Execute(nil, peticiones, estados)
				fin <- salida{propio, codigo}
			}()
			esperar := func(quiere svc.State) svc.Status {
				t.Helper()
				select {
				case e := <-estados:
					if e.State != quiere {
						t.Fatalf("estado %d, quiere %d", e.State, quiere)
					}
					return e
				case <-time.After(5 * time.Second):
					t.Fatalf("sin el estado %d", quiere)
				}
				return svc.Status{}
			}
			esperar(svc.StartPending)
			acepta := svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPreShutdown
			if e := esperar(svc.Running); e.Accepts&acepta != acepta {
				t.Errorf("acepta detener, apagar y antes de apagar: %+v", e)
			}
			select {
			case <-cancelado:
				t.Fatal("el contexto se canceló antes de detener")
			default:
			}
			peticiones <- svc.ChangeRequest{Cmd: c.orden}
			esperar(svc.StopPending)
			select {
			case <-cancelado:
			case <-time.After(5 * time.Second):
				t.Fatal("la orden no canceló el contexto")
			}
			select {
			case s := <-fin:
				if s.propio != c.propio || s.codigo != c.codigo {
					t.Errorf("salida %+v, quiere %v %d", s, c.propio, c.codigo)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("Execute no terminó tras el drenaje")
			}
		})
	}
}
