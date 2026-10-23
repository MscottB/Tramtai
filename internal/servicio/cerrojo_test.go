package servicio

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestCerrojoAyudante no es una prueba: es el «primer servicio» que TestSegundoServicioNoArranca lanza en otro proceso.
func TestCerrojoAyudante(t *testing.T) {
	datos := os.Getenv("TRAMTAI_PRUEBA_CERROJO")
	if datos == "" {
		t.Skip("solo como proceso hijo")
	}
	soltar, err := TomarCerrojo(datos, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer soltar()
	os.Stdout.WriteString("cerrojo tomado\n")
	time.Sleep(time.Minute) // el padre lo mata al acabar
}

func TestSegundoServicioNoArranca(t *testing.T) {
	datos := t.TempDir()
	primero := exec.Command(os.Args[0], "-test.run=^TestCerrojoAyudante$", "-test.v")
	primero.Env = append(os.Environ(), "TRAMTAI_PRUEBA_CERROJO="+datos)
	salida, err := primero.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := primero.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { primero.Process.Kill(); primero.Wait() }()
	lector := bufio.NewScanner(salida)
	for lector.Scan() && !strings.Contains(lector.Text(), "cerrojo tomado") {
	}

	// Otro proceso sobre la misma carpeta: falla con un mensaje claro, también si espera un poco.
	if _, err := TomarCerrojo(datos, 300*time.Millisecond); err == nil || !strings.Contains(err.Error(), "otro servicio de Tramtai") {
		t.Fatalf("el segundo servicio tenía que fallar: %v", err)
	}
	// Dentro del mismo proceso, el cerrojo también es exclusivo; al soltarlo se puede volver a tomar.
	otra := t.TempDir()
	soltar, err := TomarCerrojo(otra, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := TomarCerrojo(otra, 0); err == nil {
		t.Fatal("el cerrojo se tomó dos veces")
	}
	soltar()
	if soltar, err = TomarCerrojo(otra, 0); err != nil {
		t.Fatalf("tras soltarlo: %v", err)
	}
	soltar()

	// Al morir el primero, el sistema suelta el cerrojo: el reinicio, que espera, lo consigue.
	primero.Process.Kill()
	primero.Wait()
	if soltar, err = TomarCerrojo(datos, 10*time.Second); err != nil {
		t.Fatalf("tras cerrar el primero: %v", err)
	}
	soltar()
}
