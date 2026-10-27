//go:build windows

package main

import (
	"context"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc"
)

// TestManejadorDrena: el manejador del servicio de Windows informa StartPending y Running, y un svc.Stop cancela el
// contexto del servicio (que así drena como «tramtai servicio detener») e informa StopPending hasta que termina.
func TestManejadorDrena(t *testing.T) {
	cancelado := make(chan struct{})
	m := &manejador{ejecutar: func(ctx context.Context) error {
		<-ctx.Done()
		close(cancelado)
		time.Sleep(50 * time.Millisecond) // el drenaje
		return nil
	}}
	peticiones, estados := make(chan svc.ChangeRequest), make(chan svc.Status, 16)
	fin := make(chan uint32, 1)
	go func() {
		_, codigo := m.Execute(nil, peticiones, estados)
		fin <- codigo
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
	if e := esperar(svc.Running); e.Accepts&svc.AcceptStop == 0 || e.Accepts&svc.AcceptShutdown == 0 {
		t.Errorf("acepta detener y apagar: %+v", e)
	}
	select {
	case <-cancelado:
		t.Fatal("el contexto se canceló antes de detener")
	default:
	}
	peticiones <- svc.ChangeRequest{Cmd: svc.Stop}
	esperar(svc.StopPending)
	select {
	case <-cancelado:
	case <-time.After(5 * time.Second):
		t.Fatal("svc.Stop no canceló el contexto")
	}
	select {
	case codigo := <-fin:
		if codigo != 0 {
			t.Errorf("código de salida %d", codigo)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Execute no terminó tras el drenaje")
	}
}
