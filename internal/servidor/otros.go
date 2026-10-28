//go:build !linux && !windows

package servidor

import (
	"context"
	"os"
	"os/exec"
)

// sinBloquear no se usa aquí: prepararDatos solo se llega a usar en Linux y en Windows.
const sinBloquear = 0

// Instalar no está disponible en este sistema.
func Instalar(Opciones) error { return noDisponible() }

// Desinstalar no está disponible en este sistema.
func Desinstalar(bool) error { return noDisponible() }

// Estado no está disponible en este sistema.
func Estado(context.Context) (EstadoSistema, error) { return EstadoSistema{}, noDisponible() }

// Reiniciar no está disponible en este sistema.
func Reiniciar() error { return noDisponible() }

// GuardarClave no está disponible en este sistema.
func GuardarClave(string, string) error { return noDisponible() }

// BorrarClave no está disponible en este sistema.
func BorrarClave(string) error { return noDisponible() }

// GuardarTOTP no existe fuera de Linux y Windows.
func GuardarTOTP(string) error { return noDisponible() }

// BorrarTOTP no existe fuera de Linux y Windows.
func BorrarTOTP() error { return noDisponible() }

// EsAdministrador dice si el proceso corre como root.
func EsAdministrador() bool { return os.Geteuid() == 0 }

// AjustarDueno no hace nada: el dueño de lo que escribe la CLI solo importa con la cuenta del servicio de Linux.
func AjustarDueno(string) error { return nil }

func ajustarDueno(*os.Root) error { return nil }

// OrdenEntrar no existe fuera de Linux y Windows.
func OrdenEntrar(string, string, []string) (*exec.Cmd, error) { return nil, noDisponible() }
