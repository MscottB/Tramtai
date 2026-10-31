package apuntar

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

const (
	marcaIni         = "# >>> tramtai"
	marcaFin         = "# <<< tramtai"
	prefijoComentado = "# tramtai: " // lo que se antepone a una línea del usuario que Tramtai tiene que apartar
)

// clavesCodex son las claves de nivel superior que Tramtai fija y que no pueden estar dos veces.
var clavesCodex = []string{"model", "model_provider"}

// formatoCodex edita config.toml por líneas, sin reserializar: lo que no es de Tramtai queda byte a byte como estaba.
// Tramtai escribe dos bloques entre marcas (el modelo, arriba; el proveedor, al final) y comenta con un prefijo las líneas
// «model» y «model_provider» que ya hubiera. go-toml solo sirve para ubicar claves y para validar el resultado.
type formatoCodex struct{}

// Codex apunta Codex CLI a Tramtai. La clave no va en el archivo: auth.command ejecuta Tramtai, que la imprime.
func Codex() Formato { return formatoCodex{} }

// cadenaTOML escribe s como cadena TOML: literal ('…') si puede, para que las barras de una ruta de Windows no se
// escapen; si lleva una comilla simple o un carácter de control, como cadena básica con su escape.
func cadenaTOML(s string) string {
	if strings.IndexFunc(s, func(r rune) bool { return r == '\'' || r == 0x7f || (r < 0x20 && r != '\t') }) < 0 {
		return "'" + s + "'"
	}
	return cadenaBasica(s)
}

func cadenaBasica(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func finDeLinea(l string) string { return strings.TrimSuffix(strings.TrimSuffix(l, "\n"), "\r") }

func esMarca(l, marca string) bool { return strings.TrimSpace(finDeLinea(l)) == marca }

// abrirTOML separa la marca de orden de bytes, deduce el fin de línea («\r\n» solo si todos lo son) y devuelve las líneas
// del archivo sin los bloques de Tramtai, cada una con su terminador. Un bloque sin su marca de cierre no se toca.
func abrirTOML(actual []byte) (conBOM bool, eol string, ls []string) {
	cuerpo := bytes.TrimPrefix(actual, bom)
	conBOM, eol = len(cuerpo) != len(actual), "\n"
	if n := bytes.Count(cuerpo, salto); n > 0 && n == bytes.Count(cuerpo, []byte("\r\n")) {
		eol = "\r\n"
	}
	todas := strings.SplitAfter(string(cuerpo), "\n")
	if todas[len(todas)-1] == "" {
		todas = todas[:len(todas)-1]
	}
	for i := 0; i < len(todas); i++ {
		if esMarca(todas[i], marcaIni) {
			if j := indiceFin(todas, i+1); j >= 0 {
				// el último bloque no llevaba salto final porque el archivo tampoco: la línea anterior vuelve a quedar sin él
				if j == len(todas)-1 && !strings.HasSuffix(todas[j], "\n") && len(ls) > 0 {
					ls[len(ls)-1] = finDeLinea(ls[len(ls)-1])
				}
				i = j
				continue
			}
		}
		ls = append(ls, todas[i])
	}
	return conBOM, eol, ls
}

func indiceFin(ls []string, desde int) int {
	for j := desde; j < len(ls); j++ {
		if esMarca(ls[j], marcaFin) {
			return j
		}
	}
	return -1
}

// ubicar valida el TOML y dice en qué línea están las claves de nivel superior de una sola parte, en qué línea empieza la
// primera tabla (len(ls) si no hay) y el contenido ya interpretado.
func ubicar(ls []string) (claves map[string]int, tabla int, contenido map[string]any, err error) {
	texto := []byte(strings.Join(ls, ""))
	if err := toml.Unmarshal(texto, &contenido); err != nil {
		return nil, 0, nil, fmt.Errorf("%w: %v", ErrArchivoIlegible, err)
	}
	inicios, desde := make([]int, len(ls)), 0
	for i, l := range ls {
		inicios[i], desde = desde, desde+len(l)
	}
	linea := func(r unstable.Range) int { return sort.SearchInts(inicios, int(r.Offset)+1) - 1 }
	claves, tabla = map[string]int{}, len(ls)
	var p unstable.Parser
	p.Reset(texto)
	for p.NextExpression() {
		e := p.Expression()
		if e.Kind == unstable.Table || e.Kind == unstable.ArrayTable {
			it := e.Key()
			it.Next()
			tabla = linea(it.Node().Raw)
			break
		}
		if e.Kind == unstable.KeyValue {
			it := e.Key()
			it.Next()
			if k := it.Node(); it.IsLast() {
				claves[string(k.Data)] = linea(k.Raw)
			}
		}
	}
	return claves, tabla, contenido, nil
}

// bloque une las líneas entre las marcas; no lleva salto tras la marca de cierre.
func bloque(eol string, lineas ...string) string {
	return marcaIni + eol + strings.Join(lineas, eol) + eol + marcaFin
}

func (formatoCodex) Aplicar(actual []byte, v Valores) ([]byte, map[string]any, map[string]any, error) {
	if len(v.Orden) == 0 {
		return nil, nil, nil, errSinOrden
	}
	conBOM, eol, ls := abrirTOML(actual)
	claves, _, contenido, err := ubicar(ls)
	if err != nil {
		return nil, nil, nil, err
	}
	previos := map[string]any{}
	for _, k := range clavesCodex {
		if i, hay := claves[k]; hay { // TOML no admite claves repetidas: la del usuario se aparta, no se borra
			previos[k], ls[i] = contenido[k], prefijoComentado+ls[i]
		}
	}
	proveedor := []string{"[model_providers.tramtai]", `name = "Tramtai"`, "base_url = " + cadenaBasica(v.URLOpenAI),
		`wire_api = "responses"`, "", "[model_providers.tramtai.auth]", "command = " + cadenaTOML(v.Orden[0])}
	if len(v.Orden) > 1 {
		args := make([]string, len(v.Orden)-1)
		for i, a := range v.Orden[1:] {
			args[i] = cadenaBasica(a)
		}
		proveedor = append(proveedor, "args = ["+strings.Join(args, ", ")+"]")
	}
	// si el archivo no acababa en salto de línea, el bloque final tampoco: así Quitar lo deja como estaba
	sinFinal := len(ls) > 0 && !strings.HasSuffix(ls[len(ls)-1], "\n")
	cola := bloque(eol, proveedor...)
	if sinFinal {
		cola = eol + cola
	} else {
		cola += eol
	}
	var b strings.Builder
	b.WriteString(bloque(eol, "model = "+cadenaBasica(v.Modelo), `model_provider = "tramtai"`) + eol)
	b.WriteString(strings.Join(ls, ""))
	b.WriteString(cola)
	var resultado map[string]any
	if err := toml.Unmarshal([]byte(b.String()), &resultado); err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %v", ErrArchivoIlegible, err)
	}
	if resultado["model_provider"] != "tramtai" {
		return nil, nil, nil, ErrArchivoIlegible
	}
	escrito := map[string]any{"model": v.Modelo, "model_provider": "tramtai",
		"model_providers.tramtai.base_url": v.URLOpenAI, "model_providers.tramtai.auth.command": v.Orden[0]}
	salida := b.String()
	if conBOM {
		salida = string(bom) + salida
	}
	return []byte(salida), escrito, previos, nil
}

var claveDeLinea = regexp.MustCompile(`^\s*["']?([A-Za-z0-9_-]+)["']?\s*=`)

func (formatoCodex) Quitar(actual []byte, a Anotacion) ([]byte, bool, error) {
	conBOM, _, ls := abrirTOML(actual)
	claves, tabla, _, err := ubicar(ls)
	if err != nil {
		return nil, false, err
	}
	for i := 0; i < tabla; i++ {
		resto, comentada := strings.CutPrefix(ls[i], prefijoComentado)
		if !comentada {
			continue
		}
		if m := claveDeLinea.FindStringSubmatch(resto); m != nil {
			_, eraSuya := a.Previos[m[1]]
			_, hayOtra := claves[m[1]] // si el usuario ya puso la suya, descomentar la dejaría repetida
			if eraSuya && !hayOtra {
				ls[i] = resto
			}
		}
	}
	cuerpo := strings.Join(ls, "")
	if conBOM {
		cuerpo = string(bom) + cuerpo
	}
	return []byte(cuerpo), strings.TrimSpace(cuerpo) == "", nil
}

func (formatoCodex) Contiene(actual []byte, a Anotacion) bool {
	var m map[string]any
	if len(a.Escrito) == 0 || toml.Unmarshal(bytes.TrimPrefix(actual, bom), &m) != nil {
		return false
	}
	for k, anotado := range a.Escrito {
		var valor any = m
		for _, parte := range strings.Split(k, ".") {
			t, ok := valor.(map[string]any)
			if !ok {
				return false
			}
			valor = t[parte]
		}
		if !igual(valor, anotado) {
			return false
		}
	}
	return true
}

// rutaCodex es $CODEX_HOME/config.toml o, sin esa variable, ~/.codex/config.toml.
func rutaCodex(_ Alcance, _ string) (string, error) {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return filepath.Join(dir, "config.toml"), nil
	}
	casa, err := os.UserHomeDir()
	return filepath.Join(casa, ".codex", "config.toml"), err
}
