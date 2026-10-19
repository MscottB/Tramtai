package actualizacion

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"strings"

	"tramtai/internal/idioma"
)

// archivoFirma es la firma ed25519 de SHA256SUMS.txt que Publicar sube a cada versión (scripts/firmar). Sin ella, o si no
// es válida, la actualización de un clic no instala: quien pudiera cambiar el instalador de una versión también podría
// cambiar sus sumas, pero no firmarlas.
const archivoFirma = archivoSumas + ".sig"

// clavePublicaB64 es la pública del secreto FIRMA_ACTUALIZACIONES de Publicar. Cambiarla exige un release firmado aún con
// la anterior que ya la incluya (docs/equipo/versionado.md).
const clavePublicaB64 = "6lZqaLao2eDv8cR5MaVkhXt+9G4luy8pDoHYo3sq1BA="

// clavePublica es variable para que las pruebas firmen con su propio par.
var clavePublica = ed25519.PublicKey(desdeBase64(clavePublicaB64))

func desdeBase64(texto string) []byte {
	datos, _ := base64.StdEncoding.DecodeString(texto)
	return datos
}

// verificarFirma comprueba que firma (en base64) es la de sumas con clavePublica.
func verificarFirma(sumas, firma []byte) error {
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(firma)))
	if err != nil || len(sig) != ed25519.SignatureSize || len(clavePublica) != ed25519.PublicKeySize || !ed25519.Verify(clavePublica, sumas, sig) {
		return errors.New(idioma.T("la firma de esta versión no es válida: descárgala desde su página"))
	}
	return nil
}
