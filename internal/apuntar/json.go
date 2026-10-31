package apuntar

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/tailscale/hujson"

	"tramtai/internal/idioma"
)

var (
	errSinOrden    = errors.New(idioma.T("falta la orden que entrega la clave"))
	errSinClave    = errors.New(idioma.T("falta la clave del espacio"))
	errSinProyecto = errors.New(idioma.T("falta la carpeta del proyecto"))
	// errOrdenConPorcentaje: el apiKeyHelper lo ejecuta cmd.exe en Windows, que expande «%» incluso entre comillas.
	errOrdenConPorcentaje = errors.New(idioma.T("la ruta de Tramtai lleva un «%»: cmd.exe la expandiría; instálalo en otra carpeta"))
)

var bom = []byte("\xef\xbb\xbf")

// claveJSON es un valor que Tramtai escribe en una ruta del archivo («env», «ANTHROPIC_MODEL»). Un secreto se anota con
// su marca «sha256:» y nunca se guarda como valor previo, porque la anotación va en claro.
type claveJSON struct {
	ruta    []string
	valor   any
	secreto bool
}

func (c claveJSON) nombre() string { return strings.Join(c.ruta, ".") }

// formatoJSON edita archivos JSON y JSONC (comentarios y comas finales) con hujson: solo toca las rutas de claves, así que
// comentarios, orden y espacios del resto quedan como estaban.
type formatoJSON struct {
	claves func(v Valores) ([]claveJSON, error)
}

func marca(valor string) string {
	h := sha256.Sum256([]byte(valor))
	return "sha256:" + hex.EncodeToString(h[:])
}

func aJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	err := e.Encode(v)
	return bytes.TrimSpace(b.Bytes()), err
}

func puntero(ruta []string) string {
	var s strings.Builder
	for _, p := range ruta {
		s.WriteString("/" + strings.NewReplacer("~", "~0", "/", "~1").Replace(p))
	}
	return s.String()
}

func igual(a, b any) bool {
	x, e1 := aJSON(a)
	y, e2 := aJSON(b)
	return e1 == nil && e2 == nil && bytes.Equal(x, y)
}

// coincide compara lo que hay en el archivo con lo anotado (una clave secreta se anotó como su marca).
func coincide(archivo, anotado any) bool {
	if s, ok := archivo.(string); ok {
		if m, ok := anotado.(string); ok && strings.HasPrefix(m, "sha256:") {
			return m == marca(s)
		}
	}
	return igual(archivo, anotado)
}

// documento es el archivo abierto: el valor de hujson y si traía marca de orden de bytes.
type documento struct {
	v      hujson.Value
	bom    bool
	vacio  bool   // el archivo estaba en blanco: se parte de {}
	unidad string // un nivel de sangrado del archivo; vacío si está en una línea
	eol    string // «\r\n» si el archivo los usa, si no «\n»
}

var salto = []byte("\n")

func tieneComentario(b []byte) bool {
	return bytes.Contains(b, []byte("//")) || bytes.Contains(b, []byte("/*"))
}

// unidadDe deduce el sangrado de un nivel del objeto raíz, por lo que precede a su primera clave: «  » si no tiene
// claves aún, y vacío si la primera clave sigue a la llave en la misma línea.
func unidadDe(raiz *hujson.Object) string {
	if len(raiz.Members) == 0 {
		return "  "
	}
	extra := raiz.Members[0].Name.BeforeExtra
	i := bytes.LastIndexByte(extra, '\n')
	if i < 0 {
		return ""
	}
	linea := extra[i+1:]
	return string(linea[:len(linea)-len(bytes.TrimLeft(linea, " \t"))])
}

func abrir(actual []byte) (*documento, error) {
	d := &documento{bom: bytes.HasPrefix(actual, bom), eol: "\n"}
	cuerpo := bytes.Clone(bytes.TrimPrefix(actual, bom)) // hujson edita sobre el búfer que recibe: no el del llamador
	if n := bytes.Count(cuerpo, salto); n > 0 && n == bytes.Count(cuerpo, []byte("\r\n")) {
		d.eol = "\r\n" // solo si todos lo son: un archivo ya mezclado se deja como está
	}
	if len(bytes.TrimSpace(cuerpo)) == 0 {
		cuerpo, d.vacio = []byte("{}"), true
	}
	v, err := hujson.Parse(cuerpo)
	if err == nil {
		if raiz, ok := v.Value.(*hujson.Object); ok {
			d.unidad = unidadDe(raiz)
		} else {
			err = errors.New("la raíz no es un objeto")
		}
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrArchivoIlegible, err)
	}
	d.v = v
	return d, nil
}

// leer devuelve el valor de la ruta; hay es false si no existe (o si un tramo no es un objeto).
func (d *documento) leer(ruta []string) (valor any, hay bool) {
	n := d.v.Find(puntero(ruta))
	if n == nil {
		return nil, false
	}
	b, err := hujson.Standardize(n.Pack())
	if err != nil {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if dec.Decode(&valor) != nil {
		return nil, false
	}
	return valor, true
}

func (d *documento) parche(op, ptr string, valor any) error {
	p := map[string]any{"op": op, "path": ptr}
	if op != "remove" {
		p["value"] = valor
	}
	b, err := aJSON([]any{p})
	if err != nil {
		return err
	}
	return d.v.Patch(b)
}

// quitar borra la clave. Si arrastra un comentario de línea de la clave anterior hasta la llave de cierre, el sangrado
// que traía la clave borrada se reemplaza por el de la llave.
func (d *documento) quitar(ruta []string) error {
	if err := d.parche("remove", puntero(ruta), nil); err != nil {
		return err
	}
	prof := len(ruta) - 1
	if n := d.v.Find(puntero(ruta[:prof])); n != nil && d.unidad != "" {
		if obj, ok := n.Value.(*hujson.Object); ok && tieneComentario(obj.AfterExtra) && bytes.Contains(obj.AfterExtra, salto) {
			obj.AfterExtra = conSangrado(obj.AfterExtra, d.eol, strings.Repeat(d.unidad, prof))
		}
	}
	return nil
}

// conSangrado deja el espacio en blanco que sigue al último salto de línea de extra (o uno nuevo, si no hay) como indent.
func conSangrado(extra []byte, eol, indent string) []byte {
	i := bytes.LastIndexByte(extra, '\n')
	if i < 0 {
		return append(bytes.Clone(extra), eol+indent...)
	}
	return append(bytes.Clone(extra[:i+1]), indent...)
}

// anadir pone el valor en la ruta. Si la clave es nueva y el archivo está en varias líneas, la deja con su línea y su
// sangrado (el parche la inserta en compacto); si ya existía, solo cambia el valor.
func (d *documento) anadir(ruta []string, valor any) error {
	nueva := d.v.Find(puntero(ruta)) == nil
	if err := d.parche("add", puntero(ruta), valor); err != nil || !nueva || d.unidad == "" {
		return err
	}
	prof := len(ruta) - 1
	obj, ok := d.v.Find(puntero(ruta[:prof])).Value.(*hujson.Object)
	if !ok || (len(obj.Members) > 1 && !bytes.Contains(obj.Members[0].Name.BeforeExtra, salto)) {
		return nil // el objeto va en una línea
	}
	m := &obj.Members[len(obj.Members)-1]
	m.Value.BeforeExtra = []byte(" ")
	// un comentario de línea de la clave anterior (`"a": 1 // mío`) pasa a ir antes de la nueva: se conserva y solo se
	// rehace el sangrado que va tras su último salto de línea
	m.Name.BeforeExtra = conSangrado(m.Name.BeforeExtra, d.eol, strings.Repeat(d.unidad, prof+1))
	if !bytes.Contains(obj.AfterExtra, salto) {
		obj.AfterExtra = conSangrado(obj.AfterExtra, d.eol, strings.Repeat(d.unidad, prof))
	}
	b, err := aJSON(valor)
	if err != nil {
		return err
	}
	var sangrado bytes.Buffer
	if json.Indent(&sangrado, b, strings.Repeat(d.unidad, prof+1), d.unidad) == nil {
		if v, err := hujson.Parse(bytes.ReplaceAll(sangrado.Bytes(), salto, []byte(d.eol))); err == nil {
			m.Value.Value = v.Value
		}
	}
	return nil
}

// poner escribe el valor creando los objetos intermedios que falten; si un tramo existe pero no es un objeto, el
// archivo no se toca (ErrArchivoIlegible) porque habría que perder lo que hay.
func (d *documento) poner(ruta []string, valor any) error {
	for i := 1; i < len(ruta); i++ {
		n := d.v.Find(puntero(ruta[:i]))
		if n == nil {
			if err := d.anadir(ruta[:i], map[string]any{}); err != nil {
				return err
			}
		} else if _, ok := n.Value.(*hujson.Object); !ok {
			return fmt.Errorf("%w: %s", ErrArchivoIlegible, strings.Join(ruta[:i], "."))
		}
	}
	return d.anadir(ruta, valor)
}

// podar borra los objetos que han quedado vacíos al quitar la ruta, de dentro hacia fuera, sin llegar a la raíz. Límite
// conocido: un objeto que ya estaba vacío antes de activar (`"env": {}`) tampoco sobrevive a desactivar, porque la
// anotación solo guarda claves de hoja y no puede distinguirlo del que creó Tramtai; no cambia el sentido del archivo.
func (d *documento) podar(ruta []string) error {
	for i := len(ruta) - 1; i > 0; i-- {
		if v, hay := d.leer(ruta[:i]); !hay {
			continue
		} else if m, ok := v.(map[string]any); !ok || len(m) > 0 {
			return nil
		}
		if err := d.quitar(ruta[:i]); err != nil {
			return err
		}
	}
	return nil
}

// bytes devuelve el archivo. Si el original ya estaba con el formato de hujson se vuelve a formatear (así lo añadido
// queda igual que el resto); si no, se respeta su estilo y solo cambia lo tocado.
func (d *documento) bytes(original []byte) []byte {
	salida := d.v.Pack()
	cuerpo := bytes.TrimPrefix(original, bom)
	formateado, err := hujson.Format(bytes.Clone(cuerpo)) // Format reescribe el búfer que recibe
	if err == nil && bytes.Equal(formateado, cuerpo) {
		if f, err := hujson.Format(salida); err == nil {
			salida = f
		}
	}
	if d.eol == "\r\n" { // hujson reescribe algunos saltos como «\n» al mover comentarios y comas
		salida = bytes.ReplaceAll(bytes.ReplaceAll(salida, []byte("\r\n"), salto), salto, []byte("\r\n"))
	}
	if d.vacio && !bytes.HasSuffix(salida, salto) {
		salida = append(salida, '\n')
	}
	if d.bom {
		salida = slices.Concat(bom, salida)
	}
	return salida
}

func (f formatoJSON) Aplicar(actual []byte, v Valores) ([]byte, map[string]any, map[string]any, error) {
	claves, err := f.claves(v)
	if err != nil {
		return nil, nil, nil, err
	}
	d, err := abrir(actual)
	if err != nil {
		return nil, nil, nil, err
	}
	escrito, previos := map[string]any{}, map[string]any{}
	for _, c := range claves {
		if previo, hay := d.leer(c.ruta); hay && !c.secreto {
			previos[c.nombre()] = previo
		}
		escrito[c.nombre()] = c.valor
		if c.secreto {
			escrito[c.nombre()] = marca(c.valor.(string))
		}
		if err := d.poner(c.ruta, c.valor); err != nil {
			return nil, nil, nil, err
		}
	}
	return d.bytes(actual), escrito, previos, nil
}

func (f formatoJSON) Quitar(actual []byte, a Anotacion) ([]byte, bool, error) {
	d, err := abrir(actual)
	if err != nil {
		return nil, false, err
	}
	var quitadas [][]string
	for _, k := range clavesOrdenadas(a.Escrito) {
		ruta := strings.Split(k, ".")
		valor, hay := d.leer(ruta)
		if !hay || !coincide(valor, a.Escrito[k]) {
			continue // el usuario la cambió o la borró: ya no es de Tramtai
		}
		if previo, ok := a.Previos[k]; ok {
			err = d.poner(ruta, previo)
		} else {
			err = d.quitar(ruta)
			quitadas = append(quitadas, ruta)
		}
		if err != nil {
			return nil, false, err
		}
	}
	for _, ruta := range quitadas {
		if err := d.podar(ruta); err != nil {
			return nil, false, err
		}
	}
	resto, _ := d.leer(nil)
	m, _ := resto.(map[string]any)
	return d.bytes(actual), len(m) == 0, nil
}

func (f formatoJSON) Contiene(actual []byte, a Anotacion) bool {
	d, err := abrir(actual)
	if err != nil || len(a.Escrito) == 0 {
		return false
	}
	for k, anotado := range a.Escrito {
		if valor, hay := d.leer(strings.Split(k, ".")); !hay || !coincide(valor, anotado) {
			return false
		}
	}
	return true
}

func clavesOrdenadas(m map[string]any) []string {
	l := make([]string, 0, len(m))
	for k := range m {
		l = append(l, k)
	}
	sort.Strings(l)
	return l
}

// ordenEnLinea une la orden con espacios, entre comillas dobles las partes que los tengan: la ejecuta una shell (cmd.exe
// en Windows, según la M0).
// Entre comillas dobles cmd trata como literales los caracteres &^()|<>, pero «%» lo expande aunque haya comillas y no se
// puede escapar de forma fiable: una parte con «%» es un error.
func ordenEnLinea(orden []string) (string, error) {
	partes := make([]string, len(orden))
	for i, p := range orden {
		if strings.Contains(p, "%") {
			return "", errOrdenConPorcentaje
		}
		partes[i] = p
		if strings.ContainsAny(p, " \t&^()|<>") {
			partes[i] = `"` + p + `"`
		}
	}
	return strings.Join(partes, " "), nil
}

// ClaudeCode escribe en settings.json la URL, el modelo y el apiKeyHelper que entrega la clave de la API desde el
// almacén. La M0 con Claude Code 2.1.288 no necesitó ninguna otra variable.
func ClaudeCode() Formato {
	return formatoJSON{claves: func(v Valores) ([]claveJSON, error) {
		if len(v.Orden) == 0 {
			return nil, errSinOrden
		}
		helper, err := ordenEnLinea(v.Orden)
		if err != nil {
			return nil, err
		}
		return []claveJSON{
			{ruta: []string{"env", "ANTHROPIC_BASE_URL"}, valor: v.URLAnthropic},
			{ruta: []string{"env", "ANTHROPIC_MODEL"}, valor: v.Modelo},
			{ruta: []string{"apiKeyHelper"}, valor: helper},
		}, nil
	}}
}

// OpenCode vale para opencode y para Kilo Code, que comparten formato: un proveedor «tramtai» con la clave en el archivo.
func OpenCode() Formato {
	return formatoJSON{claves: func(v Valores) ([]claveJSON, error) {
		if v.Clave == "" {
			return nil, errSinClave
		}
		p := []string{"provider", "tramtai"}
		con := func(resto ...string) []string { return append(append([]string{}, p...), resto...) }
		return []claveJSON{
			{ruta: []string{"model"}, valor: "tramtai/" + v.Modelo},
			{ruta: con("npm"), valor: "@ai-sdk/openai-compatible"},
			{ruta: con("name"), valor: "Tramtai"},
			{ruta: con("options", "baseURL"), valor: v.URLOpenAI},
			{ruta: con("options", "apiKey"), valor: v.Clave, secreto: true},
			// el nombre del modelo lleva «/» (y a veces «.»): va entero como una clave, no como un tramo de la ruta
			{ruta: con("models"), valor: map[string]any{v.Modelo: map[string]any{"name": v.Modelo + " (Tramtai)"}}},
		}, nil
	}}
}

// claudeGestionado dice si alguno de los archivos de ajustes gestionados de Claude Code fija el destino o la clave: una
// política de la organización manda sobre lo que se escriba en settings.json.
func claudeGestionado(rutas []string) bool {
	for _, ruta := range rutas {
		b, err := os.ReadFile(ruta)
		if err != nil {
			continue
		}
		d, err := abrir(b)
		if err != nil {
			continue
		}
		for _, r := range [][]string{{"env", "ANTHROPIC_BASE_URL"}, {"apiKeyHelper"}} {
			if _, hay := d.leer(r); hay {
				return true
			}
		}
	}
	return false
}

func rutaClaudeCode(alcance Alcance, proyecto string) (string, error) {
	if alcance == AlcanceProyecto {
		if proyecto == "" {
			return "", errSinProyecto
		}
		return filepath.Join(proyecto, ".claude", "settings.local.json"), nil
	}
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "settings.json"), nil
	}
	casa, err := os.UserHomeDir()
	return filepath.Join(casa, ".claude", "settings.json"), err
}

// carpetaXDG es ~/.config o, si está puesta, XDG_CONFIG_HOME (opencode la respeta, según la M0).
func carpetaXDG() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir, nil
	}
	casa, err := os.UserHomeDir()
	return filepath.Join(casa, ".config"), err
}

func rutaOpenCode(_ Alcance, _ string) (string, error) {
	base, err := carpetaXDG()
	if err != nil {
		return "", err
	}
	jsonc := filepath.Join(base, "opencode", "opencode.jsonc")
	if _, err := os.Stat(jsonc); err == nil {
		return jsonc, nil
	}
	return filepath.Join(base, "opencode", "opencode.json"), nil
}

func rutaKilo(_ Alcance, _ string) (string, error) {
	base, err := carpetaXDG()
	return filepath.Join(base, "kilo", "kilo.jsonc"), err
}

func gestionadoClaudeCode() bool {
	return claudeGestionado([]string{
		`C:\Program Files\ClaudeCode\managed-settings.json`,
		"/Library/Application Support/ClaudeCode/managed-settings.json",
		"/etc/claude-code/managed-settings.json",
	})
}
