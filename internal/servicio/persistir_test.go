package servicio

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
)

// Si el registro de espacios no se puede guardar, la acción del usuario no responde que salió bien, y el fallo queda en
// el registro del servicio y en los errores del estado (al reiniciar, el espacio volvería a estar como antes).
func TestFalloAlGuardarElEstado(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	var registro strings.Builder
	s.opc.Registros = &registro
	inst, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	ruta := filepath.Join(s.opc.Datos, "espacios.json")
	os.Remove(ruta)
	if err := os.MkdirAll(filepath.Join(ruta, "ocupado"), 0o700); err != nil { // un directorio: no se puede reemplazar
		t.Fatal(err)
	}
	if err := s.DetenerEspacio(inst.ID); err == nil {
		t.Fatal("detener respondió bien sin poder guardar el registro de espacios")
	}
	if !strings.Contains(registro.String(), "espacios") {
		t.Fatalf("el fallo tiene que quedar en el registro del servicio: %q", registro.String())
	}
	if e := s.Estado(context.Background()); !strings.Contains(strings.Join(e.Errores, " "), "espacios") {
		t.Fatalf("el fallo tiene que verse en el estado: %+v", e.Errores)
	}
}

// Olvidar y activar el mismo espacio se excluyen: no se borran la base y la clave de un espacio que se está abriendo
// (quedaría abierto sobre archivos borrados), y quien lo active mientras se olvida espera y recibe un error.
func TestOlvidarMientrasSeAbre(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	inst, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	id := inst.ID
	claveDe(t, s, inst) // sin clave no hay archivo que comprobar al final
	if err := s.DetenerEspacio(id); err != nil {
		t.Fatal(err)
	}
	dentro, soltar := make(chan struct{}), make(chan struct{})
	conBase(t, func(ruta string) (*persistencia.Almacen, error) {
		close(dentro)
		<-soltar
		return persistencia.Abrir(ruta)
	})
	activado := make(chan error, 1)
	go func() { _, err := s.ActivarEspacio(id); activado <- err }()
	<-dentro
	e, _ := s.registro().buscar(id)
	if err := s.Olvidar(id, e.Nombre); err == nil {
		t.Fatal("se olvidó un espacio mientras se abría")
	}
	close(soltar)
	if err := <-activado; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(RutaTokenInstancia(s.opc.Datos, id)); err != nil {
		t.Fatalf("la clave del espacio abierto se borró: %v", err)
	}
}

// Con el espacio ya activo y un Olvidar en curso (su marca en abriendo), activarlo no responde «activo» con la
// instancia que Olvidar está a punto de cerrar: espera y recibe el error del olvido (hallazgo 9).
func TestActivarActivoMientrasSeOlvida(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	inst, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	olvido := &apertura{listo: make(chan struct{}), err: nucleo.NuevoError(nucleo.CodigoNoEncontrado, "se estaba olvidando")}
	s.mu.Lock()
	s.abriendo[inst.ID] = olvido
	s.mu.Unlock()
	activado := make(chan error, 1)
	go func() { _, err := s.ActivarEspacio(inst.ID); activado <- err }()
	select {
	case err := <-activado:
		t.Fatalf("respondió sin esperar al olvido en curso: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	s.mu.Lock()
	delete(s.abriendo, inst.ID)
	s.mu.Unlock()
	close(olvido.listo)
	if err := <-activado; nucleo.CodigoDe(err) != nucleo.CodigoNoEncontrado {
		t.Fatalf("tenía que recibir el error del olvido: %v", err)
	}
}
