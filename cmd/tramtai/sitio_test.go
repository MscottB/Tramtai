package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
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

// vaciosSitio son las etiquetas sin cierre: no entran en la pila de lenguas.
var vaciosSitio = map[string]bool{"meta": true, "link": true, "img": true, "br": true, "input": true, "source": true, "hr": true}

// codigosEnLinea saca el texto de cada <code> fuera de un <pre> con su idioma efectivo ("" si nada lo marca): las órdenes
// que la página cita dentro de una frase.
func codigosEnLinea(pagina string) (codigos []struct{ texto, lang string }) {
	type abierta struct{ nombre, lang string }
	var pila []abierta
	inicio, langCodigo := -1, ""
	for _, m := range etiquetaSitio.FindAllStringSubmatchIndex(pagina, -1) {
		cierre, nombre, atributos := pagina[m[2]:m[3]] == "/", strings.ToLower(pagina[m[4]:m[5]]), pagina[m[6]:m[7]]
		lang, enPre := "", false
		for _, a := range pila {
			if a.lang != "" {
				lang = a.lang
			}
			enPre = enPre || a.nombre == "pre"
		}
		switch {
		case cierre:
			for i := len(pila) - 1; i >= 0; i-- {
				if pila[i].nombre == nombre {
					pila = pila[:i]
					break
				}
			}
			if nombre == "code" && inicio >= 0 {
				texto := etiquetaSitio.ReplaceAllString(pagina[inicio:m[0]], "")
				codigos = append(codigos, struct{ texto, lang string }{html.UnescapeString(texto), langCodigo})
				inicio = -1
			}
		case vaciosSitio[nombre] || strings.HasSuffix(atributos, "/"):
		default:
			if l := langSitio.FindStringSubmatch(atributos); l != nil {
				lang = l[1]
			}
			pila = append(pila, abierta{nombre, lang})
			if nombre == "code" && !enPre {
				inicio, langCodigo = m[1], lang
			}
		}
	}
	return codigos
}

// ordenDeLinea saca los argumentos (con sus opciones) de una línea que es una orden de Tramtai: tras un sudo y un ./ o .\
// opcionales, empieza por tramtai, tramtai.exe o tramtai-servidor. Las demás (brew, tar, curl, comentarios…) no lo son.
// Un # o unos puntos suspensivos («tramtai-servidor agentes …») cortan la línea; una \ final (continuación) y un hueco
// para rellenar (<id>, <orden>) no son argumentos.
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
		if strings.HasPrefix(a, "#") || a == "…" {
			break
		}
		if a != `\` && !(strings.HasPrefix(a, "<") && strings.HasSuffix(a, ">")) {
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
		switch {
		case nombre == "":
		case strings.HasPrefix(a, "--"):
			if f := cmd.Flags().Lookup(nombre); f != nil {
				existe, pideValor = true, f.NoOptDefVal == ""
			}
		default:
			if f := cmd.Flags().ShorthandLookup(nombre[:1]); f != nil {
				existe, pideValor = true, f.NoOptDefVal == ""
				conValor = conValor || len(nombre) > 1 // -n50: el valor va pegado
			}
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

// TestSinOpciones: sinOpciones quita las opciones y sus valores (separados, pegados o con =) y deja los argumentos;
// una opción desconocida, sin su valor o sin nombre es un error.
func TestSinOpciones(t *testing.T) {
	cmd := &cobra.Command{Use: "prueba"}
	cmd.Flags().IntP("numero", "n", 0, "")
	cmd.Flags().StringP("modelo", "m", "", "")
	cmd.Flags().BoolP("todas", "t", false, "")
	for _, c := range []struct {
		args, quiere []string
	}{
		{[]string{"a", "-n", "50", "b"}, []string{"a", "b"}},
		{[]string{"a", "-n50", "b"}, []string{"a", "b"}}, // valor pegado: b no es el valor
		{[]string{"-n=50", "a"}, []string{"a"}},
		{[]string{"--modelo", "x", "a"}, []string{"a"}},
		{[]string{"--modelo=x", "a"}, []string{"a"}},
		{[]string{"-t", "a"}, []string{"a"}}, // booleana: no se come a
		{[]string{"-", "a"}, []string{"-", "a"}},
	} {
		if got, err := sinOpciones(cmd, c.args); err != nil || !slices.Equal(got, c.quiere) {
			t.Errorf("%q -> %q, %v; quería %q", c.args, got, err, c.quiere)
		}
	}
	for _, args := range [][]string{{"-=x"}, {"--=x"}, {"-z"}, {"--otra"}, {"-m"}} {
		if _, err := sinOpciones(cmd, args); err == nil {
			t.Errorf("%q debería ser un error", args)
		}
	}
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

// TestComandosDelSitioExisten: cada orden de tramtai que enseñan los <pre> de index.html, servidor.html y
// desarrolladores.html, sus frases y las de la Ayuda (site/ayuda/*.html) existe, y las de los lang="en" existen con
// nombres en inglés. Lo que no está dentro de ningún lang se lee como español (se enseña en los
// dos idiomas, así que las Tareas 15 y 16 lo marcan por idioma). Solo lee archivos y llama a Find: no ejecuta nada.
func TestComandosDelSitioExisten(t *testing.T) {
	raiz := nuevaRaiz()
	type linea struct {
		fuente, texto   string
		ingles, enLinea bool
	}
	var lineas []linea
	for _, archivo := range []string{"index.html", "servidor.html", "desarrolladores.html"} {
		datos, err := os.ReadFile("../../site/" + archivo)
		if err != nil {
			t.Fatal(err)
		}
		antes := len(lineas)
		for _, pre := range preSitio.FindAllStringSubmatch(string(datos), -1) {
			es, en := lineasDePre(pre[1])
			for _, l := range es {
				lineas = append(lineas, linea{archivo, l, false, false})
			}
			for _, l := range en {
				lineas = append(lineas, linea{archivo, l, true, false})
			}
		}
		// También las órdenes citadas en una frase (<code> fuera de los <pre>), en el idioma de su bloque. En
		// desarrolladores.html no: las de sus frases (tramtai espacios, global token --rotar, agentes conectar…) aún no
		// tienen alias en inglés.
		if archivo != "desarrolladores.html" {
			for _, c := range codigosEnLinea(string(datos)) {
				lineas = append(lineas, linea{archivo + " (en una frase)", c.texto, c.lang == "en", true})
			}
		}
		if len(lineas) == antes {
			t.Fatalf("%s: no hay ningún <pre>: ¿cambió el marcado?", archivo)
		}
	}
	// Y las de la Ayuda (site/ayuda/*.html), que las cita entre comillas («…» o “…”) dentro de su frase en cada idioma.
	ayudas, err := filepath.Glob("../../site/ayuda/*.html")
	if err != nil || len(ayudas) == 0 {
		t.Fatalf("no se leyó site/ayuda: %v", err)
	}
	for _, archivo := range ayudas {
		datos, err := os.ReadFile(archivo)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range citasDeAyuda(string(datos)) {
			lineas = append(lineas, linea{"ayuda/" + filepath.Base(archivo), c.texto, c.lang == "en", true})
		}
	}
	revisadas, enFrases, enAyuda := 0, 0, 0
	for _, l := range lineas {
		args, servidor, ok := ordenDeLinea(l.texto)
		if !ok {
			continue
		}
		ingles := l.ingles
		if strings.HasPrefix(l.fuente, "ayuda/") {
			enAyuda++
			ingles = ingles && !(len(args) > 0 && sinAliasIngles[args[0]])
		} else if l.enLinea {
			enFrases++
		} else {
			revisadas++
		}
		if err := comprobarOrdenSitio(raiz, args, servidor, ingles); err != nil {
			t.Errorf("%s: %q (inglés %v): %v", l.fuente, strings.TrimSpace(html.UnescapeString(l.texto)), l.ingles, err)
		}
	}
	// Hoy son 26: 11 en index.html (tramtai terminal, igual en los dos idiomas, cuenta una vez), 12 en servidor.html y 3
	// en desarrolladores.html. Perder los bloques de una página baja de aquí.
	if revisadas < 26 {
		t.Errorf("solo se revisaron %d órdenes (mínimo 26): ¿cambió el marcado de los <pre>?", revisadas)
	}
	t.Logf("órdenes revisadas: %d en <pre>, %d en frases y %d en la Ayuda", revisadas, enFrases, enAyuda)
	// Las de las frases (tramtai-servidor estado, registro, actualizar…), hoy 38 entre los dos idiomas.
	if enFrases < 36 {
		t.Errorf("solo se revisaron %d órdenes en frases (mínimo 36): ¿cambió el marcado de los <code>?", enFrases)
	}
	if enAyuda < minimoAyuda {
		t.Errorf("solo se revisaron %d órdenes en la Ayuda (mínimo %d): ¿cambiaron sus comillas o sus <span lang>?", enAyuda, minimoAyuda)
	}
	for nombre := range sinAliasIngles {
		if c, _, err := raiz.Find([]string{nombre}); err != nil || len(c.Aliases) > 0 {
			t.Errorf("«tramtai %s» ya tiene alias en inglés (o no existe): quítalo de sinAliasIngles", nombre)
		}
	}
}

// sinAliasIngles: órdenes de primer nivel que aún no tienen nombre en inglés; la Ayuda en inglés las cita en español.
// shortcut: solo se comprueba que existan; cuando tengan alias, se quitan de aquí y la Ayuda en inglés pasa a usarlo.
var sinAliasIngles = map[string]bool{"actualizar": true, "conversor": true, "espacio": true, "espacios": true, "global": true, "proyecto": true}

// minimoAyuda: las órdenes que cita la Ayuda entre los dos idiomas (hoy 106), con algo de margen.
const minimoAyuda = 100

var (
	spanAyuda = regexp.MustCompile(`(?s)<span lang="(es|en)">(.*?)</span>`)
	citaAyuda = regexp.MustCompile(`[«“]([^«»“”]*)[»”]`)
)

// citasDeAyuda saca de una página de la Ayuda lo citado entre comillas en cada <span lang>, con su idioma: las órdenes
// que la Ayuda pone en una frase. ordenDeLinea decide después cuáles son órdenes de Tramtai.
func citasDeAyuda(pagina string) (citas []struct{ texto, lang string }) {
	for _, s := range spanAyuda.FindAllStringSubmatch(pagina, -1) {
		texto := html.UnescapeString(etiquetaSitio.ReplaceAllString(s[2], ""))
		for _, c := range citaAyuda.FindAllStringSubmatch(texto, -1) {
			citas = append(citas, struct{ texto, lang string }{c[1], s[1]})
		}
	}
	return citas
}

// TestCitasDeAyuda: cada cita lleva el idioma de su <span>, sin etiquetas y con las entidades resueltas.
func TestCitasDeAyuda(t *testing.T) {
	pagina := `<li><span lang="es">«tramtai agentes entrar codex» y «Instalar»</span><span lang="en">“tramtai agents login <a>codex</a>” &amp; “x &lt;id&gt;”</span></li>`
	var r []string
	for _, c := range citasDeAyuda(pagina) {
		r = append(r, c.lang+":"+c.texto)
	}
	quiere := []string{"es:tramtai agentes entrar codex", "es:Instalar", "en:tramtai agents login codex", "en:x <id>"}
	if !slices.Equal(r, quiere) {
		t.Errorf("%q, quería %q", r, quiere)
	}
}

// TestCodigosEnLineaSitio: el idioma sale del bloque que envuelve al <code>, y los <code> de un <pre> no cuentan.
func TestCodigosEnLineaSitio(t *testing.T) {
	pagina := `<div lang="en"><p>Run <code>tramtai-servidor open</code>.</p><img src="x"></div>` +
		`<p><span lang="es"><code>tramtai-servidor abrir</code></span></p><pre><code>tramtai x</code></pre><code>&amp;</code>`
	var r []string
	for _, c := range codigosEnLinea(pagina) {
		r = append(r, c.lang+"|"+c.texto)
	}
	if got := strings.Join(r, ","); got != "en|tramtai-servidor open,es|tramtai-servidor abrir,|&" {
		t.Errorf("codigosEnLinea: %q", got)
	}
}

// TestOrdenDeLineaSitio: sudo, ./ y .\ no cuentan, un # o un … corta, una \ final o un <hueco> no es un argumento y lo que
// no es de Tramtai se ignora.
func TestOrdenDeLineaSitio(t *testing.T) {
	for linea, esperada := range map[string]string{
		`sudo ./tramtai server install`:               "tramtai|server install",
		`.\tramtai.exe tools enable --all \`:          "tramtai|tools enable --all",
		`sudo tramtai-servidor admin password # ok`:   "servidor|admin password",
		`brew install tramtai`:                        "",
		`# tramtai terminal`:                          "",
		`tramtai-servidor agentes … y más`:            "servidor|agentes",
		`tramtai agentes suscripcion <id> --personal`: "tramtai|agentes suscripcion --personal",
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
