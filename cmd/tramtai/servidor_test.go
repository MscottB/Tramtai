package main

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"tramtai/internal/servidor"
	"tramtai/internal/terminos"
)

// simularServidor sustituye lo que toca el sistema en «tramtai servidor»: si es administrador, instalar y desinstalar
// (que anotan lo que reciben) y el modo servidor. Devuelve las llamadas anotadas.
func simularServidor(t *testing.T, administrador bool) (instalado *[]servidor.Opciones, desinstalado *[]bool) {
	t.Helper()
	instalado, desinstalado = &[]servidor.Opciones{}, &[]bool{}
	a, i, d, u := esAdministrador, instalarServidor, desinstalarServidor, usarServidor
	t.Cleanup(func() { esAdministrador, instalarServidor, desinstalarServidor, usarServidor = a, i, d, u })
	esAdministrador = func() bool { return administrador }
	instalarServidor = func(o servidor.Opciones) error { *instalado = append(*instalado, o); return nil }
	desinstalarServidor = func(borrar bool) error { *desinstalado = append(*desinstalado, borrar); return nil }
	usarServidor = func() {}
	return instalado, desinstalado
}

// ejecutarServidor corre una orden con lo que el usuario escribiría en la entrada.
func ejecutarServidor(t *testing.T, entrada string, args ...string) (salida string, err error) {
	t.Helper()
	anterior := dirDatos
	datos := t.TempDir()
	dirDatos = func() (string, error) { return datos, nil }
	t.Cleanup(func() { dirDatos = anterior })
	raiz := nuevaRaiz()
	var o bytes.Buffer
	raiz.SetOut(&o)
	raiz.SetErr(&o)
	raiz.SetIn(strings.NewReader(entrada))
	raiz.SetArgs(args)
	err = raiz.Execute()
	return o.String(), err
}

// TestServidorNoAdministrador: sin ser administrador, cada orden lo dice y no hace nada.
func TestServidorNoAdministrador(t *testing.T) {
	instalado, desinstalado := simularServidor(t, false)
	for _, args := range [][]string{
		{"servidor", "instalar", "--acepto-terminos"}, {"server", "install", "--accept-terms"},
		{"servidor", "desinstalar"}, {"server", "uninstall", "--delete-data"},
		{"servidor", "estado"}, {"server", "status"},
		{"servidor", "reiniciar"}, {"server", "restart"},
		{"servidor", "registro"}, {"server", "log"},
	} {
		_, err := ejecutarServidor(t, "s\n", args...)
		if err == nil || !strings.Contains(err.Error(), "administrador") {
			t.Errorf("%v sin ser administrador: %v", args, err)
		}
	}
	if len(*instalado) > 0 || len(*desinstalado) > 0 {
		t.Errorf("sin ser administrador no se instala ni desinstala: %v %v", *instalado, *desinstalado)
	}
}

// TestServidorUsaSuRaiz: «tramtai servidor» trabaja siempre con los datos del servicio del sistema, aunque no se pase
// --servidor; las demás órdenes, no.
func TestServidorUsaSuRaiz(t *testing.T) {
	simularServidor(t, false)
	var modo []string
	usarServidor = func() { modo = append(modo, "servidor") }
	_, _ = ejecutarServidor(t, "", "server", "status")
	_, _ = ejecutarServidor(t, "", "version")
	if len(modo) != 1 {
		t.Errorf("solo «servidor» pone el modo servidor: %v", modo)
	}
}

// TestServidorEnvoltorio: tramtai-servidor lanza «tramtai --servidor "$@"»; ahí las órdenes de «servidor» van sin
// «servidor» (tramtai-servidor instalar) y las demás quedan como están (tramtai-servidor servicio estado).
func TestServidorEnvoltorio(t *testing.T) {
	raiz := nuevaRaiz()
	for args, esperados := range map[string]string{
		"--servidor instalar --acepto-terminos": "--servidor servidor instalar --acepto-terminos",
		"--server status":                       "--server servidor status",
		"--servidor registro -f":                "--servidor servidor registro -f",
		"--servidor servidor estado":            "--servidor servidor estado",
		"--servidor servicio estado":            "--servidor servicio estado",
		"--servidor":                            "--servidor",
		"estado":                                "estado",
	} {
		if r := strings.Join(ordenDeServidor(raiz, strings.Fields(args)), " "); r != esperados {
			t.Errorf("%q: %q, se esperaba %q", args, r, esperados)
		}
	}
}

// TestServidorInstalarTerminos: instalar muestra los cinco riesgos del servidor y el enlace, y solo sigue si se escribe
// «acepto» (o con --acepto-terminos, sin preguntar); queda anotado cómo se aceptaron.
func TestServidorInstalarTerminos(t *testing.T) {
	instalado, _ := simularServidor(t, true)
	salida, err := ejecutarServidor(t, "no\n", "servidor", "instalar")
	if err == nil || len(*instalado) > 0 {
		t.Fatalf("sin escribir «acepto» no se instala: %v %v", err, *instalado)
	}
	for _, riesgo := range append(terminos.Riesgos(true), terminos.URL) {
		if !strings.Contains(salida, riesgo) {
			t.Errorf("falta en la salida: %q", riesgo)
		}
	}
	if _, err := ejecutarServidor(t, " Acepto \n", "servidor", "instalar"); err != nil {
		t.Fatal(err)
	}
	if _, err := ejecutarServidor(t, "", "server", "install", "--accept-terms"); err != nil {
		t.Fatal(err)
	}
	if len(*instalado) != 2 || !(*instalado)[0].AceptoTerminos || (*instalado)[0].Como != terminos.ComoServidorInstalar ||
		(*instalado)[1].Como != terminos.ComoOpcion || (*instalado)[1].Origen == "" {
		t.Errorf("instalaciones: %+v", *instalado)
	}
}

// TestServidorDesinstalarConfirma: --borrar-datos pide confirmación; sin ella, no se hace nada.
func TestServidorDesinstalarConfirma(t *testing.T) {
	_, desinstalado := simularServidor(t, true)
	for _, c := range []struct {
		entrada string
		args    []string
	}{
		{"n\n", []string{"servidor", "desinstalar", "--borrar-datos"}},
		{"s\n", []string{"server", "uninstall", "--delete-data"}},
		{"", []string{"servidor", "desinstalar"}},
	} {
		salida, err := ejecutarServidor(t, c.entrada, c.args...)
		if err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
		if cancelado := strings.Contains(salida, "desinstalación cancelada"); cancelado != (c.entrada == "n\n") {
			t.Errorf("%v con %q: solo al responder que no se dice «cancelada»: %q", c.args, c.entrada, salida)
		}
	}
	if len(*desinstalado) != 2 || !(*desinstalado)[0] || (*desinstalado)[1] {
		t.Errorf("desinstalaciones: %v", *desinstalado)
	}
}

// TestReinicioEnServidor: en el modo servidor, «Reiniciar» de la Web UI no lanza un proceso suelto fuera del sistema:
// el servicio drena y sale con su código propio, y systemd (Restart=on-failure) o la recuperación de Windows lo vuelven
// a arrancar. Fuera del modo servidor lanza el servicio nuevo, como siempre.
func TestReinicioEnServidor(t *testing.T) {
	lanzar0 := lanzar
	t.Cleanup(func() { lanzar = lanzar0 })
	lanzados := 0
	lanzar = func(string, string, ...string) error { lanzados++; return nil }
	var pedido atomic.Bool
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	if err := reinicioDelServicio("datos", "servicio.log", &pedido)(); err != nil || lanzados != 0 || !pedido.Load() {
		t.Errorf("modo servidor: %v, lanzados %d, pedido %v", err, lanzados, pedido.Load())
	}
	if codigoReinicio == 0 || codigoReinicio == 1 || codigoSalida(fmt.Errorf("al parar: %w", errReinicioPedido)) != codigoReinicio ||
		codigoSalida(errors.New("otro fallo")) != 1 {
		t.Errorf("códigos de salida: %d", codigoReinicio)
	}
	t.Setenv("TRAMTAI_SERVIDOR", "")
	pedido.Store(false)
	if err := reinicioDelServicio("datos", "servicio.log", &pedido)(); err != nil || lanzados != 1 || pedido.Load() {
		t.Errorf("sin modo servidor: %v, lanzados %d, pedido %v", err, lanzados, pedido.Load())
	}
}

// TestConversionElevadaEnServidor: en el modo servidor, como administrador, la CLI no ejecuta el Python de la raíz del
// servidor (lo puede cambiar la cuenta del servicio): la conversión la hace el servicio. Los archivos de texto no lo
// necesitan, y sin ser administrador no hay nada que proteger.
func TestConversionElevadaEnServidor(t *testing.T) {
	simularServidor(t, true)
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	if err := conversionLocalPermitida([]string{"informe.pdf"}); err == nil || !strings.Contains(err.Error(), "la conversión la hace el servicio") {
		t.Errorf("PDF como administrador: %v", err)
	}
	if err := conversionLocalPermitida([]string{"notas.txt"}); err != nil {
		t.Errorf("texto: %v", err)
	}
	if _, err := ejecutarServidor(t, "", "convertir", "informe.pdf"); err == nil || !strings.Contains(err.Error(), "la conversión la hace el servicio") {
		t.Errorf("tramtai convertir: %v", err)
	}
	esAdministrador = func() bool { return false }
	if err := conversionLocalPermitida([]string{"informe.pdf"}); err != nil {
		t.Errorf("sin ser administrador: %v", err)
	}
}

// TestServidorInstalarConversor: --conversor (--converter) llega a Instalar con la ruta absoluta del paquete.
func TestServidorInstalarConversor(t *testing.T) {
	instalado, _ := simularServidor(t, true)
	for _, opcion := range []string{"--conversor", "--converter"} {
		if _, err := ejecutarServidor(t, "", "servidor", "instalar", "--acepto-terminos", opcion, "conversor.zip"); err != nil {
			t.Fatal(err)
		}
	}
	quiere, _ := filepath.Abs("conversor.zip")
	if len(*instalado) != 2 || (*instalado)[0].Conversor != quiere || (*instalado)[1].Conversor != quiere {
		t.Errorf("instalaciones: %+v", *instalado)
	}
}

// TestServidorDesinstalarAviso: un Aviso de Desinstalar (el perfil que Windows aún tiene cargado) se muestra, pero la
// desinstalación se dio por hecha.
func TestServidorDesinstalarAviso(t *testing.T) {
	simularServidor(t, true)
	desinstalarServidor = func(bool) error { return servidor.Aviso("el perfil sigue cargado") }
	salida, err := ejecutarServidor(t, "s\n", "servidor", "desinstalar", "--borrar-datos")
	if err != nil || !strings.Contains(salida, "el perfil sigue cargado") || !strings.Contains(salida, "servidor desinstalado") {
		t.Errorf("con aviso: %v %q", err, salida)
	}
}
