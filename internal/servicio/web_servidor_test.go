package servicio

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/autoarranque"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/totp"
)

// TestEstadoServidorUI: GET /ui/servidor da a cualquier sesión del servidor (también la remota, la de la contraseña) el
// modo de HTTPS, los dominios, los proxies de confianza, cuándo vence el certificado, si hay administrador con TOTP y las
// suscripciones; la API de administración (la terminal) da lo mismo. Fuera del servidor, 404; sin sesión, 401.
func TestEstadoServidorUI(t *testing.T) {
	dir := t.TempDir()
	certificado, clave := parTemporal(t, dir, "tramtai.ejemplo.org")
	global := "servicio:\n  dominios: [tramtai.ejemplo.org]\n  proxy:\n    confiables: [10.0.0.0/8]\n  tls:\n    modo: archivos\n" +
		"    certificado: " + strconv.Quote(certificado) + "\n    clave: " + strconv.Quote(clave) + "\n" +
		"adaptadores:\n  claude:\n    autenticacion: suscripcion\n    suscripcion_personal: true\n    suscripcion_simultaneas: 2\n"
	secreto, _ := totp.Generar()
	s, ts, seguro := conAdministrador(t, global, secreto)
	s.opc.Expuesto = true
	n := navegadorSeguro(t, seguro)
	if r, _ := n.pedir("GET", "/ui/servidor", ""); r.StatusCode != 401 {
		t.Fatalf("sin sesión: %d", r.StatusCode)
	}
	codigo, _ := totp.Codigo(secreto, time.Now())
	if r, cuerpo := n.acceder(contrasenaPrueba, codigo); r.StatusCode != 200 {
		t.Fatalf("acceder: %d %s", r.StatusCode, cuerpo)
	}
	r, cuerpo := n.pedir("GET", "/ui/servidor", "")
	var e contrato.EstadoServidor
	if err := json.Unmarshal([]byte(cuerpo), &e); err != nil || r.StatusCode != 200 {
		t.Fatalf("GET /ui/servidor: %d %s %v", r.StatusCode, cuerpo, err)
	}
	pemCert, _ := os.ReadFile(certificado)
	bloque, _ := pem.Decode(pemCert)
	x, _ := x509.ParseCertificate(bloque.Bytes)
	if e.TLS != "archivos" || !slices.Equal(e.Dominios, []string{"tramtai.ejemplo.org"}) || !slices.Equal(e.Confiables, []string{"10.0.0.0/8"}) ||
		e.VenceCertificado == nil || !e.VenceCertificado.Equal(x.NotAfter) || !e.Administrador || !e.TOTP || !e.Expuesto ||
		!slices.Equal(e.Suscripciones, []contrato.SuscripcionServidor{{Agente: "claude", Personal: true, Simultaneas: 2}}) {
		t.Errorf("estado del servidor: %+v", e)
	}
	if strings.Contains(cuerpo, clave) || strings.Contains(cuerpo, secreto) {
		t.Fatalf("no lleva rutas de claves ni secretos: %s", cuerpo)
	}
	if r, desdeAdmin := admin(t, s, ts.URL, "GET", "/api/v1/servicio/servidor", ""); r.StatusCode != 200 || desdeAdmin != cuerpo {
		t.Errorf("la API de administración: %d %s", r.StatusCode, desdeAdmin)
	}

	// Sin TLS propio, sin administrador y con la suscripción por clave de API: lo dice, sin vencimiento ni suscripciones.
	os.Remove(filepath.Join(s.opc.Datos, "administrador"))
	s.opc.Config.Fijar(configuracion.OrigenGlobal, map[string]any{"servicio.tls.modo": "ninguno", "adaptadores.claude.autenticacion": "clave_api"})
	if _, cuerpo := admin(t, s, ts.URL, "GET", "/api/v1/servicio/servidor", ""); !strings.Contains(cuerpo, `"tls":"ninguno"`) ||
		!strings.Contains(cuerpo, `"administrador":false`) || !strings.Contains(cuerpo, `"suscripciones":[]`) || strings.Contains(cuerpo, "vence_certificado") {
		t.Errorf("sin TLS ni administrador: %s", cuerpo)
	}

	// Fuera del servidor no existe.
	s.opc.Entorno = "escritorio"
	if r, _ := admin(t, s, ts.URL, "GET", "/api/v1/servicio/servidor", ""); r.StatusCode != 404 {
		t.Errorf("fuera del servidor (administración): %d", r.StatusCode)
	}
	escritorio, tsEscritorio, _ := entorno(t, "", "", adaptadores.Simulado{})
	if _, err := escritorio.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	local := nuevoNavegador(t, tsEscritorio.URL)
	cod, _ := escritorio.CodigoSesion("global")
	local.pedir("GET", "/ui/entrar?codigo="+cod, "")
	if r, _ := local.pedir("GET", "/ui/servidor", ""); r.StatusCode != 404 {
		t.Errorf("fuera del servidor (Web UI): %d", r.StatusCode)
	}
}

// TestServidorSinArranqueNiInstalar: en el servidor no hay arranque con el sistema ni «Instalar ahora»: el servicio del
// sistema arranca con el equipo y se actualiza con tramtai-servidor actualizar. La sesión dice además el entorno.
func TestServidorSinArranqueNiInstalar(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	s.opc.Autoarranque = func() (bool, autoarranque.Modo) { return true, autoarranque.ModoApp }
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	if a := s.ajustesServicio(); a.Autoarranque == nil {
		t.Fatal("en el escritorio sí hay arranque con el sistema")
	}
	s.opc.Entorno = "servidor"
	if a := s.ajustesServicio(); a.Autoarranque != nil || a.ModoArranque != "" {
		t.Errorf("en el servidor no hay arranque con el sistema: %+v", a)
	}
	if e := s.estadoActualizacion(t.Context(), false); e.Instalable {
		t.Errorf("en el servidor no se instala desde la Web UI: %+v", e)
	}
	n := nuevoNavegador(t, ts.URL)
	cod, _ := s.CodigoSesion("global")
	n.pedir("GET", "/ui/entrar?codigo="+cod, "")
	if _, cuerpo := n.pedir("GET", "/ui/sesion", ""); !strings.Contains(cuerpo, `"entorno":"servidor"`) {
		t.Errorf("la sesión dice el entorno: %s", cuerpo)
	}
}
