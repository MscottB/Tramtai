package servicio

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/carpetas"
	"tramtai/internal/contrato"
	"tramtai/internal/version"
)

func TestAcercaDe(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	n := nuevoNavegador(t, ts.URL)
	if r, _ := n.pedir("GET", "/ui/acerca", ""); r.StatusCode != 401 {
		t.Fatalf("sin sesión: %d", r.StatusCode)
	}

	codigo, _ := s.CodigoSesion("global")
	n.pedir("GET", "/ui/entrar?codigo="+codigo, "")
	r, cuerpo := n.pedir("GET", "/ui/acerca", "")
	var acerca struct {
		Version  string `json:"version"`
		Servicio struct {
			PID int `json:"pid"`
		} `json:"servicio"`
		Rutas *struct {
			Datos string `json:"datos"`
		} `json:"rutas"`
	}
	if err := json.Unmarshal([]byte(cuerpo), &acerca); err != nil || r.StatusCode != 200 {
		t.Fatalf("sesión local: %d %s %v", r.StatusCode, cuerpo, err)
	}
	if acerca.Version != version.Texto() || acerca.Servicio.PID != os.Getpid() || acerca.Rutas == nil || acerca.Rutas.Datos != s.opc.Datos {
		t.Fatalf("versión, pid y carpeta de datos en una sesión local: %s", cuerpo)
	}

	// Una sesión que no es de este equipo no ve las rutas: llevan el nombre del usuario del sistema.
	remoto := nuevoNavegador(t, ts.URL)
	codigo, _ = s.CodigoSesion("global")
	remoto.pedir("GET", "/ui/entrar?codigo="+codigo, "", "X-Forwarded-For", "203.0.113.5")
	r, cuerpo = remoto.pedir("GET", "/ui/acerca", "", "X-Forwarded-For", "203.0.113.5")
	if r.StatusCode != 200 || strings.Contains(cuerpo, `"rutas"`) || !strings.Contains(cuerpo, `"version"`) {
		t.Fatalf("sesión remota sin rutas: %d %s", r.StatusCode, cuerpo)
	}
}

// TestAcercaEntorno: /ui/acerca dice de qué entorno es el servicio (servidor, portable o escritorio) y, en el servidor, con
// qué cuenta del sistema corre.
func TestAcercaEntorno(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	acerca := func() contrato.Acerca {
		t.Helper()
		n := nuevoNavegador(t, ts.URL)
		codigo, _ := s.CodigoSesion("global")
		n.pedir("GET", "/ui/entrar?codigo="+codigo, "")
		r, cuerpo := n.pedir("GET", "/ui/acerca", "")
		var a contrato.Acerca
		if err := json.Unmarshal([]byte(cuerpo), &a); err != nil || r.StatusCode != 200 || !strings.Contains(cuerpo, `"entorno"`) {
			t.Fatalf("GET /ui/acerca: %d %s %v", r.StatusCode, cuerpo, err)
		}
		return a
	}
	if a := acerca(); a.Entorno != carpetas.Entorno() || a.Cuenta != "" {
		t.Errorf("sin entorno en Opciones es el del proceso, sin cuenta: %+v", a)
	}
	s.opc.Entorno = "servidor"
	if a := acerca(); a.Entorno != "servidor" || a.Cuenta == "" {
		t.Errorf("servidor con su cuenta: %+v", a)
	}
}
