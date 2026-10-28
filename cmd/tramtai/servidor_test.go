package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"

	"tramtai/internal/actualizacion"
	"tramtai/internal/servidor"
	"tramtai/internal/terminos"
)

// simularServidor sustituye lo que toca el sistema en «tramtai servidor»: si es administrador, instalar y desinstalar
// (que anotan lo que reciben) y el modo servidor. Devuelve las llamadas anotadas.
func simularServidor(t *testing.T, administrador bool) (instalado *[]servidor.Opciones, desinstalado *[]bool) {
	t.Helper()
	instalado, desinstalado = &[]servidor.Opciones{}, &[]bool{}
	a, i, d, u, e := esAdministrador, instalarServidor, desinstalarServidor, usarServidor, estadoServidor
	t.Cleanup(func() {
		esAdministrador, instalarServidor, desinstalarServidor, usarServidor, estadoServidor = a, i, d, u, e
	})
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
		{"servidor", "actualizar"}, {"server", "update"},
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
		"--servidor actualizar":                 "--servidor servidor actualizar",
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
	// cobra no lo imprime como «Error: …»: es una salida esperada (un fallo de verdad sí se imprime).
	for _, c := range []struct {
		err   error
		error bool
	}{{errReinicioPedido, false}, {errors.New("otro fallo"), true}} {
		var salida bytes.Buffer
		orden := &cobra.Command{Use: "iniciar", RunE: func(cmd *cobra.Command, _ []string) error { return terminarServicio(cmd, c.err) }}
		orden.SetOut(&salida)
		orden.SetErr(&salida)
		orden.SetArgs(nil)
		if err := orden.Execute(); !errors.Is(err, c.err) || strings.Contains(salida.String(), "Error") != c.error {
			t.Errorf("%v: %v, salida %q", c.err, err, salida.String())
		}
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

// TestServidorEstadoExpuesto: «servidor estado» recuerda el riesgo 3 si la configuración del servidor lo abre a la red
// (aquí, con un proxy de confianza); en loopback sin proxy, no.
func TestServidorEstadoExpuesto(t *testing.T) {
	simularServidor(t, true)
	estadoServidor = func(context.Context) (servidor.EstadoSistema, error) {
		return servidor.EstadoSistema{Instalado: true, Activo: "active"}, nil
	}
	riesgo := terminos.Riesgos(true)[2]
	for config, quiere := range map[string]bool{
		"servicio:\n  proxy:\n    confiables: [127.0.0.1/32]\n": true,
		"servicio:\n  host: 127.0.0.1\n":                        false,
	} {
		global := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(global, []byte(config), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
		salida, err := ejecutarServidor(t, "", "servidor", "estado")
		if err != nil || strings.Contains(salida, riesgo) != quiere {
			t.Errorf("%q: %v, riesgo 3 en la salida %v: %q", config, err, !quiere, salida)
		}
	}
}

// TestServidorEstadoLetsEncrypt: «servidor estado» recuerda el acuerdo de suscriptor de Let's Encrypt si está activado.
func TestServidorEstadoLetsEncrypt(t *testing.T) {
	simularServidor(t, true)
	estadoServidor = func(context.Context) (servidor.EstadoSistema, error) {
		return servidor.EstadoSistema{Instalado: true, Activo: "active"}, nil
	}
	for config, quiere := range map[string]bool{letsencryptConfig: true, "servicio:\n  host: 127.0.0.1\n": false} {
		global := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(global, []byte(config), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
		salida, err := ejecutarServidor(t, "", "servidor", "estado")
		if err != nil || strings.Contains(salida, servidor.AvisoLetsEncrypt()) != quiere {
			t.Errorf("%q: %v, aviso de Let's Encrypt en la salida %v: %q", config, err, !quiere, salida)
		}
	}
}

// TestServidorReiniciarLetsEncrypt: «servidor reiniciar» recuerda el acuerdo de suscriptor con letsencrypt, pero no el
// riesgo 3 (ese es de «estado»).
func TestServidorReiniciarLetsEncrypt(t *testing.T) {
	simularServidor(t, true)
	anterior := reiniciarServidor
	reiniciarServidor = func() error { return nil }
	t.Cleanup(func() { reiniciarServidor = anterior })
	for config, quiere := range map[string]bool{letsencryptConfig: true, "servicio:\n  host: 127.0.0.1\n": false} {
		global := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(global, []byte(config), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
		salida, err := ejecutarServidor(t, "", "servidor", "reiniciar")
		if err != nil || strings.Contains(salida, servidor.AvisoLetsEncrypt()) != quiere || strings.Contains(salida, terminos.Riesgos(true)[2]) {
			t.Errorf("%q: %v, aviso de Let's Encrypt en la salida %v: %q", config, err, !quiere, salida)
		}
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

// TestServidorActualizar: «tramtai-servidor actualizar» (y update) llega a Actualizar; si ya está al día lo dice sin
// fallar, y si lo instaló el paquete, remite a apt o dnf con error. Desde otra copia lo hace el programa instalado, y
// esta no hace nada más.
func TestServidorActualizar(t *testing.T) {
	simularServidor(t, true)
	anterior, delegar0 := actualizarServidor, delegarActualizar
	t.Cleanup(func() { actualizarServidor, delegarActualizar = anterior, delegar0 })
	delegarActualizar = func(io.Writer, io.Writer) (bool, error) { return true, nil }
	actualizarServidor = func(context.Context, func(context.Context, string) (string, error)) error {
		t.Error("con el instalado delegado, esta copia no actualiza")
		return nil
	}
	if salida, err := ejecutarServidor(t, "", "servidor", "actualizar"); err != nil || strings.Contains(salida, "actualizado") {
		t.Errorf("delegado: %v %q", err, salida)
	}
	delegarActualizar = func(io.Writer, io.Writer) (bool, error) { return false, nil }
	for _, c := range []struct {
		res    error
		args   []string
		salida string
		falla  bool
	}{
		{nil, []string{"--servidor", "actualizar"}, "servidor actualizado", false},
		{nil, []string{"server", "update"}, "servidor actualizado", false},
		{actualizacion.ErrAlDia, []string{"servidor", "actualizar"}, actualizacion.ErrAlDia.Error(), false},
		{servidor.ErrInstaladoPorPaquete, []string{"servidor", "actualizar"}, "", true},
	} {
		llamadas := 0
		actualizarServidor = func(context.Context, func(context.Context, string) (string, error)) error { llamadas++; return c.res }
		salida, err := ejecutarServidor(t, "", ordenDeServidor(nuevaRaiz(), c.args)...)
		if llamadas != 1 || (err != nil) != c.falla || !strings.Contains(salida, c.salida) {
			t.Errorf("%v con %v: %d llamadas, %v, %q", c.args, c.res, llamadas, err, salida)
		}
	}
}
