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
	"runtime"
	"strings"
	"sync"
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
	// ErrOtraActivacion: la herramienta ya está apuntada a Tramtai en otro archivo (otro proyecto); hay que quitar esa antes.
	ErrOtraActivacion = errors.New(idioma.T("la herramienta ya está activada en otro archivo: desactívala antes"))
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
//
// escrito y previos van a la anotación, que se guarda en claro: nunca llevan secretos. Para una clave que se escribe en el
// archivo (la del espacio), el valor anotado es la marca «sha256:» + el hash en hexadecimal del valor, y Contiene compara
// con esa marca. Las claves de previos son un subconjunto de las de escrito; una clave que no existía antes no figura.
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
	Gestionado     func() bool                          // nil salvo Claude Code: ¿una política de la organización manda sobre su archivo?
	Previa         func(ruta string, existe bool) error // nil salvo Continue: ¿hay algo que impida activar?; se llama antes de escribir nada
	Formato        Formato
}

// Lista son las herramientas conocidas, en el orden en que se muestran.
func Lista() []Herramienta {
	return []Herramienta{
		{ID: "claude-code", Nombre: "Claude Code", EnProyecto: true, Programa: "claude", Ruta: rutaClaudeCode,
			Gestionado: gestionadoClaudeCode, Formato: ClaudeCode()},
		{ID: "codex", Nombre: "Codex", Programa: "codex", Ruta: rutaCodex, Formato: Codex()},
		{ID: "continue", Nombre: "Continue", ClaveEnArchivo: true, Programa: "cn", Ruta: rutaContinue, Previa: previaContinue, Formato: Continue()},
		{ID: "opencode", Nombre: "opencode", ClaveEnArchivo: true, Programa: "opencode", Ruta: rutaOpenCode, Formato: OpenCode()},
		{ID: "kilo", Nombre: "Kilo Code", ClaveEnArchivo: true, Programa: "kilo", Ruta: rutaKilo, Formato: OpenCode()},
	}
}

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

// sigue comprueba que el archivo es todavía el que se leyó (comparando el contenido entero, que es más firme que una
// huella y cuesta lo mismo en archivos de configuración); si no, ErrCambioMientras.
func sigue(ruta string, actual []byte, existe bool) error {
	otro, hay, err := leerArchivo(ruta)
	if err != nil {
		return err
	}
	if hay != existe || !bytes.Equal(otro, actual) {
		return ErrCambioMientras
	}
	return nil
}

// transformar lee el archivo, calcula con f su nuevo contenido y comprueba que nadie lo cambió entre medias; si lo
// cambiaron, lo repite una vez sobre lo nuevo y, si vuelve a cambiar, ErrCambioMientras. No escribe: eso es del llamador,
// que así solo escribe sobre un archivo que sigue siendo el que leyó.
func transformar(ruta string, f func(actual []byte) ([]byte, error)) (actual []byte, existe bool, nuevo []byte, err error) {
	for intento := 0; intento < 2; intento++ {
		if actual, existe, err = leerArchivo(ruta); err != nil {
			return nil, false, nil, err
		}
		if nuevo, err = f(actual); err != nil {
			return nil, false, nil, err
		}
		if err = sigue(ruta, actual, existe); err == nil {
			return actual, existe, nuevo, nil
		}
		if !errors.Is(err, ErrCambioMientras) {
			return nil, false, nil, err
		}
	}
	return nil, false, nil, ErrCambioMientras
}

// antesDeEscribir es un punto de enganche de las pruebas: corre justo antes de la comprobación final y de escribir el
// archivo de la herramienta.
var antesDeEscribir = func() {}

// cerrojos serializa, dentro del proceso, las operaciones de cada herramienta: dos Aplicar o un Quitar a la vez sobre la
// misma anotación se pisarían (dos copias, previos perdidos).
var cerrojos sync.Map // datos/id → *sync.Mutex

func bloquear(datos, id string) (soltar func()) {
	m, _ := cerrojos.LoadOrStore(filepath.Join(datos, id), &sync.Mutex{})
	m.(*sync.Mutex).Lock()
	return m.(*sync.Mutex).Unlock
}

// reintentar repite op en Windows durante unos 600 ms: allí borrar o renombrar falla mientras otro proceso tiene el
// archivo abierto (un antivirus, la propia herramienta). En los demás sistemas no hace falta.
func reintentar(op func() error) error {
	err := op()
	for espera := 5 * time.Millisecond; err != nil && runtime.GOOS == "windows" && espera <= 320*time.Millisecond; espera *= 2 {
		time.Sleep(espera)
		err = op()
	}
	return err
}

// copiasPorHerramienta es cuántas copias se guardan de cada herramienta: las activaciones recientes, no un archivo
// por cada una que se haya hecho jamás.
const copiasPorHerramienta = 3

// podarCopias borra las copias más antiguas de la herramienta. Es limpieza de mejor esfuerzo: si algo falla, sobra una copia.
func podarCopias(datos, id string) {
	carpeta := filepath.Join(carpetaTramtai(datos), "copias")
	entradas, err := os.ReadDir(carpeta) // viene ordenado por nombre, y el nombre lleva la fecha
	if err != nil {
		return
	}
	var propias []string
	for _, e := range entradas {
		// «<id>-<AAAAMMDD-HHMMSS>…»: la fecha distingue «claude» de «claude-code»
		resto, ok := strings.CutPrefix(e.Name(), id+"-")
		if !ok || len(resto) < 15 {
			continue
		}
		if _, err := time.Parse("20060102-150405", resto[:15]); err == nil {
			propias = append(propias, e.Name())
		}
	}
	for _, nombre := range propias[:max(0, len(propias)-copiasPorHerramienta)] {
		_ = os.Remove(filepath.Join(carpeta, nombre))
	}
}

// Aplicar apunta la herramienta a Tramtai. La primera vez de cada activación guarda una copia del archivo entero; si ya
// estaba activa en esa misma ruta (otro modelo, otro puerto) conserva la copia, la fecha y los valores previos originales.
// Si estaba activa en otra ruta devuelve ErrOtraActivacion: hay que quitar aquella antes. El orden es copia, anotación y
// archivo: un corte a medias nunca deja el archivo apuntando a Tramtai sin anotación con que deshacerlo.
func Aplicar(datos string, h Herramienta, alcance Alcance, proyecto string, v Valores) (Anotacion, error) {
	ruta, err := h.Ruta(alcance, proyecto)
	if err != nil {
		return Anotacion{}, err
	}
	defer bloquear(datos, h.ID)()
	previa, activa, err := Leer(datos, h.ID)
	if err != nil {
		return Anotacion{}, err
	}
	if activa && previa.Ruta != ruta {
		return Anotacion{}, ErrOtraActivacion
	}
	var escrito, previos map[string]any
	actual, existe, nuevo, err := transformar(ruta, func(actual []byte) (nuevo []byte, err error) {
		nuevo, escrito, previos, err = h.Formato.Aplicar(actual, v)
		return nuevo, err
	})
	if err != nil {
		return Anotacion{}, err
	}
	if h.Previa != nil {
		if err := h.Previa(ruta, existe); err != nil {
			return Anotacion{}, err
		}
	}
	a := Anotacion{ID: h.ID, Ruta: ruta, Alcance: alcance, Modelo: v.Modelo, API: h.ID, Puerto: v.Puerto,
		Escrito: escrito, Previos: previos, NoExistia: !existe, Fecha: time.Now().UTC().Truncate(time.Second)}
	if len(v.Orden) > 0 {
		a.Orden = v.Orden[0]
	}
	copia := ""
	if activa {
		a.Fecha, a.NoExistia = previa.Fecha, previa.NoExistia
		if a.Previos == nil {
			a.Previos = map[string]any{}
		}
		for k := range previa.Escrito {
			// lo que ahora hay en esas claves ya era de Tramtai: vale lo que había antes de la primera activación, y si no
			// había nada, nada (que Quitar borre la clave en lugar de «restaurar» el valor de Tramtai)
			if val, ok := previa.Previos[k]; ok {
				a.Previos[k] = val
			} else {
				delete(a.Previos, k)
			}
		}
	} else if existe {
		copia = rutaCopia(datos, a)
		if err := configuracion.EscribirAtomico(copia, actual, 0o600); err != nil {
			return Anotacion{}, err
		}
	}
	deshacer := func() { // limpieza de mejor esfuerzo: el error que importa es el que se devuelve
		if copia != "" {
			_ = os.Remove(copia)
		}
		if activa {
			_ = guardar(datos, previa)
		} else {
			_ = Borrar(datos, h.ID)
		}
	}
	if err := guardar(datos, a); err != nil {
		deshacer()
		return Anotacion{}, err
	}
	antesDeEscribir()
	err = sigue(ruta, actual, existe) // lo último antes de pisar: la copia y la anotación han tardado
	if err == nil {
		err = configuracion.EscribirAtomico(ruta, nuevo, 0o600)
	}
	if err != nil {
		deshacer()
		return Anotacion{}, err
	}
	if copia != "" {
		podarCopias(datos, h.ID)
	}
	return a, nil
}

// Quitar deshace lo de Tramtai en el archivo anotado y olvida la anotación. Si el archivo lo creó Tramtai y no queda nada
// más, lo borra; si ya no existe (o está vacío), solo olvida.
func Quitar(datos string, h Herramienta, a Anotacion) error {
	defer bloquear(datos, a.ID)()
	var vacio bool
	actual, existe, nuevo, err := transformar(a.Ruta, func(actual []byte) (nuevo []byte, err error) {
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
		antesDeEscribir()
		if err := sigue(a.Ruta, actual, existe); err != nil {
			return err
		}
		if vacio && a.NoExistia {
			err = reintentar(func() error {
				if e := os.Remove(a.Ruta); !errors.Is(e, fs.ErrNotExist) {
					return e
				}
				return nil
			})
		} else {
			err = configuracion.EscribirAtomico(a.Ruta, nuevo, 0o600)
		}
		if err != nil {
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
	defer bloquear(datos, a.ID)()
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
