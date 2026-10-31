//go:build windows

package main

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows/svc"
)

// dentroDelServicio evita que iniciarServicio, llamado desde el manejador, vuelva a entrar en svc.Run.
var dentroDelServicio atomic.Bool

// comoServicioWindows dice si este proceso es el servicio que lanzó el administrador de servicios: corre con su
// propia cuenta (NT SERVICE\Tramtai), aunque su testigo cuente como elevado.
func comoServicioWindows() bool { return dentroDelServicio.Load() }

// ejecutarComoServicio corre iniciarServicio bajo el administrador de servicios si este proceso es el servicio de
// Windows (tramtai servidor instalar): detenerlo o apagar el equipo cancela su contexto, que drena como «tramtai
// servicio detener». Dice false si no es un servicio y hay que seguir como siempre.
func ejecutarComoServicio(cmd *cobra.Command, archivo string) (bool, error) {
	if dentroDelServicio.Load() {
		return false, nil
	}
	if es, _ := svc.IsWindowsService(); !es { // sin saberlo, como en una consola
		return false, nil
	}
	dentroDelServicio.Store(true)
	m := &manejador{ejecutar: func(ctx context.Context) error {
		cmd.SetContext(ctx)
		return iniciarServicio(cmd, archivo, false) // el error queda en el registro (anotarError)
	}}
	// El nombre no cuenta con un servicio de proceso propio: vale también para el de las pruebas.
	if err := svc.Run("Tramtai", m); err != nil {
		return true, err
	}
	return true, m.err
}

// manejador es el servicio para el administrador de servicios: ejecutar corre hasta que se cancela su contexto.
type manejador struct {
	ejecutar func(context.Context) error
	err      error
}

// Execute informa StartPending y Running y, al pedir detener o apagar, cancela el contexto e informa StopPending con un
// paso más cada pocos segundos mientras drena, para que Windows no lo dé por colgado.
func (m *manejador) Execute(_ []string, peticiones <-chan svc.ChangeRequest, estado chan<- svc.Status) (bool, uint32) {
	estado <- svc.Status{State: svc.StartPending}
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	fin := make(chan error, 1)
	go func() { fin <- m.ejecutar(ctx) }()
	// PreShutdown llega antes del apagado y da más margen para drenar que Shutdown.
	estado <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPreShutdown}
	var paso <-chan time.Time
	parando := svc.Status{State: svc.StopPending, WaitHint: 10_000}
	for {
		select {
		case m.err = <-fin:
			// Con la configuración inválida sale limpio: la recuperación no lo arranca una y otra vez (fallaría igual);
			// el registro dice por qué.
			if m.err != nil && !configuracionMala(m.err) {
				return true, uint32(codigoSalida(m.err)) // 1, o codigoReinicio: la recuperación lo vuelve a arrancar
			}
			return false, 0
		case <-paso:
			parando.CheckPoint++
			estado <- parando
		case p := <-peticiones:
			switch p.Cmd {
			case svc.Interrogate:
				estado <- p.CurrentStatus
			case svc.Stop, svc.Shutdown, svc.PreShutdown:
				if paso == nil {
					cancelar()
					estado <- parando
					paso = time.Tick(5 * time.Second)
				}
			}
		}
	}
}
