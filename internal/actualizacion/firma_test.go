package actualizacion

import (
	"crypto/ed25519"
	"testing"
)

// Una clave pública mal copiada haría que ninguna versión se pudiera instalar con un clic.
func TestClavePublicaIncrustada(t *testing.T) {
	if len(desdeBase64(clavePublicaB64)) != ed25519.PublicKeySize {
		t.Fatalf("clavePublicaB64 no es una clave pública ed25519: %q", clavePublicaB64)
	}
	if verificarFirma([]byte("sumas"), []byte("no es una firma")) == nil {
		t.Fatal("se aceptó una firma que no es base64")
	}
}
