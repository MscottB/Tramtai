package main

import (
	"fmt"
	"html"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

var (
	preSitio      = regexp.MustCompile(`(?s)<pre[^>]*>(.*?)</pre>`)
	etiquetaSitio = regexp.MustCompile(`<(/?)([a-zA-Z0-9]+)([^>]*)>`)
	langSitio     = regexp.MustCompile(`\blang="(\w+)"`)
)

// neutrosSitio son nombres que se escriben igual en los dos idiomas: valen en una línea en inglés sin alias.
var neutrosSitio = map[string]bool{"terminal": true, "token": true, "totp": true, "version": true}

// ordenesNuevasSitio son las líneas que enseñará el sitio nuevo (Tareas 15 y 16), todas en inglés.
var ordenesNuevasSitio = []string{
	"tramtai service start -d",
	"sudo ./tramtai server install",
	"sudo tramtai-servidor admin password",
	`.\tramtai.exe server install`,
}

// lineasDePre separa el contenido de un <pre> en líneas de texto. es es cada línea para quien lee en español: lo que no
// está marcado y lo marcado lang="es". en solo trae las líneas que llevan algo lang="en", con lo no marcado más lo
// marcado lang="en". Una línea sin nada lang="en" (aunque esté dentro de lang="es") es solo español.
func lineasDePre(contenido string) (es, en []string) {
	var pila []string // el idioma efectivo de cada elemento abierto ("" si ningún ancestro lo marca)
	var bufEs, bufEn strings.Builder
	hayEn := false
	cerrar := func() {
		es = append(es, bufEs.String())
		if hayEn {
			en = append(en, bufEn.String())
		}
		bufEs.Reset()
		bufEn.Reset()
		hayEn = false
	}
	texto := func(t string) {
		lang := ""
		if len(pila) > 0 {
			lang = pila[len(pila)-1]
		}
		for i, trozo := range strings.Split(t, "\n") {
			if i > 0 {
				cerrar()
			}
			if lang != "en" {
				bufEs.WriteString(trozo)
			}
			if lang != "es" {
				bufEn.WriteString(trozo)
			}
			hayEn = hayEn || lang == "en" && strings.TrimSpace(trozo) != ""
		}
	}
	resto := contenido
	for {
		m := etiquetaSitio.FindStringSubmatchIndex(resto)
		if m == nil {
			texto(resto)
			break
		}
		texto(resto[:m[0]])
		cierre, nombre, atributos := resto[m[2]:m[3]] == "/", resto[m[4]:m[5]], resto[m[6]:m[7]]
		switch {
		case nombre == "br":
			cerrar()
		case cierre:
			if len(pila) > 0 {
				pila = pila[:len(pila)-1]
			}
		default:
			lang := ""
			if len(pila) > 0 {
				lang = pila[len(pila)-1]
			}
			if l := langSitio.FindStringSubmatch(atributos); l != nil {
				lang = l[1]
			}
			pila = append(pila, lang)
		}
		resto = resto[m[1]:]
	}
	cerrar()
	return es, en
}

// ordenDeLinea saca los argumentos (con sus opciones) de una línea que es una orden de Tramtai: tras un sudo y un ./ o .\
// opcionales, empieza por tramtai, tramtai.exe o tramtai-servidor. Las demás (brew, tar, curl, comentarios…) no lo son.
// Un # corta la línea y una \ final (continuación) no es un argumento.
func ordenDeLinea(linea string) (args []string, servidor, ok bool) {
	campos := strings.Fields(html.UnescapeString(linea))
	if len(campos) > 0 && campos[0] == "sudo" {
		campos = campos[1:]
	}
	if len(campos) == 0 {
		return nil, false, false
	}
	programa := strings.TrimSuffix(strings.TrimLeft(campos[0], `./\`), ".exe")
	if programa != "tramtai" && programa != "tramtai-servidor" {
		return nil, false, false
	}
	for _, a := range campos[1:] {
		if strings.HasPrefix(a, "#") {
			break
		}
		if a != `\` {
			args = append(args, a)
		}
	}
	return args, programa == "tramtai-servidor", true
}

// sinOpciones quita de los argumentos las opciones de cmd y los valores de las que lo piden (--modelo x, -a x), con las
// definiciones de cobra: una opción booleana no se come el argumento siguiente. Una opción que cmd no tiene es un error.
func sinOpciones(cmd *cobra.Command, args []string) ([]string, error) {
	cmd.InheritedFlags() // junta las opciones persistentes de los padres en cmd.Flags()
	var limpios []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			limpios = append(limpios, a)
			continue
		}
		nombre, _, conValor := strings.Cut(strings.TrimLeft(a, "-"), "=")
		var existe, pideValor bool
		if strings.HasPrefix(a, "--") {
			if f := cmd.Flags().Lookup(nombre); f != nil {
				existe, pideValor = true, f.NoOptDefVal == ""
			}
		} else if f := cmd.Flags().ShorthandLookup(nombre[:1]); f != nil {
			existe, pideValor = true, f.NoOptDefVal == ""
		}
		switch {
		case !existe:
			return nil, fmt.Errorf("%q no tiene la opción %s", cmd.CommandPath(), a)
		case pideValor && !conValor && i+1 == len(args):
			return nil, fmt.Errorf("la opción %s necesita un valor", a)
		case pideValor && !conValor:
			i++
		}
	}
	return limpios, nil
}

// comprobarOrdenSitio dice por qué una orden no existe en su idioma. En inglés cada nombre escrito tiene que ser un alias
// o un nombre neutro, no el nombre en español que Find también acepta. tramtai-servidor equivale a tramtai --servidor: el
// «servidor» que ordenDeServidor añade no lo escribe el usuario y no se comprueba.
func comprobarOrdenSitio(raiz *cobra.Command, args []string, servidor, ingles bool) error {
	insertado := false
	if servidor {
		completo := ordenDeServidor(raiz, append([]string{"--servidor"}, args...))
		insertado = len(completo) == len(args)+2
		args = completo[1:]
	}
	cmd, _, err := raiz.Find(args)
	if err != nil {
		return err
	}
	limpios, err := sinOpciones(cmd, args)
	if err != nil {
		return err
	}
	var ruta []*cobra.Command
	for c := cmd; c.HasParent(); c = c.Parent() {
		ruta = slices.Insert(ruta, 0, c)
	}
	if len(limpios) < len(ruta) {
		return fmt.Errorf("Find resolvió %d comandos con %d palabras", len(ruta), len(limpios))
	}
	palabras, resto := limpios[:len(ruta)], limpios[len(ruta):]
	if cmd.HasSubCommands() && len(resto) > 0 { // Find no falla con un subcomando desconocido dentro de un grupo
		return fmt.Errorf("«%s» no existe dentro de %q", resto[0], cmd.CommandPath())
	}
	for i, c := range ruta {
		if ingles && !(i == 0 && insertado) && !c.HasAlias(palabras[i]) && !(neutrosSitio[palabras[i]] && c.Name() == palabras[i]) {
			return fmt.Errorf("«%s» solo se resuelve con el nombre en español de %q: falta su alias en inglés", palabras[i], c.CommandPath())
		}
	}
	return nil
}

// TestOrdenesDelSitioIdioma: una línea en inglés no pasa con un nombre en español, ni siquiera el que cobra acepta.
func TestOrdenesDelSitioIdioma(t *testing.T) {
	raiz := nuevaRaiz()
	for _, c := range []struct {
		args             []string
		servidor, ingles bool
		correcta         bool
	}{
		{[]string{"servicio", "iniciar"}, false, false, true},
		{[]string{"servicio", "iniciar"}, false, true, false},
		{[]string{"service", "start"}, false, true, true},
		{[]string{"service", "iniciar"}, false, true, false},
		{[]string{"terminal"}, false, true, true},
		{[]string{"instalar"}, true, false, true},
		{[]string{"instalar"}, true, true, false},
		{[]string{"install"}, true, true, true},
		{[]string{"admin", "password"}, true, true, true},
		{[]string{"admin", "totp", "enable"}, true, true, true},
		{[]string{"administrador", "totp", "activar"}, true, false, true},
		// Un grupo en medio, una opción booleana y otra con valor (que no es un argumento ni una palabra del comando).
		{[]string{"tools", "enable", "--all", "--model", "claude/haiku"}, false, true, true},
		{[]string{"tools", "enable", "claude-code", "--model=claude/haiku"}, false, true, true},
		{[]string{"herramientas", "activar", "--todas", "--modelo", "claude/haiku"}, false, false, true},
		{[]string{"tools", "enable", "--all", "--model"}, false, true, false},
		{[]string{"tools", "enable", "--nada"}, false, true, false},
		{[]string{"tools", "activar", "--all"}, false, true, false},
		{[]string{"service", "start", "-d"}, false, true, true},
		{[]string{"servicio", "no-existe"}, false, false, false},
		{[]string{"no-existe"}, false, false, false},
	} {
		if err := comprobarOrdenSitio(raiz, c.args, c.servidor, c.ingles); (err == nil) != c.correcta {
			t.Errorf("%v (servidor %v, inglés %v): %v, correcta %v", c.args, c.servidor, c.ingles, err, c.correcta)
		}
	}
}

// TestComandosDelSitioExisten: cada orden de tramtai que enseñan los <pre> de index.html y servidor.html existe, y las de
// los lang="en" existen con nombres en inglés. Lo que no está dentro de ningún lang se lee como español (se enseña en los
// dos idiomas, así que las Tareas 15 y 16 lo marcan por idioma). Solo lee archivos y llama a Find: no ejecuta nada.
func TestComandosDelSitioExisten(t *testing.T) {
	raiz := nuevaRaiz()
	type linea struct {
		fuente, texto string
		ingles        bool
	}
	var lineas []linea
	for _, archivo := range []string{"index.html", "servidor.html"} {
		datos, err := os.ReadFile("../../site/" + archivo)
		if err != nil {
			t.Fatal(err)
		}
		antes := len(lineas)
		for _, pre := range preSitio.FindAllStringSubmatch(string(datos), -1) {
			es, en := lineasDePre(pre[1])
			for _, l := range es {
				lineas = append(lineas, linea{archivo, l, false})
			}
			for _, l := range en {
				lineas = append(lineas, linea{archivo, l, true})
			}
		}
		if len(lineas) == antes {
			t.Fatalf("%s: no hay ningún <pre>: ¿cambió el marcado?", archivo)
		}
	}
	for _, o := range ordenesNuevasSitio {
		lineas = append(lineas, linea{"Tareas 15 y 16", o, true})
	}
	revisadas := 0
	for _, l := range lineas {
		args, servidor, ok := ordenDeLinea(l.texto)
		if !ok {
			continue
		}
		revisadas++
		if err := comprobarOrdenSitio(raiz, args, servidor, l.ingles); err != nil {
			t.Errorf("%s: %q (inglés %v): %v", l.fuente, strings.TrimSpace(html.UnescapeString(l.texto)), l.ingles, err)
		}
	}
	// Hoy son 10 (4 nuevas y 6 de los <pre>): perder los bloques de index.html o servidor.html baja de aquí.
	if revisadas < 10 {
		t.Errorf("solo se revisaron %d órdenes (mínimo 10): ¿cambió el marcado de los <pre>?", revisadas)
	}
}

// TestOrdenDeLineaSitio: sudo, ./ y .\ no cuentan, un # corta, una \ final no es un argumento y lo que no es de Tramtai se ignora.
func TestOrdenDeLineaSitio(t *testing.T) {
	for linea, esperada := range map[string]string{
		`sudo ./tramtai server install`:             "tramtai|server install",
		`.\tramtai.exe tools enable --all \`:        "tramtai|tools enable --all",
		`sudo tramtai-servidor admin password # ok`: "servidor|admin password",
		`brew install tramtai`:                      "",
		`# tramtai terminal`:                        "",
	} {
		args, servidor, ok := ordenDeLinea(linea)
		r := ""
		if ok {
			r = map[bool]string{false: "tramtai", true: "servidor"}[servidor] + "|" + strings.Join(args, " ")
		}
		if r != esperada {
			t.Errorf("%q: %q, se esperaba %q", linea, r, esperada)
		}
	}
}
