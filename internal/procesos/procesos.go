// Package procesos supervisa los procesos de los agentes: los lanza en un grupo propio (Job Object en Windows,
// grupo de procesos en Unix) para que cancelar o agotar el tiempo termine el árbol completo.
package procesos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// PlazoTerminacion es lo que se espera a que el árbol termine tras pedir su parada antes de declarar
// cancelacion_no_confirmada.
var PlazoTerminacion = 10 * time.Second

const maximoErrores = 16 << 10

// Orden describe un proceso. La entrada va por stdin: el prompt nunca se pasa como argumento.
type Orden struct {
	Programa   string
	Argumentos []string
	Directorio string
	Entorno    []string
	Entrada    string
}

// Resultado es el final del proceso: código de salida y las últimas líneas de stderr.
type Resultado struct {
	CodigoSalida int
	Errores      string
}

type claveAlIniciar struct{}

// ConAlIniciar devuelve un contexto con el que Ejecutar llamará a f con el PID y la firma de inicio del proceso
// en cuanto esté supervisado; así se puede terminar si el servicio cae.
func ConAlIniciar(ctx context.Context, f func(pid int, firma string)) context.Context {
	return context.WithValue(ctx, claveAlIniciar{}, f)
}

// TerminarHuerfano termina el árbol de un proceso que quedó vivo tras una caída del servicio, solo si su firma de
// inicio coincide: un PID reutilizado por otro programa nunca se toca. Devuelve true si lo terminó.
func TerminarHuerfano(pid int, firma string) bool {
	if pid <= 0 || firma == "" || firmaInicio(pid) != firma {
		return false
	}
	return terminarArbol(pid) == nil
}

// Ejecutar lanza la orden y llama a linea por cada línea de stdout según llega. Si ctx se cancela, termina el
// árbol y devuelve ctx.Err() solo cuando confirma la terminación; si no puede confirmarla, devuelve
// cancelacion_no_confirmada con el PID para el diagnóstico.
func Ejecutar(ctx context.Context, o Orden, linea func(string) error) (Resultado, error) {
	cmd := exec.Command(o.Programa, o.Argumentos...)
	cmd.Dir, cmd.Env, cmd.Stdin = o.Directorio, o.Entorno, strings.NewReader(o.Entrada)
	salida := &lineas{alLinea: linea}
	errores := &cola{limite: maximoErrores}
	cmd.Stdout, cmd.Stderr = salida, errores
	// Un nieto que herede stdout no debe bloquear Wait cuando el proceso principal ya terminó.
	cmd.WaitDelay = 2 * time.Second
	preparar(cmd)
	if err := cmd.Start(); err != nil {
		return Resultado{}, nucleo.NuevoError(nucleo.CodigoAgenteNoDisponible, idioma.Tf("no se pudo lanzar el agente: %s", err.Error()))
	}
	g, err := agrupar(cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return Resultado{}, fmt.Errorf(idioma.T("no se pudo supervisar el proceso: %w"), err)
	}
	defer g.cerrar() // elimina cualquier descendiente que siga vivo
	if alIniciar, ok := ctx.Value(claveAlIniciar{}).(func(int, string)); ok {
		alIniciar(cmd.Process.Pid, firmaInicio(cmd.Process.Pid))
	}

	fin := make(chan error, 1)
	go func() { fin <- cmd.Wait() }()
	var errEspera error
	select {
	case errEspera = <-fin:
	case <-ctx.Done():
		g.terminar()
		select {
		case <-fin:
			return Resultado{CodigoSalida: -1, Errores: errores.String()}, ctx.Err()
		case <-time.After(PlazoTerminacion):
			return Resultado{CodigoSalida: -1}, nucleo.NuevoError(nucleo.CodigoCancelacionNoConfirmada,
				idioma.Tf("el proceso %d no terminó tras %s", cmd.Process.Pid, PlazoTerminacion))
		}
	}
	salida.vaciar()
	r := Resultado{CodigoSalida: cmd.ProcessState.ExitCode(), Errores: errores.String()}
	if salida.err != nil {
		return r, salida.err
	}
	var salidaProceso *exec.ExitError
	if errEspera != nil && !errors.As(errEspera, &salidaProceso) && !errors.Is(errEspera, exec.ErrWaitDelay) {
		return r, errEspera
	}
	return r, nil
}

// lineas parte stdout en líneas. Tras el primer error de alLinea descarta el resto sin cortar el proceso:
// así el agente no se bloquea escribiendo en una tubería llena.
type lineas struct {
	mu      sync.Mutex
	buf     []byte
	alLinea func(string) error
	err     error
}

func (l *lineas) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf = append(l.buf, p...)
	for {
		i := bytes.IndexByte(l.buf, '\n')
		if i < 0 {
			break
		}
		l.entregar(string(bytes.TrimRight(l.buf[:i], "\r")))
		l.buf = l.buf[i+1:]
	}
	return len(p), nil
}

func (l *lineas) vaciar() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.buf) > 0 {
		l.entregar(string(l.buf))
		l.buf = nil
	}
}

func (l *lineas) entregar(texto string) {
	if l.err == nil && texto != "" {
		l.err = l.alLinea(texto)
	}
}

// cola conserva solo los últimos bytes de stderr.
type cola struct {
	mu     sync.Mutex
	buf    []byte
	limite int
}

func (c *cola) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buf = append(c.buf, p...)
	if exceso := len(c.buf) - c.limite; exceso > 0 {
		c.buf = c.buf[exceso:]
	}
	return len(p), nil
}

func (c *cola) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.TrimSpace(string(c.buf))
}
