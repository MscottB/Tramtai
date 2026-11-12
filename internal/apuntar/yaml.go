package apuntar

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"tramtai/internal/idioma"
)

const (
	nombreModeloYAML = "Tramtai"
	claveEscritoYAML = "models.Tramtai" // en escrito, la entrada entera con su apiKey como marca
	claveModels      = "models"         // en escrito, la lista que contiene el modelo; en previos, la línea de «models» que había si estaba vacía
)

var (
	// errModeloAjeno: ya hay un modelo «Tramtai» y no lo escribió Tramtai (otra forma, otros campos, o varios); pisarlo perdería lo del usuario.
	errModeloAjeno = errors.New(idioma.T("ya hay un modelo «Tramtai» en config.yaml que no es de Tramtai: renómbralo o quítalo"))
	// errModelosEnLinea: Tramtai edita la lista por líneas, y una lista entre corchetes con elementos no tiene una por elemento.
	errModelosEnLinea = errors.New(idioma.T("«models» de config.yaml está escrito entre corchetes: pásalo a una lista con guiones"))
	// ErrContinueJSON: Continue lee config.yaml si existe y, si no, config.json; crear el YAML dejaría sin efecto el JSON del usuario.
	ErrContinueJSON = errors.New(idioma.T("Continue usa config.json en esta carpeta: pásalo a config.yaml antes de activarlo"))
)

// cabeceraNueva es lo que lleva un config.yaml que crea Tramtai además del modelo (Continue lo exige).
var cabeceraNueva = []string{"name", "Local", "version", "1.0.0", "schema", "v1"}

// apiBaseTramtai es la forma de la URL que escribe Tramtai (el puerto y la API cambian entre activaciones).
var apiBaseTramtai = regexp.MustCompile(`^http://(127\.0\.0\.1|localhost|\[::1\]):\d+/p/[a-z0-9_-]+/openai/v1$`)

// formatoYAML edita config.yaml por líneas, sin reserializar: yaml.v3 re-escribiría todo el documento (sangrado, comillas,
// líneas en blanco) y lo que no es de Tramtai tiene que quedar byte a byte como estaba. yaml.Node solo sirve para ubicar
// «models», sus elementos y el de Tramtai, y para validar; antes de devolver nada se comprueba, decodificando, que el resto
// del archivo no cambió de sentido.
//
// Quitar quita el modelo solo si sigue valiendo lo que escribió Tramtai (la apiKey, por su marca); si el usuario lo tocó,
// es suyo y se queda.
type formatoYAML struct{}

// Continue apunta Continue a Tramtai: un modelo «Tramtai» en la lista models de ~/.continue/config.yaml, con la clave dentro.
func Continue() Formato { return formatoYAML{} }

// archivoYAML es el archivo en líneas, cada una con su terminador.
type archivoYAML struct {
	bom bool
	eol string // «\r\n» solo si todas las líneas lo usan
	ls  []string
}

func abrirYAML(actual []byte) *archivoYAML {
	cuerpo := bytes.TrimPrefix(actual, bom)
	a := &archivoYAML{bom: len(cuerpo) != len(actual), eol: "\n"}
	if n := bytes.Count(cuerpo, salto); n > 0 && n == bytes.Count(cuerpo, []byte("\r\n")) {
		a.eol = "\r\n"
	}
	a.ls = strings.SplitAfter(string(cuerpo), "\n")
	if a.ls[len(a.ls)-1] == "" {
		a.ls = a.ls[:len(a.ls)-1]
	}
	return a
}

func (a *archivoYAML) texto() string { return strings.Join(a.ls, "") }

func (a *archivoYAML) bytes() []byte {
	if a.bom {
		return append(bytes.Clone(bom), a.texto()...)
	}
	return []byte(a.texto())
}

// raiz valida el archivo (un solo documento, sin claves repetidas) y devuelve su mapa raíz; nil si no tiene contenido.
func (a *archivoYAML) raiz() (*yaml.Node, error) {
	dec := yaml.NewDecoder(strings.NewReader(a.texto()))
	var doc, otro yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrArchivoIlegible, err)
	}
	if err := dec.Decode(&otro); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: más de un documento", ErrArchivoIlegible)
	}
	var valor any
	if err := doc.Decode(&valor); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrArchivoIlegible, err)
	}
	r := doc.Content[0]
	if r.Kind == yaml.ScalarNode && r.Tag == "!!null" && r.Value == "" {
		return nil, nil
	}
	if r.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: la raíz no es un mapa", ErrArchivoIlegible)
	}
	return r, nil
}

// par devuelve la posición de la clave en el mapa, o -1.
func par(m *yaml.Node, clave string) int {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Kind == yaml.ScalarNode && m.Content[i].Value == clave {
			return i
		}
	}
	return -1
}

func sangria(l string) int { return len(l) - len(strings.TrimLeft(l, " ")) }

// ajena dice si la línea ni es contenido ni lo acompaña: en blanco, o un comentario que no sangra más que sangr.
func ajena(l string, sangr int) bool {
	t := strings.TrimSpace(l)
	return t == "" || strings.HasPrefix(t, "#") && sangria(l) <= sangr
}

// fin es la línea siguiente al contenido que empieza en desde y llega como mucho hasta limite: los comentarios y espacios
// del final no son suyos (son de lo que sigue), salvo los que sangran más que sangr, que pueden ser de un texto de varias líneas.
func (a *archivoYAML) fin(desde, limite, sangr int) int {
	for limite > desde+1 && ajena(a.ls[limite-1], sangr) {
		limite--
	}
	return limite
}

// guion es la línea del «-» de un elemento de la lista.
func (a *archivoYAML) guion(item *yaml.Node) int {
	i := min(item.Line-1, len(a.ls)-1)
	for i > 0 && !strings.HasPrefix(strings.TrimSpace(a.ls[i]), "-") {
		i--
	}
	return i
}

// terminaEnBloque dice si lo último del elemento es un texto de bloque («|» o «>»).
func terminaEnBloque(it *yaml.Node) bool {
	for len(it.Content) > 0 {
		it = it.Content[len(it.Content)-1]
	}
	return it.Kind == yaml.ScalarNode && it.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0
}

// tramos son los de cada elemento de la lista models, que está en el par k de la raíz: la línea de su guion y la siguiente a su contenido.
func (a *archivoYAML) tramos(raiz *yaml.Node, k int) (inicio, fin []int) {
	limite := len(a.ls)
	if k+2 < len(raiz.Content) {
		limite = raiz.Content[k+2].Line - 1
	}
	items := raiz.Content[k+1].Content
	for _, it := range items {
		inicio = append(inicio, a.guion(it))
	}
	for i, it := range items {
		sig := limite
		if i+1 < len(items) {
			sig = inicio[i+1]
		}
		// solo un texto de varias líneas (| o >) puede llevar dentro líneas que parecen comentarios; si no, todo comentario o blanco del final es de lo que sigue
		sangr := math.MaxInt
		if terminaEnBloque(it) {
			sangr = sangria(a.ls[inicio[i]])
		}
		fin = append(fin, a.fin(inicio[i], sig, sangr))
	}
	return inicio, fin
}

// reemplazar sustituye ls[desde:hasta] por las líneas nuevas (sin terminador). Si lo sustituido era el final del archivo y
// acababa sin salto de línea, lo nuevo tampoco lo lleva; al insertar tras una última línea sin salto se le da uno y el bloque
// queda sin él. Así Quitar, que hace lo contrario, devuelve el archivo como estaba.
func (a *archivoYAML) reemplazar(desde, hasta int, nuevas []string) {
	sinSalto := false
	if hasta == len(a.ls) {
		if hasta > desde {
			sinSalto = !strings.HasSuffix(a.ls[hasta-1], "\n")
		} else if desde > 0 && !strings.HasSuffix(a.ls[desde-1], "\n") {
			a.ls[desde-1] += a.eol
			sinSalto = true
		}
	}
	lineas := make([]string, len(nuevas))
	for i, l := range nuevas {
		lineas[i] = l + a.eol
	}
	if sinSalto && len(lineas) > 0 {
		lineas[len(lineas)-1] = nuevas[len(nuevas)-1]
	}
	a.ls = slices.Concat(a.ls[:desde], lineas, a.ls[hasta:])
}

// quitarLineas deja el archivo sin las líneas marcadas; si se fue la última, que no acababa en salto, la nueva última tampoco.
func (a *archivoYAML) quitarLineas(quitar map[int]bool) {
	ultimaSinSalto := len(a.ls) > 0 && quitar[len(a.ls)-1] && !strings.HasSuffix(a.ls[len(a.ls)-1], "\n")
	var resto []string
	for i, l := range a.ls {
		if !quitar[i] {
			resto = append(resto, l)
		}
	}
	if ultimaSinSalto && len(resto) > 0 {
		resto[len(resto)-1] = finDeLinea(resto[len(resto)-1])
	}
	a.ls = resto
}

var plano = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_./:@+-]*$`)

// escalarYAML escribe s como escalar: tal cual si YAML lo lee igual (y como texto), si no entre comillas dobles.
func escalarYAML(s string) string {
	var v any
	if plano.MatchString(s) && yaml.Unmarshal([]byte(s), &v) == nil && v == s {
		return s
	}
	b, _ := aJSON(s)
	return string(b)
}

// entradaYAML es el modelo de Tramtai tal como queda en el archivo.
func entradaYAML(v Valores) map[string]any {
	return map[string]any{"name": nombreModeloYAML, "provider": "openai", "model": v.Modelo, "apiBase": v.URLOpenAI, "apiKey": v.Clave,
		"roles": []any{"chat", "edit", "apply"}, "capabilities": []any{"tool_use"}}
}

func bloqueYAML(v Valores, ind string) []string {
	return []string{ind + "- name: " + nombreModeloYAML, ind + "  provider: openai", ind + "  model: " + escalarYAML(v.Modelo),
		ind + "  apiBase: " + escalarYAML(v.URLOpenAI), ind + "  apiKey: " + escalarYAML(v.Clave),
		ind + "  roles:", ind + "    - chat", ind + "    - edit", ind + "    - apply", ind + "  capabilities:", ind + "    - tool_use"}
}

// esTramtai dice si el elemento de la lista se llama «Tramtai».
func esTramtai(it *yaml.Node) bool {
	if it.Kind != yaml.MappingNode {
		return false
	}
	i := par(it, "name")
	return i >= 0 && it.Content[i+1].Kind == yaml.ScalarNode && it.Content[i+1].Value == nombreModeloYAML
}

// esDeTramtai dice si el modelo tiene la forma exacta que escribe Tramtai: solo cambian el modelo, la URL (puerto) y la clave.
func esDeTramtai(it *yaml.Node) bool {
	var m map[string]any
	if it.Decode(&m) != nil {
		return false
	}
	_, ok1 := m["model"].(string)
	base, ok2 := m["apiBase"].(string)
	_, ok3 := m["apiKey"].(string)
	if !ok1 || !ok2 || !ok3 || !apiBaseTramtai.MatchString(base) {
		return false
	}
	ref := entradaYAML(Valores{})
	for _, k := range []string{"model", "apiBase", "apiKey"} {
		ref[k] = m[k]
	}
	return igual(m, ref)
}

// sigueEscrito dice si el elemento sigue valiendo lo anotado (la apiKey, comparada por su marca).
func sigueEscrito(it *yaml.Node, a Anotacion) bool {
	esperado, hay := a.Escrito[claveEscritoYAML]
	var m map[string]any
	if !hay || it.Decode(&m) != nil {
		return false
	}
	if k, ok := m["apiKey"].(string); ok {
		m["apiKey"] = marca(k)
	}
	return igual(m, esperado)
}

// sinTramtai es el contenido del archivo sin lo que Tramtai añade (su modelo, un models que se queda vacío, la cabecera de
// un archivo nuevo): sirve para comprobar que no cambió nada más.
func sinTramtai(texto string) (map[string]any, error) {
	var m map[string]any
	if err := yaml.Unmarshal([]byte(texto), &m); err != nil {
		return nil, err
	}
	switch l := m["models"].(type) {
	case nil:
		delete(m, "models")
	case []any:
		resto := slices.DeleteFunc(slices.Clone(l), func(e any) bool {
			x, ok := e.(map[string]any)
			return ok && x["name"] == nombreModeloYAML
		})
		if len(resto) == 0 {
			delete(m, "models")
		} else {
			m["models"] = resto
		}
	}
	cabecera := map[string]any{}
	for i := 0; i < len(cabeceraNueva); i += 2 {
		cabecera[cabeceraNueva[i]] = cabeceraNueva[i+1]
	}
	if reflect.DeepEqual(m, cabecera) || len(m) == 0 {
		return map[string]any{}, nil
	}
	return m, nil
}

// mismoResto comprueba que antes y despues solo difieren en lo de Tramtai.
func mismoResto(antes, despues string) error {
	x, err1 := sinTramtai(antes)
	y, err2 := sinTramtai(despues)
	if err1 != nil || err2 != nil || !reflect.DeepEqual(x, y) {
		return fmt.Errorf("%w: el resultado no conserva el resto del archivo", ErrArchivoIlegible)
	}
	return nil
}

func (formatoYAML) Aplicar(actual []byte, v Valores) ([]byte, map[string]any, map[string]any, error) {
	if v.Clave == "" {
		return nil, nil, nil, errSinClave
	}
	a := abrirYAML(actual)
	original := a.texto()
	raiz, err := a.raiz()
	if err != nil {
		return nil, nil, nil, err
	}
	fin := len(a.ls)
	previos := map[string]any{}
	k := -1
	if raiz != nil {
		k = par(raiz, "models")
	}
	switch {
	case raiz == nil:
		var linea []string
		for i := 0; i < len(cabeceraNueva); i += 2 {
			linea = append(linea, cabeceraNueva[i]+": "+cabeceraNueva[i+1])
		}
		a.reemplazar(fin, fin, append(append(linea, "models:"), bloqueYAML(v, "  ")...))
	case k < 0:
		a.reemplazar(fin, fin, append([]string{"models:"}, bloqueYAML(v, "  ")...))
	default:
		clave, lista := raiz.Content[k], raiz.Content[k+1]
		switch {
		case lista.Kind == yaml.ScalarNode && lista.Tag == "!!null" && lista.Value == "":
			previos[claveModels] = finDeLinea(a.ls[clave.Line-1])
			a.reemplazar(clave.Line, clave.Line, bloqueYAML(v, "  "))
		case lista.Kind == yaml.SequenceNode && lista.Style&yaml.FlowStyle != 0 && len(lista.Content) == 0:
			// «models: []»: se quitan los corchetes y la lista se escribe debajo
			if lista.Line != clave.Line {
				return nil, nil, nil, errModelosEnLinea
			}
			l := a.ls[lista.Line-1]
			i := len(string([]rune(l)[:lista.Column-1]))
			j := strings.IndexByte(l[i:], ']')
			if j < 0 {
				return nil, nil, nil, errModelosEnLinea
			}
			previos[claveModels] = finDeLinea(l)
			a.ls[lista.Line-1] = strings.TrimRight(l[:i], " \t") + l[i+j+1:]
			a.reemplazar(lista.Line, lista.Line, bloqueYAML(v, "  "))
		case lista.Kind == yaml.SequenceNode && lista.Style&yaml.FlowStyle != 0:
			return nil, nil, nil, errModelosEnLinea
		case lista.Kind == yaml.SequenceNode:
			inicio, finItems := a.tramos(raiz, k)
			var propios []int
			for i, it := range lista.Content {
				if esTramtai(it) {
					propios = append(propios, i)
				}
			}
			switch {
			case len(propios) == 0:
				a.reemplazar(finItems[len(finItems)-1], finItems[len(finItems)-1], bloqueYAML(v, a.ls[inicio[0]][:sangria(a.ls[inicio[0]])]))
			case len(propios) == 1 && esDeTramtai(lista.Content[propios[0]]):
				i := propios[0]
				a.reemplazar(inicio[i], finItems[i], bloqueYAML(v, a.ls[inicio[i]][:sangria(a.ls[inicio[i]])]))
			default:
				return nil, nil, nil, errModeloAjeno
			}
		default:
			return nil, nil, nil, fmt.Errorf("%w: «models» no es una lista", ErrArchivoIlegible)
		}
	}
	nuevo := a.texto()
	if err := mismoResto(original, nuevo); err != nil {
		return nil, nil, nil, err
	}
	anotada := entradaYAML(v)
	anotada["apiKey"] = marca(v.Clave)
	escrito := map[string]any{claveEscritoYAML: anotada, claveModels: nombreModeloYAML}
	if !(formatoYAML{}).Contiene(a.bytes(), Anotacion{Escrito: escrito}) {
		return nil, nil, nil, fmt.Errorf("%w: el modelo no quedó como se escribió", ErrArchivoIlegible)
	}
	return a.bytes(), escrito, previos, nil
}

func (formatoYAML) Quitar(actual []byte, anot Anotacion) ([]byte, bool, error) {
	a := abrirYAML(actual)
	original := a.texto()
	raiz, err := a.raiz()
	if err != nil {
		return nil, false, err
	}
	if raiz != nil {
		if k := par(raiz, "models"); k >= 0 && raiz.Content[k+1].Kind == yaml.SequenceNode && raiz.Content[k+1].Style&yaml.FlowStyle == 0 {
			a.quitarModelo(raiz, k, anot)
		}
	}
	nuevo := a.texto()
	if err := mismoResto(original, nuevo); err != nil {
		return nil, false, err
	}
	return a.bytes(), strings.TrimSpace(nuevo) == "", nil
}

// quitarModelo borra los elementos de Tramtai que siguen como se escribieron; si la lista se queda sin ninguno, su clave
// también, y si no queda más que la cabecera que pone Tramtai a un archivo nuevo, esa también. Límite conocido: un models
// que ya estaba vacío antes de activar vuelve como estaba (Aplicar anota su línea en previos).
func (a *archivoYAML) quitarModelo(raiz *yaml.Node, k int, anot Anotacion) {
	inicio, fin := a.tramos(raiz, k)
	quitar := map[int]bool{}
	items := raiz.Content[k+1].Content
	quitados := 0
	for i, it := range items {
		if esTramtai(it) && sigueEscrito(it, anot) {
			for l := inicio[i]; l < fin[i]; l++ {
				quitar[l] = true
			}
			quitados++
		}
	}
	if quitados == 0 {
		return
	}
	if quitados == len(items) {
		linea := raiz.Content[k].Line - 1
		if previo, ok := anot.Previos[claveModels].(string); ok {
			// el models estaba vacío antes de activar: vuelve su línea tal cual, con su comentario
			a.ls[linea] = previo + strings.TrimPrefix(a.ls[linea], finDeLinea(a.ls[linea]))
		} else {
			quitar[linea] = true
			if len(raiz.Content) == 2*(len(cabeceraNueva)/2)+2 {
				a.marcarCabecera(raiz, k, quitar)
			}
		}
	}
	a.quitarLineas(quitar)
}

// marcarCabecera marca las líneas de los otros pares de la raíz si son justo la cabecera que pone Tramtai.
func (a *archivoYAML) marcarCabecera(raiz *yaml.Node, k int, quitar map[int]bool) {
	var tramos [][2]int
	for i := 0; i < len(raiz.Content); i += 2 {
		if i == k {
			continue
		}
		clave, valor := raiz.Content[i], raiz.Content[i+1]
		j := slices.Index(cabeceraNueva, clave.Value)
		if j < 0 || j%2 != 0 || valor.Kind != yaml.ScalarNode || valor.Value != cabeceraNueva[j+1] {
			return
		}
		limite := len(a.ls)
		if i+2 < len(raiz.Content) {
			limite = raiz.Content[i+2].Line - 1
		}
		tramos = append(tramos, [2]int{clave.Line - 1, a.fin(clave.Line-1, limite, 0)})
	}
	for _, t := range tramos {
		for l := t[0]; l < t[1]; l++ {
			quitar[l] = true
		}
	}
}

func (formatoYAML) Contiene(actual []byte, anot Anotacion) bool {
	raiz, err := abrirYAML(actual).raiz()
	if err != nil || raiz == nil {
		return false
	}
	k := par(raiz, "models")
	if k < 0 || raiz.Content[k+1].Kind != yaml.SequenceNode {
		return false
	}
	return slices.ContainsFunc(raiz.Content[k+1].Content, func(it *yaml.Node) bool { return esTramtai(it) && sigueEscrito(it, anot) })
}

// previaContinue se niega a crear config.yaml junto a un config.json: Continue dejaría de leer el JSON.
func previaContinue(ruta string, existe bool) error {
	if existe {
		return nil
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(ruta), "config.json")); err == nil {
		return ErrContinueJSON
	}
	return nil
}

// rutaContinue es $CONTINUE_GLOBAL_DIR/config.yaml o, sin esa variable, ~/.continue/config.yaml.
func rutaContinue(_ Alcance, _ string) (string, error) {
	if dir := os.Getenv("CONTINUE_GLOBAL_DIR"); dir != "" {
		return filepath.Join(dir, "config.yaml"), nil
	}
	casa, err := os.UserHomeDir()
	return filepath.Join(casa, ".continue", "config.yaml"), err
}
