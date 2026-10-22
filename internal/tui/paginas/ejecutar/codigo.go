package ejecutar

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/datos"
)

// Ejemplo es uno de «Ver como código»: curl, python o node (JavaScript), con su título y el código para copiar.
type Ejemplo struct{ ID, Titulo, Codigo string }

// marcaCargas marca en el cuerpo dónde van los identificadores de las cargas subidas en el paso anterior (CARGAS de
// ejemplos.ts).
const marcaCargas = "__CARGAS__"

// cuerpoCodigo es la solicitud tal como la escribe el código: sin salida ni ejecución vacías y, si hay archivos, con la
// marca de las cargas. Las claves salen en el orden de nucleo.SolicitudEjecucion.
type cuerpoCodigo struct {
	Agente        string                  `json:"agente,omitempty"`
	Modelo        string                  `json:"modelo,omitempty"`
	PerfilModelo  string                  `json:"perfil_modelo,omitempty"`
	Esfuerzo      string                  `json:"esfuerzo,omitempty"`
	Respaldo      []string                `json:"respaldo,omitempty"`
	Perfil        string                  `json:"perfil,omitempty"`
	Instrucciones string                  `json:"instrucciones,omitempty"`
	Mensajes      []nucleo.Mensaje        `json:"mensajes,omitempty"`
	Cargas        string                  `json:"cargas,omitempty"`
	Salida        *nucleo.ConfigSalida    `json:"salida,omitempty"`
	Ejecucion     *nucleo.ConfigEjecucion `json:"ejecucion,omitempty"`
}

// Ejemplos es la prueba como código por la API nativa, como curl-nativa, python-nativa y node-nativa de ejemplos.ts. Las
// cargas de s son los nombres de los archivos que se adjuntan (no sus identificadores): el código los sube primero. La
// clave va tal cual; Probar pasa el marcador «[tu clave]» y nunca una clave de verdad.
func Ejemplos(url, clave string, s nucleo.SolicitudEjecucion) []Ejemplo {
	lengua := idioma.Activo
	archivos := s.Cargas
	c := cuerpoCodigo{Agente: s.Agente, Modelo: s.Modelo, PerfilModelo: s.PerfilModelo, Esfuerzo: s.Esfuerzo, Respaldo: s.Respaldo,
		Perfil: s.Perfil, Instrucciones: s.Instrucciones, Mensajes: s.Mensajes}
	if len(archivos) > 0 {
		c.Cargas = marcaCargas
	}
	if s.Salida.Formato != "" || len(s.Salida.Esquema) > 0 || s.Salida.Reintentos > 0 || s.Salida.ConservarSalidaInvalida {
		c.Salida = &s.Salida
	}
	if s.Ejecucion != (nucleo.ConfigEjecucion{}) {
		c.Ejecucion = &s.Ejecucion
	}
	nativa := datos.Texto(lengua, "ejemplos.nativa")
	return []Ejemplo{
		{ID: "curl", Titulo: "curl · " + nativa, Codigo: curlNativa(lengua, url, clave, c, archivos)},
		{ID: "python", Titulo: "Python · " + nativa, Codigo: pythonNativa(lengua, url, clave, c, archivos)},
		{ID: "node", Titulo: "JavaScript · " + nativa, Codigo: nodeNativa(lengua, url, clave, c, archivos)},
	}
}

// jsonDe es JSON.stringify(v, null, sangria): sin escapar <, > ni &, como JavaScript.
func jsonDe(v any, sangria string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", sangria)
	_ = enc.Encode(v)
	return strings.TrimSuffix(b.String(), "\n")
}

// sh pone un texto entre comillas simples de la shell.
func sh(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// pasos numera los comentarios: sin archivos no hay paso de subida.
func pasos(lengua string) func(clave string) string {
	n := 0
	return func(clave string) string {
		n++
		return strconv.Itoa(n) + ". " + datos.Texto(lengua, clave)
	}
}

func curlNativa(lengua, url, k string, c cuerpoCodigo, archivos []string) string {
	paso := pasos(lengua)
	auth := `-H "Authorization: Bearer ` + k + `"`
	var lineas []string
	if len(archivos) > 0 {
		nombres := make([]string, len(archivos))
		for i, a := range archivos {
			nombres[i] = sh(a)
		}
		lineas = append(lineas, "# "+paso("ejemplos.subir")+" ("+datos.Texto(lengua, "ejemplos.jq")+")", `CARGAS=""`,
			"for ARCHIVO in "+strings.Join(nombres, " ")+"; do",
			"  ID=$(curl -s "+url+"/api/v1/cargas "+auth+` -F "archivo=@\"$ARCHIVO\"" | jq -r .identificador_carga)`,
			`  CARGAS="$CARGAS${CARGAS:+,}\"$ID\""`, "done", "")
	}
	cuerpo := strings.Replace(sh(jsonDe(c, "  ")), `"`+marcaCargas+`"`, `['"$CARGAS"']`, 1)
	lineas = append(lineas, "# "+paso("ejemplos.crear"),
		"TRABAJO=$(curl -s "+url+"/api/v1/trabajos "+auth+` \`,
		`  -H "Content-Type: application/json" -H "Idempotency-Key: $(date +%s)" \`,
		"  -d "+cuerpo+" | jq -r .id)", "",
		"# "+paso("ejemplos.seguirCurl"), "curl -sN "+url+"/api/v1/trabajos/$TRABAJO/eventos "+auth, "",
		"# "+paso("ejemplos.resultado"), "curl -s "+url+"/api/v1/trabajos/$TRABAJO/salida "+auth)
	return strings.Join(lineas, "\n")
}

func pythonNativa(lengua, url, k string, c cuerpoCodigo, archivos []string) string {
	paso := pasos(lengua)
	lineas := []string{"import json, sys, uuid", "import requests", "", `BASE = "` + url + `/api/v1"`,
		`CABECERAS = {"Authorization": "Bearer ` + k + `"}`, ""}
	if len(archivos) > 0 {
		lineas = append(lineas, "# "+paso("ejemplos.subir"), "cargas = []", "for nombre in "+jsonDe(archivos, "")+":",
			`    with open(nombre, "rb") as f:`, `        r = requests.post(f"{BASE}/cargas", headers=CABECERAS, files={"archivo": f})`,
			"    if not r.ok:", "        sys.exit(r.text)", `    cargas.append(r.json()["identificador_carga"])`, "")
	}
	lineas = append(lineas, "# "+paso("ejemplos.crear"), "solicitud = "+strings.Replace(py(jsonDe(c, "")), `"`+marcaCargas+`"`, "cargas", 1),
		`r = requests.post(f"{BASE}/trabajos", headers={**CABECERAS, "Idempotency-Key": str(uuid.uuid4())}, json=solicitud)`,
		"if not r.ok:", "    sys.exit(r.text)", `trabajo = r.json()["id"]`, "",
		"# "+paso("ejemplos.seguir"), `fin, evento, datos = {}, "", []`,
		`with requests.get(f"{BASE}/trabajos/{trabajo}/eventos", headers=CABECERAS, stream=True) as eventos:`,
		`    for linea in (l.decode("utf-8") for l in eventos.iter_lines()):`,
		`        if linea.startswith("event: "):`, `            evento = linea[7:]`,
		`        elif linea.startswith("data: "):`, `            datos.append(linea[6:])`,
		`        elif linea == "" and evento:`, `            if evento == "texto":`, `                print("\n".join(datos), end="", flush=True)`,
		`            elif evento == "fin":`, `                fin = json.loads("\n".join(datos))`, `                break`,
		`            evento, datos = "", []`, `if fin.get("estado") != "completado":`, `    sys.exit(f"\n{fin}")`, "",
		"# "+paso("ejemplos.resultado"),
		`print("\n" + requests.get(f"{BASE}/trabajos/{trabajo}/salida", headers=CABECERAS).content.decode("utf-8"))`)
	return strings.Join(lineas, "\n")
}

func nodeNativa(lengua, url, k string, c cuerpoCodigo, archivos []string) string {
	paso := pasos(lengua)
	var lineas []string
	if len(archivos) > 0 {
		lineas = append(lineas, "import { readFile } from 'node:fs/promises'", "import { basename } from 'node:path'")
	}
	lineas = append(lineas, "import { randomUUID } from 'node:crypto'", "", "const BASE = '"+url+"/api/v1'",
		"const cabeceras = { Authorization: 'Bearer "+k+"' }",
		"async function pedir(ruta, opciones = {}) {", "  const r = await fetch(BASE + ruta, { ...opciones, headers: { ...cabeceras, ...opciones.headers } })",
		"  if (!r.ok) throw new Error(await r.text())", "  return r", "}", "")
	if len(archivos) > 0 {
		lineas = append(lineas, "// "+paso("ejemplos.subir"), "const cargas = []", "for (const nombre of "+jsonDe(archivos, "")+") {",
			"  const datos = new FormData()", "  datos.append('archivo', new Blob([await readFile(nombre)]), basename(nombre))",
			"  cargas.push((await (await pedir('/cargas', { method: 'POST', body: datos })).json()).identificador_carga)", "}", "")
	}
	lineas = append(lineas, "// "+paso("ejemplos.crear"), "const solicitud = "+strings.Replace(jsonDe(c, "  "), `"`+marcaCargas+`"`, "cargas", 1),
		"const { id } = await (await pedir('/trabajos', { method: 'POST', body: JSON.stringify(solicitud),",
		"  headers: { 'Content-Type': 'application/json', 'Idempotency-Key': randomUUID() } })).json()", "",
		"// "+paso("ejemplos.seguir"), "const eventos = await pedir(`/trabajos/${id}/eventos`)",
		"const lector = new TextDecoder()", "let pendiente = '', fin", "for await (const trozo of eventos.body) {",
		"  pendiente += lector.decode(trozo, { stream: true })", "  let corte", `  while ((corte = pendiente.indexOf('\n\n')) >= 0) {`,
		`    const bloque = pendiente.slice(0, corte).split('\n')`, "    pendiente = pendiente.slice(corte + 2)",
		"    const evento = bloque.find((l) => l.startsWith('event: '))?.slice(7)",
		`    const datos = bloque.filter((l) => l.startsWith('data: ')).map((l) => l.slice(6)).join('\n')`,
		"    if (evento === 'texto') process.stdout.write(datos)", "    if (evento === 'fin') fin = JSON.parse(datos)", "  }", "}",
		"if (fin?.estado !== 'completado') throw new Error(JSON.stringify(fin))", "",
		"// "+paso("ejemplos.resultado"), "console.log('\\n' + await (await pedir(`/trabajos/${id}/salida`)).text())")
	return strings.Join(lineas, "\n")
}

// py escribe un JSON como literal de Python con la sangría de py() en ejemplos.ts: cada elemento en su línea, con coma al
// final, y las claves en el orden en que vienen.
func py(texto string) string {
	d := json.NewDecoder(strings.NewReader(texto))
	var b strings.Builder
	pyValor(d, &b, "")
	return b.String()
}

func pyValor(d *json.Decoder, b *strings.Builder, sangria string) {
	tok, err := d.Token()
	if err != nil {
		return
	}
	mas := sangria + "    "
	switch v := tok.(type) {
	case json.Delim:
		abre, cierra := string(v), "]"
		if v == '{' {
			cierra = "}"
		}
		if !d.More() {
			d.Token()
			b.WriteString(abre + cierra)
			return
		}
		b.WriteString(abre + "\n")
		for d.More() {
			b.WriteString(mas)
			if v == '{' {
				clave, _ := d.Token()
				b.WriteString(jsonDe(clave, "") + ": ")
			}
			pyValor(d, b, mas)
			b.WriteString(",\n")
		}
		d.Token()
		b.WriteString(sangria + cierra)
	case nil:
		b.WriteString("None")
	case bool:
		b.WriteString(map[bool]string{true: "True", false: "False"}[v])
	default:
		b.WriteString(jsonDe(v, ""))
	}
}
