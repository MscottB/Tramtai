package servicio

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"tramtai/internal/agentes"
	"tramtai/internal/api"
	"tramtai/internal/configuracion"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// El administrador del servidor: su contraseña (solo la huella argon2id, en datos/administrador), los códigos de
// recuperación (solo sus huellas, en datos/administrador-recuperacion) y el secreto TOTP, en el almacén cifrado con el
// nombre TRAMTAI_TOTP. Lo que atiende la Web UI está en administrador_ui.go.
const (
	archivoAdministrador = "administrador"
	archivoRecuperacion  = "administrador-recuperacion"
	nombreTOTP           = "TRAMTAI_TOTP"
	largoMinimo          = 12
)

// totpSoloCLI: en Linux el servicio no puede escribir en systemd-creds, así que el TOTP se activa y se quita con la CLI
// (servidor.GuardarClave, como root); las pruebas lo cambian.
var totpSoloCLI = runtime.GOOS == "linux"

// muRecuperacion: un código de recuperación se gasta una sola vez aunque lleguen dos a la vez.
var muRecuperacion sync.Mutex

// FijarContrasena guarda la huella argon2id (m=64 MiB, t=3, p=1, sal de 16 bytes, clave de 32) de la contraseña del
// administrador, en formato PHC. Pide 12 caracteres como mínimo.
func FijarContrasena(datos, c string) error {
	if utf8.RuneCountInString(c) < largoMinimo {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("la contraseña necesita al menos %d caracteres", largoMinimo))
	}
	sal := make([]byte, 16)
	if _, err := rand.Read(sal); err != nil {
		return err
	}
	huella := argon2.IDKey([]byte(c), sal, 3, 64*1024, 1, 32)
	linea := fmt.Sprintf("$argon2id$v=%d$m=65536,t=3,p=1$%s$%s\n", argon2.Version, base64.RawStdEncoding.EncodeToString(sal), base64.RawStdEncoding.EncodeToString(huella))
	return configuracion.EscribirAtomico(filepath.Join(datos, archivoAdministrador), []byte(linea), 0o600)
}

// leerAdministrador es la línea PHC de la contraseña ("" si no hay administrador o no se puede leer).
func leerAdministrador(datos string) string {
	linea, err := os.ReadFile(filepath.Join(datos, archivoAdministrador))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(linea))
}

// ComprobarContrasena compara la contraseña con la huella guardada, en tiempo constante. Sin administrador: false, nil.
func ComprobarContrasena(datos, c string) (bool, error) {
	linea, err := os.ReadFile(filepath.Join(datos, archivoAdministrador))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return comprobarLinea(strings.TrimSpace(string(linea)), c)
}

// errAdministradorDanado: la línea de datos/administrador no es una huella argon2id. El mensaje no lleva la ruta, que
// no debe llegar a un cliente sin sesión; quien lo atiende la anota en el registro.
var errAdministradorDanado = nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida,
	idioma.T("el archivo del administrador está dañado: vuelve a poner la contraseña con «tramtai --servidor administrador contraseña»"))

// comprobarLinea compara la contraseña con una línea PHC de argon2id, con sus parámetros.
func comprobarLinea(linea, c string) (bool, error) {
	partes := strings.Split(linea, "$")
	var memoria, pasadas uint32
	var hilos uint8
	var sal, huella []byte
	var err error
	if len(partes) == 6 && partes[1] == "argon2id" && partes[2] == fmt.Sprintf("v=%d", argon2.Version) {
		if _, err = fmt.Sscanf(partes[3], "m=%d,t=%d,p=%d", &memoria, &pasadas, &hilos); err == nil {
			if sal, err = base64.RawStdEncoding.DecodeString(partes[4]); err == nil {
				huella, err = base64.RawStdEncoding.DecodeString(partes[5])
			}
		}
	} else {
		err = errors.New("formato")
	}
	if err != nil || len(huella) == 0 || pasadas == 0 || hilos == 0 {
		return false, errAdministradorDanado
	}
	calculada := argon2.IDKey([]byte(c), sal, pasadas, memoria, hilos, uint32(len(huella)))
	return subtle.ConstantTimeCompare(calculada, huella) == 1, nil
}

// normalizarCodigo deja un código de recuperación como se genera (XXXXX-XXXXX en mayúsculas): se acepta en minúsculas,
// sin guion o con espacios.
func normalizarCodigo(c string) string {
	limpio := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(c))
	if len(limpio) != 10 {
		return limpio
	}
	return limpio[:5] + "-" + limpio[5:]
}

// NuevosCodigosRecuperacion crea 10 códigos de recuperación de un solo uso (XXXXX-XXXXX en base32) y guarda solo sus
// huellas; los anteriores dejan de valer. Esta es la única vez que se ven.
func NuevosCodigosRecuperacion(datos string) ([]string, error) {
	codigos := make([]string, 10)
	var huellas strings.Builder
	for i := range codigos {
		bytes := make([]byte, 7) // 56 bits; se usan 50, 10 caracteres de base32
		if _, err := rand.Read(bytes); err != nil {
			return nil, err
		}
		codigos[i] = normalizarCodigo(base32.StdEncoding.EncodeToString(bytes)[:10])
		huellas.WriteString(api.HuellaToken(codigos[i]) + "\n")
	}
	muRecuperacion.Lock()
	defer muRecuperacion.Unlock()
	if err := configuracion.EscribirAtomico(filepath.Join(datos, archivoRecuperacion), []byte(huellas.String()), 0o600); err != nil {
		return nil, err
	}
	return codigos, nil
}

// UsarCodigoRecuperacion comprueba un código de recuperación y, si vale, lo borra: no vuelve a valer.
func UsarCodigoRecuperacion(datos, c string) (bool, error) {
	muRecuperacion.Lock()
	defer muRecuperacion.Unlock()
	ruta := filepath.Join(datos, archivoRecuperacion)
	contenido, err := os.ReadFile(ruta)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	buscada := []byte(api.HuellaToken(normalizarCodigo(c)))
	var quedan strings.Builder
	encontrado := false
	for linea := range strings.Lines(string(contenido)) {
		linea = strings.TrimSpace(linea)
		if linea == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(linea), buscada) == 1 && !encontrado {
			encontrado = true
			continue
		}
		quedan.WriteString(linea + "\n")
	}
	if !encontrado {
		return false, nil
	}
	return true, configuracion.EscribirAtomico(ruta, []byte(quedan.String()), 0o600)
}

// BorrarCodigosRecuperacion quita los códigos de recuperación (al quitar el TOTP ya no sirven).
func BorrarCodigosRecuperacion(datos string) error {
	muRecuperacion.Lock()
	defer muRecuperacion.Unlock()
	if err := os.Remove(filepath.Join(datos, archivoRecuperacion)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// ComprobarQuitarTOTP rechaza quitar el TOTP mientras un agente use la suscripción personal: en remoto la exige (spec §5).
func ComprobarQuitarTOTP(c *configuracion.Configuracion) error {
	for _, f := range agentes.Todos() {
		if c.Booleano("adaptadores." + f.Nombre + ".suscripcion_personal") {
			return nucleo.NuevoError(nucleo.CodigoAccionNoPermitida, idioma.Tf("no se puede quitar el TOTP mientras %s usa tu suscripción personal en el servidor: pásalo antes a clave de API (adaptadores.%s.autenticacion: clave_api)", f.Nombre, f.Nombre))
		}
	}
	return nil
}

// errTOTPSoloCLI es el 409 de Linux: el servicio no puede escribir en systemd-creds.
func errTOTPSoloCLI() error {
	return nucleo.NuevoError(nucleo.CodigoAccionNoPermitida, idioma.T("en Linux el servicio no puede escribir en systemd-creds: el TOTP se activa con «sudo tramtai-servidor administrador totp activar» y se quita con «sudo tramtai-servidor administrador totp quitar»"))
}

// normalizarSecreto deja el secreto TOTP en base32 en mayúsculas, sin espacios ni relleno, y exige sus 20 bytes.
func normalizarSecreto(secreto string) (string, error) {
	limpio := strings.ToUpper(strings.TrimRight(strings.ReplaceAll(secreto, " ", ""), "="))
	if datos, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(limpio); err != nil || len(datos) != 20 {
		return "", nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("el secreto TOTP tiene que ser de 20 bytes en base32"))
	}
	return limpio, nil
}

// fijarTOTP guarda el secreto TOTP en el almacén del propio servicio (Windows: el de NT SERVICE\Tramtai).
func (s *Servicio) fijarTOTP(secreto string) error {
	limpio, err := normalizarSecreto(secreto)
	if err != nil {
		return err
	}
	if totpSoloCLI {
		return errTOTPSoloCLI()
	}
	return guardarCredencial(nombreTOTP, limpio)
}

// quitarTOTP borra el secreto TOTP y los códigos de recuperación, si no hay suscripción personal. Lee la configuración
// con la que arrancó el servicio (s.opc.Config): la suscripción personal se cambia con la CLI, que reinicia el servicio.
func (s *Servicio) quitarTOTP() error {
	if err := ComprobarQuitarTOTP(s.opc.Config); err != nil {
		return err
	}
	if totpSoloCLI {
		return errTOTPSoloCLI()
	}
	if err := borrarCredencial(nombreTOTP); err != nil {
		return err
	}
	return BorrarCodigosRecuperacion(s.opc.Datos)
}

// hayTOTP dice si el administrador tiene TOTP; si el almacén no responde, cuenta como sí (para no entrar sin él).
func hayTOTP() (bool, error) {
	secreto, err := leerCredencial(nombreTOTP)
	return secreto != "" || err != nil, err
}
