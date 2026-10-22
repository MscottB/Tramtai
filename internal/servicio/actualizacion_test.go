package servicio

import (
	"context"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"tramtai/internal/actualizacion"
	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

// cuentaAtras prepara un servicio con un instalador simulado y una espera corta; devuelve cuántas veces instaló.
func cuentaAtras(t *testing.T) (*Servicio, *atomic.Int32, string) {
	t.Helper()
	var instaladas atomic.Int32
	antes, esperaAntes := instalar, esperaInstalacion
	instalar = func(string, string) error { instaladas.Add(1); return nil }
	esperaInstalacion = 80 * time.Millisecond
	t.Cleanup(func() { instalar, esperaInstalacion = antes, esperaAntes })
	dir := t.TempDir() + "/descarga"
	os.MkdirAll(dir, 0o755)
	return &Servicio{opc: Opciones{Registros: io.Discard}}, &instaladas, dir
}

func esperarFase(t *testing.T, s *Servicio, fase string) {
	t.Helper()
	for range 100 {
		if s.progresoActualizacion().Fase == fase {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("la fase no llegó a %q: %+v", fase, s.progresoActualizacion())
}

// esperarInstaladas espera a que el instalador simulado se haya llamado n veces (la fase cambia justo antes).
func esperarInstaladas(t *testing.T, instaladas *atomic.Int32, n int32) {
	t.Helper()
	for range 200 {
		if instaladas.Load() == n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("instaló %d veces, esperado %d", instaladas.Load(), n)
}

func TestCuentaAtrasInstalar(t *testing.T) {
	v := actualizacion.Version{Etiqueta: "v9.0.0"}

	t.Run("al vencer instala", func(t *testing.T) {
		s, instaladas, dir := cuentaAtras(t)
		go s.esperarEInstalar(v, dir+"/instalador.exe", dir, "c:/tramtai")
		esperarFase(t, s, "listo")
		if s.progresoActualizacion().InstalaEn == nil {
			t.Fatal("la fase listo indica cuándo instalará")
		}
		esperarFase(t, s, "instalando")
		esperarInstaladas(t, instaladas, 1)
	})
	t.Run("reiniciar ahora instala antes del plazo", func(t *testing.T) {
		s, instaladas, dir := cuentaAtras(t)
		esperaInstalacion = time.Hour
		go s.esperarEInstalar(v, dir+"/instalador.exe", dir, "c:/tramtai")
		esperarFase(t, s, "listo")
		if err := s.decidirInstalacion(true); err != nil {
			t.Fatal(err)
		}
		esperarFase(t, s, "instalando")
		esperarInstaladas(t, instaladas, 1)
	})
	t.Run("cancelar no instala y borra la descarga", func(t *testing.T) {
		s, instaladas, dir := cuentaAtras(t)
		go s.esperarEInstalar(v, dir+"/instalador.exe", dir, "c:/tramtai")
		esperarFase(t, s, "listo")
		if err := s.decidirInstalacion(false); err != nil {
			t.Fatal(err)
		}
		esperarFase(t, s, "")
		time.Sleep(150 * time.Millisecond) // pasado el plazo, tampoco instala
		if _, err := os.Stat(dir); !os.IsNotExist(err) || instaladas.Load() != 0 {
			t.Fatalf("carpeta: %v, instaladas: %d", err, instaladas.Load())
		}
	})
	t.Run("fuera de listo no se decide", func(t *testing.T) {
		s, _, _ := cuentaAtras(t)
		if err := s.decidirInstalacion(true); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
			t.Fatalf("sin cuenta atrás en curso: %v", err)
		}
	})
}

func TestCancelarConfirmadoNuncaInstala(t *testing.T) {
	v := actualizacion.Version{Etiqueta: "v9.0.0"}
	for range 200 {
		s, instaladas, dir := cuentaAtras(t)
		esperaInstalacion = time.Millisecond // el plazo vence casi a la vez que llega el cancelar
		hecho := make(chan struct{})
		go func() { s.esperarEInstalar(v, dir+"/instalador.exe", dir, "c:/tramtai"); close(hecho) }()
		err := s.decidirInstalacion(false)
		for i := 0; i < 2000 && err != nil && s.progresoActualizacion().Fase != "instalando"; i++ {
			time.Sleep(5 * time.Microsecond)
			err = s.decidirInstalacion(false)
		}
		<-hecho
		if err == nil && instaladas.Load() != 0 {
			t.Fatal("se instaló después de un Cancelar confirmado")
		}
	}
}

func TestDetenerCancelaLaCuentaAtras(t *testing.T) {
	s, instaladas, dir := cuentaAtras(t)
	esperaInstalacion = time.Hour
	go s.esperarEInstalar(actualizacion.Version{Etiqueta: "v9.0.0"}, dir+"/instalador.exe", dir, "c:/tramtai")
	esperarFase(t, s, "listo")
	s.Drenar()
	esperarFase(t, s, "")
	if instaladas.Load() != 0 {
		t.Fatal("detener el servicio en la cuenta atrás no instala")
	}
}

func TestRutasCuentaAtras(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	s.ActivarGlobal()
	n := abrirSesion(t, s, ts.URL)
	if r, _ := pedir(t, ts.URL, "POST", "/ui/actualizar/cancelar", "", ""); r.StatusCode != 401 {
		t.Errorf("sin sesión: %d", r.StatusCode)
	}
	if r, _ := n.pedir("POST", "/ui/actualizar/cancelar", ""); r.StatusCode != 401 {
		t.Errorf("sin CSRF: %d", r.StatusCode)
	}
	if r, cuerpo := n.pedir("POST", "/ui/actualizar/instalar", "", "X-CSRF-Token", n.csrf); r.StatusCode != 409 || !strings.Contains(cuerpo, "solicitud_invalida") {
		t.Errorf("fuera de listo: %d %s", r.StatusCode, cuerpo)
	}
	antes, esperaAntes := instalar, esperaInstalacion
	instalar = func(string, string) error { return nil }
	esperaInstalacion = time.Hour
	t.Cleanup(func() { instalar, esperaInstalacion = antes, esperaAntes })
	dir := t.TempDir()
	go s.esperarEInstalar(actualizacion.Version{Etiqueta: "v9.0.0"}, dir+"/i.exe", dir, "c:/tramtai")
	esperarFase(t, s, "listo")
	if r, _ := n.pedir("POST", "/ui/actualizar/cancelar", "", "X-CSRF-Token", n.csrf); r.StatusCode != 204 {
		t.Errorf("en listo: %d", r.StatusCode)
	}
	esperarFase(t, s, "")
}

// Dos «Actualizar» a la vez (el escritorio, la bandeja, otra pestaña): solo uno consulta y descarga; el otro recibe
// «ya en curso» sin ir a la red.
func TestActualizarDosVecesALaVez(t *testing.T) {
	var consultas atomic.Int32
	soltar := make(chan struct{})
	antes := ultimaVersion
	ultimaVersion = func(context.Context) (actualizacion.Version, error) {
		consultas.Add(1)
		<-soltar
		return actualizacion.Version{Etiqueta: "v0.0.1"}, nil // no es más nueva: el primero termina sin descargar
	}
	t.Cleanup(func() { ultimaVersion = antes })
	s := &Servicio{opc: Opciones{Registros: io.Discard}}
	primero := make(chan error, 1)
	go func() { _, err := s.IniciarActualizacion(context.Background()); primero <- err }()
	for consultas.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	if _, err := s.IniciarActualizacion(context.Background()); err == nil || !strings.Contains(nucleo.MensajeDe(err), "en curso") {
		t.Fatalf("la segunda petición tenía que recibir «ya en curso»: %v", err)
	}
	close(soltar)
	<-primero
	if n := consultas.Load(); n != 1 {
		t.Fatalf("se consultó %d veces", n)
	}
	if p := s.progresoActualizacion(); p.Fase != "" {
		t.Fatalf("sin versión nueva no queda nada en curso: %+v", p)
	}
}
