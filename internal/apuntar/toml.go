package apuntar

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable" // API «unstable»: fijada en go.mod; revisar al subir go-toml

	"tramtai/internal/idioma"
)

const (
	marcaIni         = "# >>> tramtai"
	marcaFin         = "# <<< tramtai"
	prefijoComentado = "# tramtai: " // lo que se antepone a una línea del usuario que Tramtai tiene que apartar
)

var (
	// errProveedorAjeno: la tabla del proveedor ya existe y no la escribió Tramtai; fusionarla o pisarla perdería lo del usuario.
	errProveedorAjeno = errors.New(idioma.T("ya hay un proveedor «tramtai» en config.toml que no es de Tramtai: renómbralo o quítalo"))
	// errClaveVariasLineas: Tramtai comenta la línea entera de model / model_provider, y eso solo vale si ocupan una.
	errClaveVariasLineas = errors.New(idioma.T("«model» o «model_provider» de config.toml ocupa varias líneas: déjalo en una sola"))
)

// clavesCodex son las claves de nivel superior que Tramtai fija y que no pueden estar dos veces.
var clavesCodex = []string{"model", "model_provider"}

// formatoCodex edita config.toml por líneas, sin reserializar: lo que no es de Tramtai queda byte a byte como estaba.
// Tramtai escribe dos bloques entre marcas (el modelo, arriba; el proveedor, al final) y comenta con un prefijo las líneas
// «model» y «model_provider» que ya hubiera. go-toml solo sirve para ubicar claves y para validar el resultado.
//
// Quitar no se fía de las marcas: quita lo que sigue valiendo lo que Tramtai escribió, esté o no entre marcas, y deja lo que
// el usuario cambió o añadió.
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
// del archivo, cada una con su terminador. Con conservar, de cada bloque de Tramtai solo se quitan las marcas; sin él, se
// quita el bloque entero. Un «# >>> tramtai» sin su cierre no es un bloque. sinFinal dice que el archivo acababa en la marca
// de cierre sin salto de línea: Aplicar escribe así el último bloque, y quien quita debe dejar la última línea sin salto.
func abrirTOML(actual []byte, conservar bool) (conBOM bool, eol string, ls []string, sinFinal bool) {
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
				if conservar {
					ls = append(ls, todas[i+1:j]...)
				}
				sinFinal = j == len(todas)-1 && !strings.HasSuffix(todas[j], "\n")
				i = j
				continue
			}
		}
		ls = append(ls, todas[i])
	}
	return conBOM, eol, ls, sinFinal
}

func indiceFin(ls []string, desde int) int {
	for j := desde; j < len(ls); j++ {
		if esMarca(ls[j], marcaFin) {
			return j
		}
	}
	return -1
}

// cabecera es una tabla del archivo: su nombre con puntos y la línea de su encabezado.
type cabecera struct {
	nombre string
	linea  int
}

// ubicacion es lo que se sabe del archivo tras validarlo.
type ubicacion struct {
	claves    map[string]int // línea de cada clave de nivel superior de una sola parte
	tabla     int            // línea de la primera tabla (len(ls) si no hay)
	tablas    []cabecera     // todas las tablas, en orden
	contenido map[string]any
}

// ubicar valida el TOML y localiza sus claves de nivel superior y sus tablas.
func ubicar(ls []string) (ubicacion, error) {
	u := ubicacion{claves: map[string]int{}, tabla: len(ls)}
	texto := []byte(strings.Join(ls, ""))
	if err := toml.Unmarshal(texto, &u.contenido); err != nil {
		return u, fmt.Errorf("%w: %v", ErrArchivoIlegible, err)
	}
	inicios, desde := make([]int, len(ls)), 0
	for i, l := range ls {
		inicios[i], desde = desde, desde+len(l)
	}
	linea := func(r unstable.Range) int { return sort.SearchInts(inicios, int(r.Offset)+1) - 1 }
	var p unstable.Parser
	p.Reset(texto)
	for p.NextExpression() {
		e := p.Expression()
		switch e.Kind {
		case unstable.Table, unstable.ArrayTable:
			var partes []string
			l := -1
			for it := e.Key(); it.Next(); {
				if l < 0 {
					l = linea(it.Node().Raw)
				}
				partes = append(partes, string(it.Node().Data))
			}
			u.tablas = append(u.tablas, cabecera{strings.Join(partes, "."), l})
			u.tabla = min(u.tabla, l)
		case unstable.KeyValue:
			if u.tabla < len(ls) {
				continue
			}
			it := e.Key()
			it.Next()
			if k := it.Node(); it.IsLast() {
				u.claves[string(k.Data)] = linea(k.Raw)
			}
		}
	}
	return u, nil
}

// subarbol baja por las claves del mapa; nil si falta alguna.
func subarbol(m map[string]any, ruta ...string) any {
	var v any = m
	for _, parte := range ruta {
		t, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = t[parte]
	}
	return v
}

// unaLinea dice si la línea sola es «clave = valor» con ese valor: así se sabe que cabe en ella y que es lo que se escribió.
func unaLinea(l, clave string, valor any) bool {
	var m map[string]any
	return valor != nil && toml.Unmarshal([]byte(l), &m) == nil && len(m) == 1 && igual(m[clave], valor)
}

// bloque une las líneas entre las marcas; no lleva salto tras la marca de cierre.
func bloque(eol string, lineas ...string) string {
	return marcaIni + eol + strings.Join(lineas, eol) + eol + marcaFin
}

func (formatoCodex) Aplicar(actual []byte, v Valores) ([]byte, map[string]any, map[string]any, error) {
	if len(v.Orden) == 0 {
		return nil, nil, nil, errSinOrden
	}
	conBOM, eol, ls, sinFinal := abrirTOML(actual, false)
	if sinFinal && len(ls) > 0 {
		ls[len(ls)-1] = finDeLinea(ls[len(ls)-1])
	}
	u, err := ubicar(ls)
	if err != nil {
		return nil, nil, nil, err
	}
	if subarbol(u.contenido, "model_providers", "tramtai") != nil {
		return nil, nil, nil, errProveedorAjeno
	}
	previos := map[string]any{}
	for _, k := range clavesCodex {
		if i, hay := u.claves[k]; hay { // TOML no admite claves repetidas: la del usuario se aparta, no se borra
			if !unaLinea(ls[i], k, u.contenido[k]) {
				return nil, nil, nil, errClaveVariasLineas
			}
			previos[k], ls[i] = u.contenido[k], prefijoComentado+ls[i]
		}
	}
	proveedor := []string{"[model_providers.tramtai]", `name = "Tramtai"`, "base_url = " + cadenaBasica(v.URLOpenAI),
		`wire_api = "responses"`, "", "[model_providers.tramtai.auth]", "command = " + cadenaTOML(v.Orden[0])}
	escrito := map[string]any{"model": v.Modelo, "model_provider": "tramtai",
		"model_providers.tramtai.base_url": v.URLOpenAI, "model_providers.tramtai.auth.command": v.Orden[0]}
	if len(v.Orden) > 1 {
		args, anotados := make([]string, len(v.Orden)-1), make([]any, len(v.Orden)-1)
		for i, a := range v.Orden[1:] {
			args[i], anotados[i] = cadenaBasica(a), a
		}
		proveedor = append(proveedor, "args = ["+strings.Join(args, ", ")+"]")
		escrito["model_providers.tramtai.auth.args"] = anotados
	}
	// si el archivo no acababa en salto de línea, el bloque final tampoco: así Quitar lo deja como estaba
	sinSalto := len(ls) > 0 && !strings.HasSuffix(ls[len(ls)-1], "\n")
	cola := bloque(eol, proveedor...)
	if sinSalto {
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
	salida := b.String()
	if conBOM {
		salida = string(bom) + salida
	}
	return []byte(salida), escrito, previos, nil
}

var claveDeLinea = regexp.MustCompile(`^\s*["']?([A-Za-z0-9_-]+)["']?\s*=`)

// proveedorEscrito es lo que debe valer la tabla model_providers.tramtai para ser entera la que escribió Tramtai.
func proveedorEscrito(e map[string]any) map[string]any {
	url, ok1 := e["model_providers.tramtai.base_url"]
	cmd, ok2 := e["model_providers.tramtai.auth.command"]
	if !ok1 || !ok2 {
		return nil
	}
	auth := map[string]any{"command": cmd}
	if args, ok := e["model_providers.tramtai.auth.args"]; ok {
		auth["args"] = args
	}
	return map[string]any{"name": "Tramtai", "base_url": url, "wire_api": "responses", "auth": auth}
}

func (formatoCodex) Quitar(actual []byte, a Anotacion) ([]byte, bool, error) {
	conBOM, _, ls, sinFinal := abrirTOML(actual, true)
	u, err := ubicar(ls)
	if err != nil {
		return nil, false, err
	}
	quitar := map[int]bool{}
	// las líneas de nivel superior que siguen valiendo lo escrito; si el usuario cambió el valor, la línea es suya y se queda
	for _, k := range clavesCodex {
		if i, hay := u.claves[k]; hay && unaLinea(ls[i], k, a.Escrito[k]) {
			quitar[i] = true
		}
	}
	// el proveedor solo se quita si sigue exactamente como se escribió, sin claves de más
	if esperado := proveedorEscrito(a.Escrito); esperado != nil && igual(subarbol(u.contenido, "model_providers", "tramtai"), esperado) {
		propia := func(c cabecera) bool {
			return c.nombre == "model_providers.tramtai" || c.nombre == "model_providers.tramtai.auth"
		}
		propias := func(i int) bool { return i < len(u.tablas) && propia(u.tablas[i]) }
		// ambos encabezados deben estar: si el contenido sale de otra forma (claves con puntos), no se toca
		if slices.ContainsFunc(u.tablas, func(c cabecera) bool { return c.nombre == "model_providers.tramtai" }) &&
			slices.ContainsFunc(u.tablas, func(c cabecera) bool { return c.nombre == "model_providers.tramtai.auth" }) {
			for i, c := range u.tablas {
				if !propias(i) {
					continue
				}
				fin := len(ls)
				if i+1 < len(u.tablas) {
					fin = u.tablas[i+1].linea
				}
				if !propias(i + 1) { // lo que cuelga tras la última tabla propia (espacios, notas) no es de Tramtai
					for fin > c.linea+1 && (strings.TrimSpace(ls[fin-1]) == "" || strings.HasPrefix(strings.TrimSpace(ls[fin-1]), "#")) {
						fin--
					}
				}
				for l := c.linea; l < fin; l++ {
					quitar[l] = true
				}
			}
		}
	}
	for i := 0; i < u.tabla; i++ {
		resto, comentada := strings.CutPrefix(ls[i], prefijoComentado)
		if !comentada {
			continue
		}
		if m := claveDeLinea.FindStringSubmatch(resto); m != nil {
			_, eraSuya := a.Previos[m[1]]
			otra, hayOtra := u.claves[m[1]] // si el usuario conserva la suya, descomentar la dejaría repetida
			if eraSuya && (!hayOtra || quitar[otra]) {
				ls[i] = resto
			}
		}
	}
	var salida []string
	for i, l := range ls {
		if !quitar[i] {
			salida = append(salida, l)
		}
	}
	if sinFinal && len(salida) > 0 {
		salida[len(salida)-1] = finDeLinea(salida[len(salida)-1])
	}
	cuerpo := strings.Join(salida, "")
	vacio := strings.TrimSpace(cuerpo) == ""
	if conBOM {
		cuerpo = string(bom) + cuerpo
	}
	return []byte(cuerpo), vacio, nil
}

func (formatoCodex) Contiene(actual []byte, a Anotacion) bool {
	var m map[string]any
	if len(a.Escrito) == 0 || toml.Unmarshal(bytes.TrimPrefix(actual, bom), &m) != nil {
		return false
	}
	for k, anotado := range a.Escrito {
		if !igual(subarbol(m, strings.Split(k, ".")...), anotado) {
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
