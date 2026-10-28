package main

import (
	"bytes"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"tramtai/internal/servicio"
	"tramtai/internal/servidor"
	"tramtai/internal/totp"
)

// simularTOTP fija los datos del servidor en una carpeta y sustituye su almacén (systemd-creds o la API del servicio).
func simularTOTP(t *testing.T) (datos string, guardado *string) {
	t.Helper()
	datos = t.TempDir()
	guardado = new(string)
	a, f, q := dirDatos, fijarTOTPServidor, quitarTOTPServidor
	t.Cleanup(func() { dirDatos, fijarTOTPServidor, quitarTOTPServidor = a, f, q })
	dirDatos = func() (string, error) { return datos, nil }
	fijarTOTPServidor = func(secreto string) error { *guardado = secreto; return nil }
	quitarTOTPServidor = func() error { *guardado = ""; return nil }
	return datos, guardado
}

// ejecutarAdmin corre una orden con lo que se escribiría en la entrada, sobre los datos de simularTOTP.
func ejecutarAdmin(t *testing.T, entrada string, args ...string) (string, error) {
	t.Helper()
	raiz := nuevaRaiz()
	var o bytes.Buffer
	raiz.SetOut(&o)
	raiz.SetErr(&o)
	raiz.SetIn(strings.NewReader(entrada))
	raiz.SetArgs(args)
	err := raiz.Execute()
	return o.String(), err
}

// TestAdministradorContrasena: sin ser administrador no toca nada; pide la contraseña dos veces y, si no coinciden o es
// corta, no guarda; si coinciden, queda la huella en datos/administrador. También con los alias en inglés.
func TestAdministradorContrasena(t *testing.T) {
	simularServidor(t, false)
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	datos, _ := simularTOTP(t)
	if _, err := ejecutarAdmin(t, "una contraseña larga\nuna contraseña larga\n", "administrador", "contraseña"); err == nil || !strings.Contains(err.Error(), "administrador") {
		t.Fatalf("sin ser administrador: %v", err)
	}
	simularServidor(t, true)
	if _, err := ejecutarAdmin(t, "una contraseña larga\notra cosa distinta\n", "admin", "password"); err == nil || !strings.Contains(err.Error(), "no coinciden") {
		t.Fatalf("distintas: %v", err)
	}
	if _, err := ejecutarAdmin(t, "corta\ncorta\n", "admin", "password"); err == nil || !strings.Contains(err.Error(), "12") {
		t.Fatalf("corta: %v", err)
	}
	if _, err := os.Stat(filepath.Join(datos, "administrador")); err == nil {
		t.Fatal("no se guarda nada si no vale")
	}
	salida, err := ejecutarAdmin(t, "una contraseña larga\nuna contraseña larga\n", "administrador", "contraseña")
	if err != nil || !strings.Contains(salida, "guardada") || strings.Contains(salida, "una contraseña larga") {
		t.Fatalf("guardar: %v\n%s", err, salida)
	}
	if ok, err := servicio.ComprobarContrasena(datos, "una contraseña larga"); !ok || err != nil {
		t.Fatalf("la huella: %v %v", ok, err)
	}
}

// TestAdministradorTOTPActivar: enseña el secreto y el enlace, y solo lo guarda si el código que se escribe vale; luego
// da los 10 códigos de recuperación. Sin contraseña no se activa.
func TestAdministradorTOTPActivar(t *testing.T) {
	simularServidor(t, true)
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	datos, guardado := simularTOTP(t)
	secreto, _ := totp.Generar()
	anterior := generarTOTP
	t.Cleanup(func() { generarTOTP = anterior })
	generarTOTP = func() (string, error) { return secreto, nil }
	codigo, _ := totp.Codigo(secreto, time.Now())
	if _, err := ejecutarAdmin(t, codigo+"\n", "administrador", "totp", "activar"); err == nil || !strings.Contains(err.Error(), "contraseña") {
		t.Fatalf("sin contraseña: %v", err)
	}
	if err := servicio.FijarContrasena(datos, "una contraseña larga"); err != nil {
		t.Fatal(err)
	}
	malo := map[bool]string{true: "111111", false: "000000"}[codigo == "000000"]
	salida, err := ejecutarAdmin(t, malo+"\n", "admin", "totp", "enable")
	if err == nil || *guardado != "" || !strings.Contains(salida, "otpauth://totp/Tramtai:") || !strings.Contains(salida, secreto) {
		t.Fatalf("con un código que no vale: %v %q\n%s", err, *guardado, salida)
	}
	salida, err = ejecutarAdmin(t, codigo+"\n", "administrador", "totp", "activar")
	if err != nil || *guardado != secreto {
		t.Fatalf("activar: %v %q\n%s", err, *guardado, salida)
	}
	if n := len(regexp.MustCompile(`[A-Z2-7]{5}-[A-Z2-7]{5}`).FindAllString(salida, -1)); n != 10 {
		t.Fatalf("códigos de recuperación: %d\n%s", n, salida)
	}
}

// TestArranqueConTOTP: iniciarServicio rellena Entrada.TOTP con el secreto del almacén cuando la regla 4 puede aplicarse
// (expuesto y con suscripción personal): sin secreto no arranca y lo dice; con él, sí. Sin exponer no pregunta al almacén.
func TestArranqueConTOTP(t *testing.T) {
	anterior := leerClave
	t.Cleanup(func() { leerClave = anterior })
	secreto, preguntas := "", 0
	leerClave = func(nombre string) (string, error) {
		preguntas++
		if nombre != "TRAMTAI_TOTP" {
			t.Errorf("pregunta por %s", nombre)
		}
		return secreto, nil
	}
	expuesta := servidor.Entrada{Host: "127.0.0.1", Confiables: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, TLS: "ninguno",
		Suscripciones: []servidor.Suscripcion{{Agente: "claude", Personal: true}}}
	if err := servidor.ComprobarArranque(conTOTP(expuesta)); err == nil || !strings.Contains(err.Error(), "TOTP") {
		t.Fatalf("sin secreto: %v", err)
	}
	secreto = "SECRETO"
	if err := servidor.ComprobarArranque(conTOTP(expuesta)); err != nil {
		t.Fatalf("con secreto: %v", err)
	}
	preguntas = 0
	if conTOTP(servidor.Entrada{Host: "127.0.0.1", TLS: "ninguno", Suscripciones: expuesta.Suscripciones}); preguntas != 0 {
		t.Fatal("sin exponer no se pregunta al almacén")
	}
}

// TestQuitarTOTPConSuscripcionPersonal (CLI): con adaptadores.claude.suscripcion_personal no se quita el TOTP y dice
// por qué; sin ella, se quita y se borran los códigos de recuperación.
func TestQuitarTOTPConSuscripcionPersonal(t *testing.T) {
	simularServidor(t, true)
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	datos, guardado := simularTOTP(t)
	*guardado = "SECRETO"
	global := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	if err := os.WriteFile(global, []byte("adaptadores:\n  claude:\n    suscripcion_personal: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ejecutarAdmin(t, "", "admin", "totp", "disable"); err == nil || !strings.Contains(err.Error(), "suscripción personal") || *guardado == "" {
		t.Fatalf("con suscripción personal: %v %q", err, *guardado)
	}
	if err := os.WriteFile(global, []byte("adaptadores:\n  claude:\n    suscripcion_personal: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := servicio.NuevosCodigosRecuperacion(datos); err != nil {
		t.Fatal(err)
	}
	if salida, err := ejecutarAdmin(t, "", "administrador", "totp", "quitar"); err != nil || *guardado != "" {
		t.Fatalf("quitar: %v %q\n%s", err, *guardado, salida)
	}
	if _, err := os.Stat(filepath.Join(datos, "administrador-recuperacion")); err == nil {
		t.Fatal("los códigos de recuperación se borran")
	}
}
