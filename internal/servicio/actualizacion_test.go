package servicio

import (
	"io"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"tramtai/internal/actualizacion"
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
