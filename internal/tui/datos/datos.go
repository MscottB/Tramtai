// Package datos trae a la terminal lo que la Web UI ya dice: la Ayuda en texto, las plantillas de salida estructurada y los
// nombres del menú. Los JSON los genera web/scripts/terminal-datos.mjs desde las fuentes de la Web UI (cd web; npm run
// sitio); el CI comprueba que lo guardado coincide con ellas.
package datos

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

// Bloque es una parte de un tema. Tipo es texto, pasos, tabla, preguntas, errores, pantalla o web. Todo es texto plano:
// Lineas son los párrafos o pasos (en tabla, sus columnas), Filas las de una tabla (título y celdas) y Pares (título o
// pregunta, texto o respuesta) los de errores y preguntas; pantalla es el recorrido de la Web UI, sin sus textos. URL, en
// web, lleva al tema en el sitio. Los textos son los de la terminal: la variante <clave>.terminal de la fuente si la hay.
type Bloque struct {
	Tipo   string      `json:"tipo"`
	Titulo string      `json:"titulo,omitempty"`
	Lineas []string    `json:"lineas,omitempty"`
	Filas  [][]string  `json:"filas,omitempty"`
	Pares  [][2]string `json:"pares,omitempty"`
	URL    string      `json:"url,omitempty"`
}

// Tema es un tema de la Ayuda; Pagina es la página de la app a la que acompaña (inicio, ejecutar…), si tiene una. URL lleva
// al tema en el sitio y Terminal es la línea «En la terminal: …», con sus teclas.
type Tema struct {
	ID       string   `json:"id"`
	Titulo   string   `json:"titulo"`
	Pagina   string   `json:"pagina,omitempty"`
	Resumen  string   `json:"resumen"`
	URL      string   `json:"url"`
	Terminal string   `json:"terminal"`
	Bloques  []Bloque `json:"bloques"`
}

// Plantilla es una de «Respuesta con forma»; Esquema es el JSON Schema, tal como lo arma la Web UI.
type Plantilla struct {
	ID            string          `json:"id"`
	Nombre        string          `json:"nombre"`
	Instrucciones string          `json:"instrucciones"`
	Esquema       json.RawMessage `json:"esquema"`
}

var (
	//go:embed ayuda.json
	ayudaJSON []byte
	//go:embed plantillas.json
	plantillasJSON []byte
	//go:embed textos.json
	textosJSON []byte
)

// cargar lee un JSON incrustado: si no se puede leer, el binario está mal hecho y la prueba lo avisa.
func cargar[T any](datos []byte) func() map[string]T {
	return sync.OnceValue(func() map[string]T {
		var m map[string]T
		if err := json.Unmarshal(datos, &m); err != nil {
			panic("tui/datos: " + err.Error())
		}
		return m
	})
}

var (
	ayuda      = cargar[[]Tema](ayudaJSON)
	plantillas = cargar[[]Plantilla](plantillasJSON)
	textos     = cargar[map[string]string](textosJSON)
)

// lengua es el idioma de los datos: en para el inglés y es para cualquier otro.
func lengua(idioma string) string {
	if idioma == "en" {
		return "en"
	}
	return "es"
}

// Ayuda son los temas de la Ayuda, en el orden de la Web UI.
func Ayuda(idioma string) []Tema { return ayuda()[lengua(idioma)] }

// Plantillas son las plantillas de salida estructurada.
func Plantillas(idioma string) []Plantilla { return plantillas()[lengua(idioma)] }

// Textof es Texto con sus {marcadores} sustituidos: pares va de dos en dos, marcador (sin llaves) y valor.
func Textof(idioma, clave string, pares ...string) string {
	llaves := make([]string, len(pares))
	for i, p := range pares {
		if i%2 == 0 {
			p = "{" + p + "}"
		}
		llaves[i] = p
	}
	return strings.NewReplacer(llaves...).Replace(Texto(idioma, clave))
}

// Texto es el nombre que la Web UI da a una clave (menu.* y los textos de la Ayuda de la interfaz); si no existe, la clave.
func Texto(idioma, clave string) string {
	if t, ok := textos()[lengua(idioma)][clave]; ok {
		return t
	}
	return clave
}

// Tiene dice si hay un texto con esa clave (Texto, si no, devuelve la clave).
func Tiene(idioma, clave string) bool {
	_, ok := textos()[lengua(idioma)][clave]
	return ok
}
