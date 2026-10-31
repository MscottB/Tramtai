// Package apuntar edita con cuidado los archivos de configuración de otras herramientas (Claude Code, Codex, Continue,
// opencode…) para que usen Tramtai: copia entera antes del primer cambio, anotación de lo escrito y de lo que había,
// escritura atómica y comprobación de que nadie tocó el archivo mientras tanto. El formato de cada archivo es de su Formato.
package apuntar

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"tramtai/internal/configuracion"
	"tramtai/internal/idioma"
)

var (
	// ErrArchivoIlegible lo devuelve un Formato cuando no puede interpretar el archivo: no se toca nada.
	ErrArchivoIlegible = errors.New(idioma.T("archivo ilegible"))
	// ErrCambioMientras: la herramienta (o el usuario) reescribió el archivo durante la edición, también en el reintento.
	ErrCambioMientras = errors.New(idioma.T("el archivo cambió mientras se escribía"))
	// ErrSinCopia: restaurar no tiene a qué volver porque el archivo lo creó Tramtai.
	ErrSinCopia = errors.New(idioma.T("no hay copia: el archivo no existía antes de activar la herramienta"))
)

// Valores es lo que se escribe en la herramienta; cada Formato toma de aquí lo que necesita.
type Valores struct {
	URLAnthropic, URLOpenAI string   // base de cada contrato
	Modelo                  string   // "agente/modelo"
	Clave                   string   // solo para las herramientas con la clave en el archivo
	Orden                   []string // ejecutable absoluto + args, para apiKeyHelper / auth.command
	Puerto                  int
}

// Formato sabe editar un tipo de archivo sin tocar lo que no es de Tramtai. No lee ni escribe en disco: eso es del núcleo.
type Formato interface {
	// Aplicar devuelve el archivo con Tramtai puesto, lo escrito y lo que había antes en esas claves.
	Aplicar(actual []byte, v Valores) (nuevo []byte, escrito, previos map[string]any, err error)
	// Quitar devuelve el archivo sin lo de Tramtai; vacio es que no queda nada más.
	Quitar(actual []byte, a Anotacion) (nuevo []byte, vacio bool, err error)
	// Contiene dice si sigue lo que escribió Tramtai.
	Contiene(actual []byte, a Anotacion) bool
}

// Herramienta describe una herramienta que se puede apuntar a Tramtai.
type Herramienta struct {
	ID, Nombre     string
	ClaveEnArchivo bool // la clave va dentro del archivo; si no, en el almacén y el archivo lleva una orden
	EnProyecto     bool // admite el archivo de un proyecto además del global
	Programa       string
	Ruta           func(alcance Alcance, proyecto string) (string, error)
	Gestionado     func() bool // nil salvo Claude Code: ¿una política de la organización manda sobre su archivo?
	Formato        Formato
}

// Lista son las herramientas conocidas, en el orden en que se muestran.
func Lista() []Herramienta { return []Herramienta{} }

// Detectada dice si la herramienta parece instalada: su programa está en el PATH o existe su carpeta de configuración.
func Detectada(h Herramienta) bool {
	if h.Programa != "" {
		if _, err := exec.LookPath(h.Programa); err == nil {
			return true
		}
	}
	if h.Ruta == nil {
		return false
	}
	ruta, err := h.Ruta(AlcanceGlobal, "")
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Dir(ruta))
	return err == nil
}

func leerArchivo(ruta string) (datos []byte, existe bool, err error) {
	datos, err = os.ReadFile(ruta)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	return datos, err == nil, err
}

// transformar lee el archivo, calcula con f su nuevo contenido y comprueba (comparando el contenido entero, que es más
// firme que una huella y cuesta lo mismo en archivos de configuración) que nadie lo cambió entre medias; si lo cambiaron, lo repite una vez sobre lo nuevo y, si vuelve a cambiar, ErrCambioMientras. No escribe: eso es del llamador,
// que así solo escribe sobre un archivo que sigue siendo el que leyó.
func transformar(ruta string, f func(actual []byte) ([]byte, error)) (actual []byte, existe bool, nuevo []byte, err error) {
	for intento := 0; intento < 2; intento++ {
		if actual, existe, err = leerArchivo(ruta); err != nil {
			return nil, false, nil, err
		}
		if nuevo, err = f(actual); err != nil {
			return nil, false, nil, err
		}
		otro, hay, err := leerArchivo(ruta)
		if err != nil {
			return nil, false, nil, err
		}
		if hay == existe && bytes.Equal(otro, actual) {
			return actual, existe, nuevo, nil
		}
	}
	return nil, false, nil, ErrCambioMientras
}

// Aplicar apunta la herramienta a Tramtai. La primera vez de cada activación guarda una copia del archivo entero; si ya
// estaba activa en esa misma ruta (otro modelo, otro puerto) conserva la copia, la fecha y los valores previos originales.
func Aplicar(datos string, h Herramienta, alcance Alcance, proyecto string, v Valores) (Anotacion, error) {
	ruta, err := h.Ruta(alcance, proyecto)
	if err != nil {
		return Anotacion{}, err
	}
	previa, activa, err := Leer(datos, h.ID)
	if err != nil {
		return Anotacion{}, err
	}
	continua := activa && previa.Ruta == ruta
	var escrito, previos map[string]any
	actual, existe, nuevo, err := transformar(ruta, func(actual []byte) (nuevo []byte, err error) {
		nuevo, escrito, previos, err = h.Formato.Aplicar(actual, v)
		return nuevo, err
	})
	if err != nil {
		return Anotacion{}, err
	}
	a := Anotacion{ID: h.ID, Ruta: ruta, Alcance: alcance, Modelo: v.Modelo, API: h.ID, Puerto: v.Puerto,
		Escrito: escrito, Previos: previos, NoExistia: !existe, Fecha: time.Now().UTC().Truncate(time.Second)}
	if len(v.Orden) > 0 {
		a.Orden = v.Orden[0]
	}
	if continua {
		a.Fecha, a.NoExistia = previa.Fecha, previa.NoExistia
		if a.Previos == nil {
			a.Previos = map[string]any{}
		}
		for k, val := range previa.Previos {
			a.Previos[k] = val // lo que ahora había en esas claves ya era de Tramtai
		}
	} else if existe {
		if err := configuracion.EscribirAtomico(rutaCopia(datos, a), actual, 0o600); err != nil {
			return Anotacion{}, err
		}
	}
	if err := configuracion.EscribirAtomico(ruta, nuevo, 0o600); err != nil {
		return Anotacion{}, err
	}
	return a, guardar(datos, a)
}

// Quitar deshace lo de Tramtai en el archivo anotado y olvida la anotación. Si el archivo lo creó Tramtai y no queda nada
// más, lo borra; si ya no existe (o está vacío), solo olvida.
func Quitar(datos string, h Herramienta, a Anotacion) error {
	var vacio bool
	actual, _, nuevo, err := transformar(a.Ruta, func(actual []byte) (nuevo []byte, err error) {
		if len(actual) == 0 {
			return nil, nil
		}
		nuevo, vacio, err = h.Formato.Quitar(actual, a)
		return nuevo, err
	})
	if err != nil {
		return err
	}
	if len(actual) > 0 {
		if vacio && a.NoExistia {
			if err := os.Remove(a.Ruta); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		} else if err := configuracion.EscribirAtomico(a.Ruta, nuevo, 0o600); err != nil {
			return err
		}
	}
	return Borrar(datos, a.ID)
}

// Restaurar devuelve el archivo a como estaba antes de activar la herramienta, con la copia de esa activación, y olvida
// la anotación. Pisa lo que haya ahora: es la salida cuando Quitar no basta.
func Restaurar(datos string, a Anotacion) (copia string, err error) {
	if a.NoExistia {
		return "", ErrSinCopia
	}
	copia = rutaCopia(datos, a)
	original, err := os.ReadFile(copia)
	if err != nil {
		return "", err
	}
	if err := configuracion.EscribirAtomico(a.Ruta, original, 0o600); err != nil {
		return "", err
	}
	return copia, Borrar(datos, a.ID)
}
