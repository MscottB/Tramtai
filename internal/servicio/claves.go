package servicio

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"tramtai/internal/api"
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

// LeerOCrearToken lee el token del archivo o crea uno aleatorio (permisos 0600, fuera del repositorio).
func LeerOCrearToken(ruta string) (string, error) {
	if datos, err := os.ReadFile(ruta); err == nil {
		if token := strings.TrimSpace(string(datos)); len(token) >= 32 {
			return token, nil
		}
	}
	return RotarToken(ruta)
}

// RotarToken genera un token nuevo y lo guarda; el anterior deja de valer al recargar.
func RotarToken(ruta string) (string, error) {
	token := "tt_" + aleatorio(32)
	return token, guardarSecreto(ruta, token)
}

// guardarSecreto escribe un token o su huella solo legible por el usuario, de forma atómica: quien lo lee a la vez (un
// Recargar, la CLI) nunca ve un archivo a medias que tomaría por vacío y regeneraría.
func guardarSecreto(ruta, valor string) error {
	return escribirAtomico(ruta, []byte(valor+"\n"), 0o600)
}

// RotarTokenPunto genera la clave nueva de un punto de acceso y guarda solo su huella: la clave se muestra una vez
// y no se puede recuperar; la anterior deja de valer al recargar.
func RotarTokenPunto(ruta string) (string, error) {
	token := "tt_" + aleatorio(32)
	return token, guardarSecreto(ruta, api.HuellaToken(token))
}

// LeerHuellaPunto devuelve la huella de la clave de un punto. Un archivo antiguo con la clave en claro se convierte
// a huella y las aplicaciones siguen funcionando; sin archivo se crea una clave que nadie conoce, así que hay que
// regenerarla para usar el punto.
func LeerHuellaPunto(ruta string) (string, error) {
	if datos, err := os.ReadFile(ruta); err == nil {
		contenido := strings.TrimSpace(string(datos))
		if strings.HasPrefix(contenido, api.PrefijoHuella) {
			return contenido, nil
		}
		if len(contenido) >= 32 {
			huella := api.HuellaToken(contenido)
			return huella, guardarSecreto(ruta, huella)
		}
	}
	token, err := RotarTokenPunto(ruta)
	return api.HuellaToken(token), err
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
func TokenInstancia(datos, id string, c *nucleo.Configuracion) (valor, origen string, err error) {
	if id == "global" {
		if variable := c.Texto("instancia_global.token_variable_entorno"); variable != "" {
			if v := os.Getenv(variable); v != "" {
				return v, "variable " + variable, nil
			}
		}
	}
	ruta := RutaTokenInstancia(datos, id)
	valor, err = LeerOCrearToken(ruta)
	return valor, ruta, err
}

func aleatorio(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
