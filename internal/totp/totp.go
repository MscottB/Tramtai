// Package totp genera y comprueba los códigos del segundo factor del administrador del servidor: RFC 6238 con SHA-1,
// 6 dígitos y pasos de 30 s, con la biblioteca estándar. El secreto son 20 bytes en base32, como lo leen las apps de
// autenticación.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const paso = 30 // segundos

// Generar crea un secreto nuevo: 20 bytes aleatorios en base32 (32 caracteres, sin relleno).
func Generar() (string, error) {
	datos := make([]byte, 20)
	if _, err := rand.Read(datos); err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(datos), nil
}

// decodificar admite el secreto como lo copian las apps: en minúsculas, con espacios y sin relleno.
func decodificar(secreto string) ([]byte, error) {
	limpio := strings.ToUpper(strings.ReplaceAll(strings.TrimRight(secreto, "="), " ", ""))
	clave, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(limpio)
	if err == nil && len(clave) == 0 {
		err = fmt.Errorf("secreto vacío")
	}
	return clave, err
}

// codigoDe es el código de 6 dígitos de un paso (RFC 4226 con el contador de RFC 6238).
func codigoDe(clave []byte, n int64) string {
	mac := hmac.New(sha1.New, clave)
	_ = binary.Write(mac, binary.BigEndian, uint64(n))
	suma := mac.Sum(nil)
	corte := suma[len(suma)-1] & 0x0f
	valor := binary.BigEndian.Uint32(suma[corte:corte+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", valor%1_000_000)
}

// Codigo es el código del secreto en el instante t.
func Codigo(secreto string, t time.Time) (string, error) {
	clave, err := decodificar(secreto)
	if err != nil {
		return "", err
	}
	return codigoDe(clave, t.Unix()/paso), nil
}

// Valido comprueba el código en el paso de t y en el anterior y el siguiente (±30 s de desfase de reloj). Un paso igual o
// anterior a ultimoPaso (el último que se aceptó) ya no vale: un código visto no se reutiliza. Devuelve el paso aceptado.
func Valido(secreto, codigo string, t time.Time, ultimoPaso int64) (int64, bool) {
	clave, err := decodificar(secreto)
	if err != nil || len(codigo) != 6 {
		return 0, false
	}
	actual := t.Unix() / paso
	for n := actual - 1; n <= actual+1; n++ {
		if n > ultimoPaso && subtle.ConstantTimeCompare([]byte(codigoDe(clave, n)), []byte(codigo)) == 1 {
			return n, true
		}
	}
	return 0, false
}

// URI es el enlace otpauth:// que las apps de autenticación leen como código QR o a mano.
func URI(secreto, cuenta string) string {
	return "otpauth://totp/Tramtai:" + url.PathEscape(cuenta) + "?secret=" + url.QueryEscape(secreto) + "&issuer=Tramtai"
}
