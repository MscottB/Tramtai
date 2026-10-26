package servicio

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"tramtai/internal/api"
	"tramtai/internal/configuracion"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// RotarTokenInstancia genera la clave nueva de una instancia y la aplica en marcha: la anterior deja de valer al
// momento, sin reiniciar. Una clave que llega de una variable de entorno se cambia en esa variable.
func (s *Servicio) RotarTokenInstancia(inst *Instancia) (string, error) {
	if strings.HasPrefix(inst.ArchivoToken, "variable ") {
		return "", nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("la clave de %s viene de la %s: cámbiala allí", inst.ID, inst.ArchivoToken))
	}
	// El archivo y la memoria cambian juntos: dos rotaciones a la vez no pueden dejarlos distintos.
	s.mu.Lock()
	defer s.mu.Unlock()
	token, err := RotarToken(inst.ArchivoToken)
	if err != nil {
		return "", err
	}
	inst.token = token
	return token, nil
}

// tokenDe lee la clave de una instancia, que puede cambiar en marcha (RotarTokenInstancia).
func (s *Servicio) tokenDe(inst *Instancia) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return inst.token
}

// instanciaPorToken compara el token con el de todas las instancias, sin revelar por tiempos cuál existe.
func (s *Servicio) instanciaPorToken(token string) *Instancia {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var elegida *Instancia
	for _, inst := range s.instancias {
		if token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(inst.token)) == 1 {
			elegida = inst
		}
	}
	return elegida
}

// largoMinimoEntorno es el largo mínimo de una clave que llega por variable de entorno: 32 caracteres (128 bits en
// hexadecimal). No se exige el largo de las generadas para no romper a quien ya usa una de 64 en hexadecimal.
const largoMinimoEntorno = 32

// LeerOCrearToken lee el token del archivo o, si no existe, crea uno aleatorio (permisos 0600, fuera del repositorio).
// Si existe pero no se puede leer o está truncado, da error: rotarlo en silencio dejaría a las apps con 401.
func LeerOCrearToken(ruta string) (string, error) {
	contenido, err := leerSecreto(ruta)
	if errors.Is(err, fs.ErrNotExist) {
		return RotarToken(ruta)
	}
	return contenido, err
}

// leerSecreto lee un token o su huella; uno de menos de 32 caracteres es un archivo dañado, no una clave.
func leerSecreto(ruta string) (string, error) {
	datos, err := os.ReadFile(ruta)
	if err != nil {
		return "", err
	}
	if contenido := strings.TrimSpace(string(datos)); len(contenido) >= 32 {
		return contenido, nil
	}
	return "", nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("la clave de %s está dañada: bórrala para crear otra (las apps tendrán que usar la nueva)", ruta))
}

// RotarToken genera un token nuevo y lo guarda; el anterior deja de valer al recargar.
func RotarToken(ruta string) (string, error) {
	token := "tt_" + aleatorio(32)
	return token, guardarSecreto(ruta, token)
}

// guardarSecreto escribe un token o su huella solo legible por el usuario, de forma atómica: quien lo lee a la vez (un
// Recargar, la CLI) nunca ve un archivo a medias que tomaría por vacío y regeneraría.
func guardarSecreto(ruta, valor string) error {
	return configuracion.EscribirAtomico(ruta, []byte(valor+"\n"), 0o600)
}

// NuevaClave genera una clave nueva y guarda solo su huella: la clave se muestra una vez y no se puede recuperar; la
// anterior deja de valer al recargar. Es siempre nueva, aunque quedara una de otro punto o espacio con el mismo nombre.
func NuevaClave(ruta string) (string, error) {
	token := "tt_" + aleatorio(32)
	return token, guardarSecreto(ruta, api.HuellaToken(token))
}

// leerHuella devuelve la huella de la clave guardada en ruta. Un archivo antiguo con la clave en claro se convierte a
// huella y las aplicaciones siguen funcionando. Sin archivo da un error con fs.ErrNotExist; uno que no se puede leer o
// está truncado da error y no se toca.
func leerHuella(ruta string) (string, error) {
	contenido, err := leerSecreto(ruta)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(contenido, api.PrefijoHuella) {
		return contenido, nil
	}
	huella := api.HuellaToken(contenido)
	return huella, guardarSecreto(ruta, huella)
}

// LeerHuellaPunto devuelve la huella de la clave de un punto. Sin archivo se crea una clave que nadie conoce, así que
// hay que regenerarla para usar el punto.
func LeerHuellaPunto(ruta string) (string, error) {
	huella, err := leerHuella(ruta)
	if errors.Is(err, fs.ErrNotExist) {
		token, err := NuevaClave(ruta)
		return api.HuellaToken(token), err
	}
	return huella, err
}

// EliminarTokenPunto borra la clave de un punto eliminado: otro con el mismo nombre no hereda la anterior.
func EliminarTokenPunto(datos, nombre string) {
	_ = os.Remove(RutaTokenPunto(datos, nombre))
}

// EliminarTokenInstancia borra el token de una instancia, si existe (al olvidar el espacio).
func EliminarTokenInstancia(datos, id string) {
	_ = os.Remove(RutaTokenInstancia(datos, id))
}

// RutaTokenInstancia es el archivo del token de una instancia (el global conserva su ruta histórica).
func RutaTokenInstancia(datos, id string) string {
	if id == "global" {
		return filepath.Join(datos, "token-global")
	}
	return filepath.Join(datos, "tokens", id)
}

// RutaTokenPunto es el archivo del token de un punto de acceso.
func RutaTokenPunto(datos, nombre string) string {
	return filepath.Join(datos, "tokens", "acceso_"+nombre)
}

// TokenInstancia devuelve el token de la instancia y su origen; el global puede venir de una variable de entorno.
func TokenInstancia(datos, id string, c *configuracion.Configuracion) (valor, origen string, err error) {
	if id == "global" {
		if variable := c.Texto("instancia_global.token_variable_entorno"); variable != "" {
			if v := os.Getenv(variable); v != "" {
				if len(v) < largoMinimoEntorno {
					return "", "", nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf(
						"la clave de la variable %s es demasiado corta: necesita al menos %d caracteres", variable, largoMinimoEntorno))
				}
				return v, "variable " + variable, nil
			}
		}
	}
	ruta := RutaTokenInstancia(datos, id)
	valor, err = LeerOCrearToken(ruta)
	return valor, ruta, err
}

// HuellaInstancia devuelve la huella de la clave de la instancia y su origen; la del global puede venir de una variable
// de entorno. Un espacio sin archivo no tiene clave: devuelve la huella vacía y la ruta, sin crear nada.
func HuellaInstancia(datos, id string, c *configuracion.Configuracion) (huella, origen string, err error) {
	if id == "global" {
		if variable := c.Texto("instancia_global.token_variable_entorno"); variable != "" {
			if v := os.Getenv(variable); v != "" {
				if len(v) < largoMinimoEntorno {
					return "", "", nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf(
						"la clave de la variable %s es demasiado corta: necesita al menos %d caracteres", variable, largoMinimoEntorno))
				}
				return api.HuellaToken(v), "variable " + variable, nil
			}
		}
	}
	ruta := RutaTokenInstancia(datos, id)
	huella, err = leerHuella(ruta)
	if errors.Is(err, fs.ErrNotExist) {
		return "", ruta, nil
	}
	return huella, ruta, err
}

func aleatorio(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
