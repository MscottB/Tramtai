//go:build unix

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// TestSIGTERMDrena: systemd detiene el servicio con SIGTERM. Un proceso hijo con un trabajo en curso que espera la
// parada como iniciarServicio (contextoDeParada) recibe SIGTERM, termina el trabajo y sale bien, en lugar de morir.
func TestSIGTERMDrena(t *testing.T) {
	if os.Getenv("TRAMTAI_PRUEBA_SIGTERM") == "1" {
		ctx, parar := contextoDeParada(context.Background())
		defer parar()
		fmt.Println("trabajando")
		<-ctx.Done()
		time.Sleep(300 * time.Millisecond) // el drenaje: el trabajo en curso termina
		fmt.Println("trabajo terminado")
		os.Exit(0)
	}
	hijo := exec.Command(os.Args[0], "-test.run=^TestSIGTERMDrena$")
	hijo.Env = append(os.Environ(), "TRAMTAI_PRUEBA_SIGTERM=1")
	salida, err := hijo.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := hijo.Start(); err != nil {
		t.Fatal(err)
	}
	lineas := bufio.NewScanner(salida)
	if !lineas.Scan() || lineas.Text() != "trabajando" {
		t.Fatalf("el hijo no empezó el trabajo: %q", lineas.Text())
	}
	if err := hijo.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	terminado := lineas.Scan() && lineas.Text() == "trabajo terminado"
	if err := hijo.Wait(); err != nil || !terminado {
		t.Errorf("con SIGTERM el trabajo termina antes de salir: terminado=%v, salida del hijo: %v", terminado, err)
	}
}
