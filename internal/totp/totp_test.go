package totp

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

// rfc es el secreto de los vectores de RFC 6238 (SHA-1): el ASCII «12345678901234567890» en base32.
var rfc = base32.StdEncoding.EncodeToString([]byte("12345678901234567890"))

// TestTOTPVectores: los de RFC 6238 en 6 dígitos (los dos últimos de los de 8).
func TestTOTPVectores(t *testing.T) {
	for _, c := range []struct {
		t      int64
		quiere string
	}{{59, "287082"}, {1111111109, "081804"}, {1111111111, "050471"}, {1234567890, "005924"}, {2000000000, "279037"}} {
		codigo, err := Codigo(rfc, time.Unix(c.t, 0))
		if err != nil || codigo != c.quiere {
			t.Errorf("t=%d: %q %v, se esperaba %s", c.t, codigo, err, c.quiere)
		}
	}
	// El secreto se acepta en minúsculas y con espacios, como lo copian las apps.
	if codigo, err := Codigo(strings.ToLower(rfc[:8])+" "+rfc[8:], time.Unix(59, 0)); err != nil || codigo != "287082" {
		t.Errorf("en minúsculas y con espacios: %q %v", codigo, err)
	}
	if _, err := Codigo("no es base32!", time.Now()); err == nil {
		t.Error("un secreto que no es base32 da error")
	}
}

// TestTOTPMargen: vale un paso antes y uno después (±30 s); a ±90 s, no.
func TestTOTPMargen(t *testing.T) {
	ahora := time.Unix(1111111109, 0)
	for _, c := range []struct {
		desfase time.Duration
		vale    bool
	}{{0, true}, {-30 * time.Second, true}, {30 * time.Second, true}, {-90 * time.Second, false}, {90 * time.Second, false}} {
		codigo, _ := Codigo(rfc, ahora.Add(c.desfase))
		if _, ok := Valido(rfc, codigo, ahora, 0); ok != c.vale {
			t.Errorf("desfase %s: %v", c.desfase, ok)
		}
	}
	if _, ok := Valido(rfc, "12345", ahora, 0); ok {
		t.Error("un código de 5 dígitos no vale")
	}
}

// TestTOTPNoReutiliza: un paso ya usado (ni uno anterior) no vuelve a valer.
func TestTOTPNoReutiliza(t *testing.T) {
	ahora := time.Unix(1111111109, 0)
	codigo, _ := Codigo(rfc, ahora)
	paso, ok := Valido(rfc, codigo, ahora, 0)
	if !ok || paso != ahora.Unix()/30 {
		t.Fatalf("primera vez: %d %v", paso, ok)
	}
	if _, ok := Valido(rfc, codigo, ahora, paso); ok {
		t.Fatal("el mismo código en el mismo paso no vale dos veces")
	}
	anterior, _ := Codigo(rfc, ahora.Add(-30*time.Second))
	if _, ok := Valido(rfc, anterior, ahora, paso); ok {
		t.Fatal("el del paso anterior no vale tras usar uno posterior")
	}
	siguiente, _ := Codigo(rfc, ahora.Add(30*time.Second))
	if p, ok := Valido(rfc, siguiente, ahora.Add(30*time.Second), paso); !ok || p != paso+1 {
		t.Fatalf("el del paso siguiente sí: %d %v", p, ok)
	}
}

func TestGenerarYURI(t *testing.T) {
	a, err := Generar()
	b, _ := Generar()
	datos, errDecodificar := base32.StdEncoding.DecodeString(a)
	if err != nil || errDecodificar != nil || len(datos) != 20 || a == b {
		t.Fatalf("Generar: %q %q %v %v", a, b, err, errDecodificar)
	}
	if u := URI("ABC", "tramtai.ejemplo.org"); u != "otpauth://totp/Tramtai:tramtai.ejemplo.org?secret=ABC&issuer=Tramtai" {
		t.Fatal(u)
	}
}
