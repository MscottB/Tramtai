//go:build !linux

package servidor

import (
	"context"
	"os"
)

// Instalar no está disponible en este sistema.
func Instalar(Opciones) error { return noDisponible() }

// Desinstalar no está disponible en este sistema.
func Desinstalar(bool) error { return noDisponible() }

// Estado no está disponible en este sistema.
func Estado(context.Context) (EstadoSistema, error) { return EstadoSistema{}, noDisponible() }

// Reiniciar no está disponible en este sistema.
func Reiniciar() error { return noDisponible() }

// EsAdministrador dice si el proceso corre como root.
func EsAdministrador() bool { return os.Geteuid() == 0 }

// AjustarDueno no hace nada: el dueño de lo que escribe la CLI solo importa con la cuenta del servicio de Linux.
func AjustarDueno(string) error { return nil }
