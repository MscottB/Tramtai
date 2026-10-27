//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows/svc"
)

// dentroDelServicio evita que iniciarServicio, llamado desde el manejador, vuelva a entrar en svc.Run.
var dentroDelServicio atomic.Bool

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
		err := iniciarServicio(cmd, archivo, false)
		if err != nil && archivo != "" { // sin consola: que el error quede en el registro (tramtai-servidor registro)
			if f, e := os.OpenFile(archivo, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600); e == nil {
				fmt.Fprintf(f, "%s %v\n", time.Now().Format(time.RFC3339), err)
				f.Close()
			}
		}
		return err
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
			if m.err != nil { // un código propio (1, o codigoReinicio): la recuperación lo vuelve a arrancar
				return true, uint32(codigoSalida(m.err))
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
