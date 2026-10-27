//go:build m0servidor && windows

package main

import "golang.org/x/sys/windows/svc"

func enServicio() bool {
	ok, err := svc.IsWindowsService()
	return err == nil && ok
}

// ejecutarServicio mide, deja el JSON y se queda en marcha hasta que lo detengan. Si falla, el JSON no existe y el
// servicio termina con error: el código de salida es la evidencia.
func ejecutarServicio(ruta, modo string) error {
	return svc.Run("TramtaiM0", manejador{ruta, modo})
}

type manejador struct{ ruta, modo string }

func (m manejador) Execute(_ []string, peticiones <-chan svc.ChangeRequest, estado chan<- svc.Status) (bool, uint32) {
	estado <- svc.Status{State: svc.StartPending}
	if err := escribir(m.ruta, m.modo); err != nil {
		return false, 1
	}
	estado <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop}
	for p := range peticiones {
		if p.Cmd == svc.Stop || p.Cmd == svc.Shutdown {
			break
		}
		if p.Cmd == svc.Interrogate {
			estado <- p.CurrentStatus
		}
	}
	estado <- svc.Status{State: svc.StopPending}
	return false, 0
}
