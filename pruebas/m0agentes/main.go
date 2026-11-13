//go:build m0agentes && windows

// Package main es la sonda del M0 de la v0.60 en Windows Server (docs/equipo/releases/v0.60-preparar-agentes/m0.md):
// registrada como servicio con su cuenta virtual (NT SERVICE\<nombre>), ejecuta la orden de sus argumentos con la
// salida en un archivo y detiene el servicio cuando la orden termina. Así se ve si esa cuenta puede instalar un CLI y
// lanzar su inicio de sesión (scripts/agentes/probar-instalacion.ps1). La usó el M0:
//
//	go build -tags m0agentes -o C:\ProgramData\TramtaiM0Agentes\sonda.exe ./pruebas/m0agentes
//	sc.exe create TramtaiM0Agentes binPath= "C:\ProgramData\TramtaiM0Agentes\sonda.exe <salida.log> <programa> <args…>" obj= "NT SERVICE\TramtaiM0Agentes"
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/windows/svc"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "uso: sonda <salida.log> <programa> [argumentos…] (arrancada por el SCM)")
		os.Exit(2)
	}
	if err := svc.Run(filepath.Base(os.Args[0]), manejador{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type manejador struct{}

func (manejador) Execute(_ []string, peticiones <-chan svc.ChangeRequest, estado chan<- svc.Status) (bool, uint32) {
	estado <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	salida, err := os.Create(os.Args[1])
	if err != nil {
		return false, 1
	}
	defer salida.Close()
	orden := exec.Command(os.Args[2], os.Args[3:]...)
	orden.Stdout, orden.Stderr = salida, salida
	if err := orden.Start(); err != nil {
		fmt.Fprintln(salida, "no arranca:", err)
		return false, 1
	}
	fin := make(chan error, 1)
	go func() { fin <- orden.Wait() }()
	for {
		select {
		case err := <-fin:
			fmt.Fprintf(salida, "\n=== la orden terminó: %v\n", err)
			return false, 0
		case p := <-peticiones:
			if p.Cmd == svc.Stop || p.Cmd == svc.Shutdown {
				// El árbol entero: pwsh y lo que lanzó (el instalador o el inicio de sesión del CLI).
				if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(orden.Process.Pid)).Run(); err != nil {
					fmt.Fprintln(salida, "taskkill:", err)
					_ = orden.Process.Kill()
				}
				<-fin
				fmt.Fprintln(salida, "\n=== detenida")
				return false, 0
			}
			estado <- p.CurrentStatus
		}
	}
}
