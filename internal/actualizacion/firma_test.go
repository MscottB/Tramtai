package actualizacion

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

// Una clave pública mal copiada haría que ninguna versión se pudiera instalar con un clic.
func TestClavesPublicasIncrustadas(t *testing.T) {
	if len(clavesPublicasB64) == 0 {
		t.Fatal("sin claves públicas no se podría instalar ninguna versión")
	}
	for _, b := range clavesPublicasB64 {
		if len(desdeBase64(b)) != ed25519.PublicKeySize {
			t.Fatalf("no es una clave pública ed25519: %q", b)
		}
	}
	if VerificarFirma([]byte("sumas"), []byte("no es una firma")) == nil {
		t.Fatal("se aceptó una firma que no es base64")
	}
}

// Durante un cambio de clave valen la vieja y la nueva: lo firmado con cualquiera de las dos se acepta.
func TestVariasClaves(t *testing.T) {
	vieja, privadaVieja, _ := ed25519.GenerateKey(nil)
	nueva, privadaNueva, _ := ed25519.GenerateKey(nil)
	_, otra, _ := ed25519.GenerateKey(nil)
	anterior := clavesPublicas
	clavesPublicas = []ed25519.PublicKey{vieja, nueva}
	t.Cleanup(func() { clavesPublicas = anterior })
	sumas := []byte("abc  instalador.exe\n")
	firma := func(p ed25519.PrivateKey) []byte {
		return []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(p, sumas)))
	}
	if VerificarFirma(sumas, firma(privadaVieja)) != nil || VerificarFirma(sumas, firma(privadaNueva)) != nil {
		t.Fatal("una firma con cualquiera de las claves debe valer")
	}
	if VerificarFirma(sumas, firma(otra)) == nil {
		t.Fatal("se aceptó una firma con una clave que no es de la app")
	}
}
