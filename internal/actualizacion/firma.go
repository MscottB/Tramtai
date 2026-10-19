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

// clavesPublicasB64 son las públicas con las que una firma vale. Para cambiar la del secreto FIRMA_ACTUALIZACIONES, un
// release (firmado aún con la anterior) añade la nueva aquí; después se cambia el secreto y, más adelante, se quita la
// vieja (docs/equipo/versionado.md).
var clavesPublicasB64 = []string{"6lZqaLao2eDv8cR5MaVkhXt+9G4luy8pDoHYo3sq1BA="}

// clavesPublicas es variable para que las pruebas firmen con su propio par.
var clavesPublicas = func() []ed25519.PublicKey {
	var claves []ed25519.PublicKey
	for _, b := range clavesPublicasB64 {
		claves = append(claves, desdeBase64(b))
	}
	return claves
}()

func desdeBase64(texto string) []byte {
	datos, _ := base64.StdEncoding.DecodeString(texto)
	return datos
}

// VerificarFirma comprueba que firma (en base64) es la de sumas con alguna de las claves públicas de la app. Publicar la
// usa antes de crear el release (scripts/firmar -comprobar), con las mismas claves que las copias instaladas.
func VerificarFirma(sumas, firma []byte) error {
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(firma)))
	if err == nil && len(sig) == ed25519.SignatureSize {
		for _, clave := range clavesPublicas {
			if len(clave) == ed25519.PublicKeySize && ed25519.Verify(clave, sumas, sig) {
				return nil
			}
		}
	}
	return errors.New(idioma.T("la firma de esta versión no es válida: descárgala desde su página"))
}
