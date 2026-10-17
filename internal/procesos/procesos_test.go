package procesos

import (
	"context"
	"os"
	"os/exec"
	"runtime"
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
	case "linea-larga":
		// 9 MB sin un solo salto de línea.
		trozo := strings.Repeat("x", 1<<20)
		for range 9 {
			os.Stdout.WriteString(trozo)
		}
		os.Exit(0)
	case "memoria":
		datos := make([]byte, 300<<20)
		for i := range datos {
			datos[i] = 1
		}
		os.Stdout.WriteString("reservado\n")
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

func TestTerminarHuerfanoSoloConSuFirma(t *testing.T) {
	type inicio struct {
		pid   int
		firma string
	}
	iniciado := make(chan inicio, 1)
	ctx, cancelar := context.WithCancel(ConAlIniciar(context.Background(), func(p int, f string) { iniciado <- inicio{p, f} }))
	listo := make(chan struct{})
	go func() {
		defer close(listo)
		_, _ = Ejecutar(ctx, orden("dormir", ""), func(string) error { return nil })
	}()
	defer func() { cancelar(); <-listo }()
	var i inicio
	select {
	case i = <-iniciado:
	case <-time.After(5 * time.Second):
	}
	pid, firma := i.pid, i.firma
	if pid == 0 || firma == "" {
		t.Fatalf("no se informó el proceso: %d %q", pid, firma)
	}
	if TerminarHuerfano(pid, firma+"x") {
		t.Fatal("con otra firma (PID reutilizado) no debía terminarlo")
	}
	if !TerminarHuerfano(pid, firma) {
		t.Fatal("con su firma debía terminarlo")
	}
	select {
	case <-listo:
	case <-time.After(5 * time.Second):
		t.Fatal("el proceso siguió vivo")
	}
}

func TestLineaConTope(t *testing.T) {
	_, err := Ejecutar(context.Background(), orden("linea-larga", ""), func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "8 MB") {
		t.Fatalf("una línea de más de 8 MB debe fallar con un error claro: %v", err)
	}
}

func TestMemoriaLimitada(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("macOS no limita la memoria de un proceso hijo")
	}
	var lineas []string
	sinTope := orden("memoria", "")
	if r, err := Ejecutar(context.Background(), sinTope, func(l string) error { lineas = append(lineas, l); return nil }); err != nil || r.CodigoSalida != 0 {
		t.Fatalf("sin tope reserva 300 MB: %+v %v", r, err)
	}
	conTope := orden("memoria", "")
	conTope.MemoriaMB = 100
	lineas = nil
	r, err := Ejecutar(context.Background(), conTope, func(l string) error { lineas = append(lineas, l); return nil })
	if err == nil && r.CodigoSalida == 0 {
		t.Fatalf("con un tope de 100 MB no puede reservar 300 MB: %+v %v", r, lineas)
	}
}
