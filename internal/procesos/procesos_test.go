package procesos

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestMain permite que el binario de pruebas actúe como proceso hijo: así las pruebas no dependen de la shell.
func TestMain(m *testing.M) {
	switch os.Getenv("PROCESOS_HIJO") {
	case "eco":
		datos := make([]byte, 1<<16)
		n, _ := os.Stdin.Read(datos)
		os.Stdout.WriteString("linea 1\nentrada: " + strings.TrimSpace(string(datos[:n])) + "\nsin salto")
		os.Stderr.WriteString("aviso")
		os.Exit(3)
	case "dormir":
		time.Sleep(time.Minute)
		os.Exit(0)
	case "arbol":
		// Lanza un nieto que dormiría un minuto e informa su PID.
		nieto := exec.Command(os.Args[0])
		nieto.Env = append(os.Environ(), "PROCESOS_HIJO=dormir")
		if err := nieto.Start(); err != nil {
			os.Exit(9)
		}
		os.Stdout.WriteString("nieto " + strconv.Itoa(nieto.Process.Pid) + "\n")
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func orden(modo, entrada string) Orden {
	return Orden{Programa: os.Args[0], Entorno: append(os.Environ(), "PROCESOS_HIJO="+modo), Entrada: entrada}
}

func TestLineasEntradaYCodigo(t *testing.T) {
	var recibidas []string
	r, err := Ejecutar(context.Background(), orden("eco", "hola"), func(l string) error {
		recibidas = append(recibidas, l)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(recibidas, "|") != "linea 1|entrada: hola|sin salto" {
		t.Fatalf("líneas %q", recibidas)
	}
	if r.CodigoSalida != 3 || r.Errores != "aviso" {
		t.Fatalf("resultado %+v", r)
	}
}

func TestCancelarTerminaElArbol(t *testing.T) {
	ctx, cancelar := context.WithCancel(context.Background())
	nieto := make(chan int, 1)
	listo := make(chan error, 1)
	go func() {
		_, err := Ejecutar(ctx, orden("arbol", ""), func(l string) error {
			if pid, ok := strings.CutPrefix(l, "nieto "); ok {
				n, _ := strconv.Atoi(pid)
				nieto <- n
			}
			return nil
		})
		listo <- err
	}()
	var pid int
	select {
	case pid = <-nieto:
	case <-time.After(20 * time.Second):
		t.Fatal("el hijo no informó a su nieto")
	}
	inicio := time.Now()
	cancelar()
	select {
	case err := <-listo:
		if err != context.Canceled {
			t.Fatalf("se esperaba context.Canceled: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("la cancelación no terminó")
	}
	if time.Since(inicio) > 5*time.Second {
		t.Fatalf("la cancelación tardó %s", time.Since(inicio))
	}
	limite := time.Now().Add(5 * time.Second)
	for vivo(pid) {
		if time.Now().After(limite) {
			t.Fatalf("el nieto %d sigue vivo: el árbol no se terminó", pid)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
