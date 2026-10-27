package main

import (
	"bytes"
	"strings"
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
		if _, err := ejecutarServidor(t, c.entrada, c.args...); err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
	}
	if len(*desinstalado) != 2 || !(*desinstalado)[0] || (*desinstalado)[1] {
		t.Errorf("desinstalaciones: %v", *desinstalado)
	}
}
