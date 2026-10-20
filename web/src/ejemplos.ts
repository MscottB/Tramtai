// ejemplos genera el código para copiar de Mis APIs (los ejemplos de cada contrato) y de Probar («Ver como código»).
import type { Solicitud } from './api/tipos'
import { aplicarModelo } from './recetas'
import { t } from './i18n'
import type { Clave } from './idiomas/es'

export interface Ejemplo {
  id: string
  titulo: () => string
  codigo: (clave: string) => string
  // nota dice lo que la petición pide y este código no puede llevar (por ejemplo, archivos por OpenAI).
  nota?: string
}

// Prueba es una petición ya armada como la envía Probar, con los nombres de sus archivos en lugar de sus cargas.
export interface Prueba {
  solicitud: Solicitud
  archivos: string[]
}

// CARGAS marca en el cuerpo dónde van los identificadores de las cargas subidas en el paso anterior.
const CARGAS = '__CARGAS__'

// py escribe un valor JSON como literal de Python (un texto JSON ya es un literal válido de Python).
function py(v: unknown, sangria = ''): string {
  if (v === null || v === undefined) return 'None'
  if (typeof v === 'boolean') return v ? 'True' : 'False'
  if (typeof v !== 'object') return JSON.stringify(v)
  const mas = sangria + '    '
  if (Array.isArray(v)) return v.length ? `[\n${v.map((x) => mas + py(x, mas)).join(',\n')},\n${sangria}]` : '[]'
  const campos = Object.entries(v).filter(([, x]) => x !== undefined)
  return campos.length ? `{\n${campos.map(([k, x]) => `${mas}${JSON.stringify(k)}: ${py(x, mas)}`).join(',\n')},\n${sangria}}` : '{}'
}
// sh pone un texto entre comillas simples de la shell.
const sh = (s: string) => `'${s.replace(/'/g, `'\\''`)}'`

function cuerpo(p: Prueba): Solicitud {
  return { ...p.solicitud, cargas: p.archivos.length ? (CARGAS as unknown as string[]) : undefined }
}
// pasos numera los comentarios: sin archivos no hay paso de subida.
const pasos = () => {
  let n = 0
  return (clave: Clave) => `${++n}. ${t(clave)}`
}

function curlNativa(url: string, p: Prueba, k: string): string {
  const paso = pasos()
  const auth = `-H "Authorization: Bearer ${k}"`
  const lineas: string[] = []
  if (p.archivos.length) {
    lineas.push(`# ${paso('ejemplos.subir')} (${t('ejemplos.jq')})`, 'CARGAS=""',
      `for ARCHIVO in ${p.archivos.map(sh).join(' ')}; do`,
      // Entre comillas para curl: un nombre con «;» o «,» no se toma como opción del campo.
      `  ID=$(curl -s ${url}/api/v1/cargas ${auth} -F "archivo=@\\"$ARCHIVO\\"" | jq -r .identificador_carga)`,
      '  CARGAS="$CARGAS${CARGAS:+,}\\"$ID\\""', 'done', '')
  }
  // La lista de cargas sale de la variable: se cierra la comilla simple, se expande "$CARGAS" y se vuelve a abrir.
  const json = sh(JSON.stringify(cuerpo(p), null, 2)).replace(`"${CARGAS}"`, `['"$CARGAS"']`)
  lineas.push(`# ${paso('ejemplos.crear')}`,
    `TRABAJO=$(curl -s ${url}/api/v1/trabajos ${auth} \\`,
    '  -H "Content-Type: application/json" -H "Idempotency-Key: $(date +%s)" \\',
    `  -d ${json} | jq -r .id)`, '',
    `# ${paso('ejemplos.seguirCurl')}`, `curl -sN ${url}/api/v1/trabajos/$TRABAJO/eventos ${auth}`, '',
    `# ${paso('ejemplos.resultado')}`, `curl -s ${url}/api/v1/trabajos/$TRABAJO/salida ${auth}`)
  return lineas.join('\n')
}

function pythonNativa(url: string, p: Prueba, k: string): string {
  const paso = pasos()
  const lineas = ['import json, sys, uuid', 'import requests', '', `BASE = "${url}/api/v1"`, `CABECERAS = {"Authorization": "Bearer ${k}"}`, '']
  if (p.archivos.length) {
    lineas.push(`# ${paso('ejemplos.subir')}`, 'cargas = []', `for nombre in ${JSON.stringify(p.archivos)}:`,
      '    with open(nombre, "rb") as f:', '        r = requests.post(f"{BASE}/cargas", headers=CABECERAS, files={"archivo": f})',
      '    if not r.ok:', '        sys.exit(r.text)', '    cargas.append(r.json()["identificador_carga"])', '')
  }
  lineas.push(`# ${paso('ejemplos.crear')}`, `solicitud = ${py(cuerpo(p)).replace(`"${CARGAS}"`, 'cargas')}`,
    'r = requests.post(f"{BASE}/trabajos", headers={**CABECERAS, "Idempotency-Key": str(uuid.uuid4())}, json=solicitud)',
    'if not r.ok:', '    sys.exit(r.text)', 'trabajo = r.json()["id"]', '',
    `# ${paso('ejemplos.seguir')}`, 'fin, evento, datos = {}, "", []',
    'with requests.get(f"{BASE}/trabajos/{trabajo}/eventos", headers=CABECERAS, stream=True) as eventos:',
    '    for linea in (l.decode("utf-8") for l in eventos.iter_lines()):',
    '        if linea.startswith("event: "):', '            evento = linea[7:]',
    '        elif linea.startswith("data: "):', '            datos.append(linea[6:])',
    '        elif linea == "" and evento:', '            if evento == "texto":', '                print("\\n".join(datos), end="", flush=True)',
    '            elif evento == "fin":', '                fin = json.loads("\\n".join(datos))', '                break',
    '            evento, datos = "", []', 'if fin.get("estado") != "completado":', '    sys.exit(f"\\n{fin}")', '',
    `# ${paso('ejemplos.resultado')}`,
    'print("\\n" + requests.get(f"{BASE}/trabajos/{trabajo}/salida", headers=CABECERAS).content.decode("utf-8"))')
  return lineas.join('\n')
}

function nodeNativa(url: string, p: Prueba, k: string): string {
  const paso = pasos()
  const lineas = [...(p.archivos.length ? ["import { readFile } from 'node:fs/promises'", "import { basename } from 'node:path'"] : []),
    "import { randomUUID } from 'node:crypto'", '', `const BASE = '${url}/api/v1'`, `const cabeceras = { Authorization: 'Bearer ${k}' }`,
    'async function pedir(ruta, opciones = {}) {', '  const r = await fetch(BASE + ruta, { ...opciones, headers: { ...cabeceras, ...opciones.headers } })',
    '  if (!r.ok) throw new Error(await r.text())', '  return r', '}', '']
  if (p.archivos.length) {
    lineas.push(`// ${paso('ejemplos.subir')}`, 'const cargas = []', `for (const nombre of ${JSON.stringify(p.archivos)}) {`,
      '  const datos = new FormData()', "  datos.append('archivo', new Blob([await readFile(nombre)]), basename(nombre))",
      "  cargas.push((await (await pedir('/cargas', { method: 'POST', body: datos })).json()).identificador_carga)", '}', '')
  }
  lineas.push(`// ${paso('ejemplos.crear')}`, `const solicitud = ${JSON.stringify(cuerpo(p), null, 2).replace(`"${CARGAS}"`, 'cargas')}`,
    "const { id } = await (await pedir('/trabajos', { method: 'POST', body: JSON.stringify(solicitud),",
    "  headers: { 'Content-Type': 'application/json', 'Idempotency-Key': randomUUID() } })).json()", '',
    `// ${paso('ejemplos.seguir')}`, 'const eventos = await pedir(`/trabajos/${id}/eventos`)',
    'const lector = new TextDecoder()', "let pendiente = '', fin", 'for await (const trozo of eventos.body) {',
    '  pendiente += lector.decode(trozo, { stream: true })', '  let corte', "  while ((corte = pendiente.indexOf('\\n\\n')) >= 0) {",
    "    const bloque = pendiente.slice(0, corte).split('\\n')", '    pendiente = pendiente.slice(corte + 2)',
    "    const evento = bloque.find((l) => l.startsWith('event: '))?.slice(7)",
    "    const datos = bloque.filter((l) => l.startsWith('data: ')).map((l) => l.slice(6)).join('\\n')",
    "    if (evento === 'texto') process.stdout.write(datos)", "    if (evento === 'fin') fin = JSON.parse(datos)", '  }', '}',
    "if (fin?.estado !== 'completado') throw new Error(JSON.stringify(fin))", '',
    `// ${paso('ejemplos.resultado')}`, "console.log('\\n' + await (await pedir(`/trabajos/${id}/salida`)).text())")
  return lineas.join('\n')
}

function goNativa(url: string, p: Prueba, k: string): string {
  const paso = pasos()
  const conArchivos = p.archivos.length > 0
  const importar = ['bufio', 'encoding/json', 'fmt', 'io', 'log', 'net/http', 'strings', ...(conArchivos ? ['bytes', 'mime/multipart', 'os', 'path/filepath'] : [])].sort()
  const lineas = ['package main', '', 'import (', ...importar.map((i) => `\t"${i}"`), ')', '', `const base = "${url}/api/v1"`, '',
    '// pedir hace una petición con la clave y falla si la respuesta no es 2xx.',
    'func pedir(metodo, ruta, tipo string, cuerpo io.Reader) *http.Response {',
    '\tpeticion, err := http.NewRequest(metodo, base+ruta, cuerpo)', '\tif err != nil {', '\t\tlog.Fatal(err)', '\t}',
    `\tpeticion.Header.Set("Authorization", "Bearer ${k}")`, '\tif tipo != "" {', '\t\tpeticion.Header.Set("Content-Type", tipo)', '\t}',
    '\trespuesta, err := http.DefaultClient.Do(peticion)', '\tif err != nil {', '\t\tlog.Fatal(err)', '\t}',
    '\tif respuesta.StatusCode >= 300 {', '\t\tdatos, _ := io.ReadAll(respuesta.Body)', '\t\tlog.Fatalf("%s: %s", respuesta.Status, datos)', '\t}',
    '\treturn respuesta', '}', '', 'func main() {']
  if (conArchivos) {
    lineas.push(`\t// ${paso('ejemplos.subir')}`, '\tvar cargas []string', `\tfor _, nombre := range []string{${p.archivos.map((a) => JSON.stringify(a)).join(', ')}} {`,
      '\t\tvar cuerpo bytes.Buffer', '\t\tformulario := multipart.NewWriter(&cuerpo)', '\t\tparte, _ := formulario.CreateFormFile("archivo", filepath.Base(nombre))',
      '\t\tdatos, err := os.ReadFile(nombre)', '\t\tif err != nil {', '\t\t\tlog.Fatal(err)', '\t\t}', '\t\tparte.Write(datos)', '\t\tformulario.Close()',
      '\t\tvar carga struct {', '\t\t\tID string `json:"identificador_carga"`', '\t\t}',
      '\t\tjson.NewDecoder(pedir("POST", "/cargas", formulario.FormDataContentType(), &cuerpo).Body).Decode(&carga)',
      '\t\tcargas = append(cargas, carga.ID)', '\t}', '')
  }
  // Un literal de Go con comillas (el texto JSON de una cadena lo es): un mensaje con comillas invertidas o % llega intacto.
  lineas.push(`\t// ${paso('ejemplos.crear')}`, `\tsolicitud := ${JSON.stringify(JSON.stringify(cuerpo(p)))}`)
  if (conArchivos) lineas.push('\tlista, _ := json.Marshal(cargas)', `\tsolicitud = strings.Replace(solicitud, ${JSON.stringify(`"${CARGAS}"`)}, string(lista), 1)`)
  lineas.push('\tvar trabajo struct {', '\t\tID string `json:"id"`', '\t}',
    '\tjson.NewDecoder(pedir("POST", "/trabajos", "application/json", strings.NewReader(solicitud)).Body).Decode(&trabajo)', '',
    `\t// ${paso('ejemplos.seguir')}`, '\teventos := bufio.NewScanner(pedir("GET", "/trabajos/"+trabajo.ID+"/eventos", "", nil).Body)',
    '\tevento := ""', '\tfor eventos.Scan() {', '\t\tlinea := eventos.Text()', '\t\tswitch {', '\t\tcase strings.HasPrefix(linea, "event: "):',
    '\t\t\tevento = linea[7:]', '\t\tcase strings.HasPrefix(linea, "data: ") && evento == "texto":', '\t\t\tfmt.Print(linea[6:])',
    '\t\tcase strings.HasPrefix(linea, "data: ") && evento == "fin":', '\t\t\tfmt.Println("\\n" + linea[6:])', '\t\t}', '\t}', '',
    `\t// ${paso('ejemplos.resultado')}`, '\tsalida, _ := io.ReadAll(pedir("GET", "/trabajos/"+trabajo.ID+"/salida", "", nil).Body)',
    '\tfmt.Println(string(salida))', '}')
  return lineas.join('\n')
}

// herramientasPython es el ciclo de herramientas por OpenAI de la guía del desarrollador (docs/ejemplos/python/herramientas.py)
// con la URL, la clave y el modelo de esta API.
function herramientasPython(url: string, modelo: string, k: string): string {
  return `import json

from openai import OpenAI

# ${t('ejemplos.herramientasComentario')}
herramientas = [{"type": "function", "function": {
    "name": "clima",
    "description": "El tiempo actual de una ciudad",
    "parameters": {"type": "object", "properties": {"ciudad": {"type": "string"}}, "required": ["ciudad"]},
}}]


def clima(ciudad):
    return f"18 °C y despejado en {ciudad}"


cliente = OpenAI(base_url="${url}/openai/v1", api_key="${k}")
mensajes = [{"role": "user", "content": "¿Qué tiempo hace en Santiago?"}]
while True:
    mensaje = cliente.chat.completions.create(model="${modelo}", messages=mensajes, tools=herramientas).choices[0].message
    if not mensaje.tool_calls:
        print(mensaje.content)
        break
    mensajes.append({"role": "assistant", "content": mensaje.content, "tool_calls": [
        {"id": l.id, "type": "function", "function": {"name": l.function.name, "arguments": l.function.arguments}}
        for l in mensaje.tool_calls
    ]})
    for llamada in mensaje.tool_calls:
        mensajes.append({"role": "tool", "tool_call_id": llamada.id, "content": clima(**json.loads(llamada.function.arguments))})`
}

const rolesOpenAI: Record<string, string> = { usuario: 'user', asistente: 'assistant', sistema: 'system' }

// tiposMIME dan el tipo de un archivo por su extensión, para mandarlo en base64 por OpenAI o Anthropic.
const tiposMIME: Record<string, string> = {
  '.pdf': 'application/pdf', '.docx': 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  '.pptx': 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
  '.xlsx': 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', '.html': 'text/html', '.htm': 'text/html',
  '.txt': 'text/plain', '.md': 'text/markdown', '.csv': 'text/csv', '.json': 'application/json', '.xml': 'application/xml',
}
const mimeDe = (nombre: string) => tiposMIME[nombre.slice(nombre.lastIndexOf('.')).toLowerCase()] ?? 'application/octet-stream'
// ficha marca en el cuerpo dónde va el archivo i; cada lenguaje la sustituye por su forma de leerlo en base64.
const ficha = (i: number) => `__ARCHIVO_${i}__`
const fichas = /"__ARCHIVO_(\d+)__"/

// conArchivos pone las fichas de los archivos en el último mensaje del usuario (o en uno nuevo), detrás de su texto.
function conArchivos(mensajes: { role: string; content: unknown }[], p: Prueba) {
  if (!p.archivos.length) return mensajes
  const partes = p.archivos.map((_, i) => ficha(i))
  const ultimo = mensajes.map((m) => m.role).lastIndexOf('user')
  if (ultimo < 0) return [...mensajes, { role: 'user', content: partes }]
  return mensajes.map((m, i) => (i === ultimo ? { ...m, content: [{ type: 'text', text: m.content }, ...partes] } : m))
}

// pythonConArchivos sustituye las fichas por la llamada que lee cada archivo (archivo() o documento()).
function pythonConArchivos(codigo: string, p: Prueba, funcion: string) {
  return codigo.replace(new RegExp(fichas, 'g'), (_, i: string) => `${funcion}(${JSON.stringify(p.archivos[+i])}, ${JSON.stringify(mimeDe(p.archivos[+i]))})`)
}

// curlConArchivos arma el cuerpo por partes y lo pasa por la entrada estándar: el base64 de un archivo grande no cabe
// como argumento de curl.
function curlConArchivos(json: string, p: Prueba, inicio: string): string {
  const trozos = json.split(fichas)
  const lineas = ['{']
  trozos.forEach((trozo, i) => {
    if (i % 2 === 0) {
      if (trozo) lineas.push(`  printf '%s' ${sh(trozo)}`)
      return
    }
    const nombre = p.archivos[+trozo]
    lineas.push(`  printf '%s' ${sh(`{"type": "file", "file": {"filename": ${JSON.stringify(nombre)}, "file_data": "data:${mimeDe(nombre)};base64,`)}`,
      `  base64 < ${sh(nombre)} | tr -d '\\n'`, `  printf '%s' '"}}'`)
  })
  lineas.push(`} | ${inicio} --data-binary @-`)
  return lineas.join('\n')
}
const esfuerzosOpenAI: Record<string, string> = { bajo: 'low', medio: 'medium', alto: 'high', muy_alto: 'xhigh', maximo: 'xhigh' }

// notaFachada dice lo que la prueba pide y solo admite la API propia (más lo que esa fachada no tiene).
function notaFachada(p: Prueba, ...extras: (string | false | undefined)[]) {
  const s = p.solicitud
  const faltan = [s.respaldo?.length && t('codigo.respaldo'), s.perfil && t('codigo.perfil'),
    s.ejecucion?.tiempo_limite_segundos && t('codigo.tiempo'), ...extras].filter(Boolean)
  return faltan.length ? t('codigo.soloNativa', { que: faltan.join(', ') }) : undefined
}

// chatOpenAI es la misma prueba por /openai/v1/chat/completions: instrucciones como mensaje de sistema, esfuerzo
// como reasoning_effort y el esquema como response_format.
function chatOpenAI(p: Prueba, modelo: string) {
  const s = p.solicitud
  const cuerpo: Record<string, unknown> = { model: modelo, messages: conArchivos([
    ...(s.instrucciones ? [{ role: 'system', content: s.instrucciones }] : []),
    ...(s.mensajes ?? []).map((m) => ({ role: rolesOpenAI[m.rol] ?? m.rol, content: m.contenido })),
  ], p) }
  if (s.esfuerzo) cuerpo.reasoning_effort = esfuerzosOpenAI[s.esfuerzo]
  if (s.salida?.esquema) cuerpo.response_format = { type: 'json_schema', json_schema: { name: 'salida', schema: s.salida.esquema } }
  return { cuerpo, nota: notaFachada(p, s.esfuerzo === 'maximo' && t('codigo.maximo')) }
}

// mensajesAnthropic es la misma prueba por /anthropic/v1/messages: las instrucciones (y los mensajes de sistema) van
// en system; esa API no admite esquema ni esfuerzo.
function mensajesAnthropic(p: Prueba, modelo: string) {
  const s = p.solicitud
  const mensajes = s.mensajes ?? []
  const sistema = [s.instrucciones, ...mensajes.filter((m) => m.rol === 'sistema').map((m) => m.contenido)].filter(Boolean).join('\n\n')
  const cuerpo: Record<string, unknown> = { model: modelo, max_tokens: 1024, ...(sistema ? { system: sistema } : {}),
    messages: conArchivos(mensajes.filter((m) => m.rol !== 'sistema').map((m) => ({ role: rolesOpenAI[m.rol] ?? m.rol, content: m.contenido })), p) }
  return { cuerpo, nota: notaFachada(p, !!s.salida?.esquema && t('codigo.esquema'), !!s.esfuerzo && t('codigo.esfuerzo')) }
}

// esquemaEjemplo es el de los ejemplos de Mis APIs: un JSON con un resumen.
const esquemaEjemplo = { type: 'object', properties: { resumen: { type: 'string' } }, required: ['resumen'] }

// ejemplos son los de lo que acepta la API (una que solo habla Anthropic no ofrece los de OpenAI); con prueba, el
// código de esa petición por la API propia, por OpenAI y por Anthropic. muestra sustituye la petición de ejemplo de la
// API propia: la de una API por aplicación solo lleva el mensaje y el archivo, porque su receta la aplica la API.
export function ejemplos(o: { url: string; modelo: string; contratos: string[]; prueba?: Prueba; muestra?: Prueba }): Ejemplo[] {
  const { url, modelo, contratos } = o
  const prueba = o.prueba ?? o.muestra ?? { archivos: ['informe.pdf'],
    solicitud: aplicarModelo({ mensajes: [{ rol: 'usuario', contenido: t('ejemplos.mensaje') }], salida: { formato: 'json', esquema: esquemaEjemplo } }, modelo) }
  const nativos: Ejemplo[] = contratos.includes('nativa') ? [
    { id: 'curl-nativa', titulo: () => `curl · ${t('ejemplos.nativa')}`, codigo: (k) => curlNativa(url, prueba, k) },
    { id: 'python-nativa', titulo: () => `Python · ${t('ejemplos.nativa')}`, codigo: (k) => pythonNativa(url, prueba, k) },
    { id: 'node-nativa', titulo: () => `Node · ${t('ejemplos.nativa')}`, codigo: (k) => nodeNativa(url, prueba, k) },
    { id: 'go-nativa', titulo: () => `Go · ${t('ejemplos.nativa')}`, codigo: (k) => goNativa(url, prueba, k) },
  ] : []
  if (o.prueba) {
    const lista = [...nativos]
    const argumentos = (cuerpo: Record<string, unknown>) => Object.entries(cuerpo).map(([c, v]) => `    ${c}=${py(v, '    ')},`).join('\n')
    if (contratos.includes('openai')) {
      const { cuerpo, nota } = chatOpenAI(o.prueba, modelo)
      const prueba = o.prueba
      const leerPy = prueba.archivos.length ? `import base64
` : ''
      const funcionPy = prueba.archivos.length ? `
# archivo lee un archivo y lo deja como parte del mensaje, en base64 (Tramtai lo convierte a texto para el agente).
def archivo(nombre, tipo):
    with open(nombre, "rb") as f:
        return {"type": "file", "file": {"filename": nombre, "file_data": f"data:{tipo};base64," + base64.b64encode(f.read()).decode()}}
` : ''
      lista.push({ id: 'python', titulo: () => 'Python · OpenAI', nota, codigo: (k) => `${leerPy}from openai import OpenAI
${funcionPy}
cliente = OpenAI(base_url="${url}/openai/v1", api_key="${k}")
r = cliente.chat.completions.create(
${pythonConArchivos(argumentos(cuerpo), prueba, 'archivo')}
)
print(r.choices[0].message.content)` },
      { id: 'curl', titulo: () => 'curl · OpenAI', nota, codigo: (k) => {
        const inicio = `curl ${url}/openai/v1/chat/completions \\
  -H "Authorization: Bearer ${k}" \\
  -H "Content-Type: application/json"`
        return prueba.archivos.length ? curlConArchivos(JSON.stringify(cuerpo, null, 2), prueba, inicio) : `${inicio} \\
  -d ${sh(JSON.stringify(cuerpo, null, 2))}`
      } })
    }
    if (contratos.includes('anthropic')) {
      const { cuerpo, nota } = mensajesAnthropic(o.prueba, modelo)
      const prueba = o.prueba
      const funcionPy = prueba.archivos.length ? `import base64
` : ''
      const documentoPy = prueba.archivos.length ? `
# documento lee un archivo y lo deja como bloque del mensaje, en base64 (Tramtai lo convierte a texto para el agente).
def documento(nombre, tipo):
    with open(nombre, "rb") as f:
        return {"type": "document", "title": nombre, "source": {"type": "base64", "media_type": tipo, "data": base64.b64encode(f.read()).decode()}}
` : ''
      lista.push({ id: 'anthropic', titulo: () => 'Python · Anthropic', nota, codigo: (k) => `${funcionPy}import anthropic
${documentoPy}
cliente = anthropic.Anthropic(base_url="${url}/anthropic", api_key="${k}")
m = cliente.messages.create(
${pythonConArchivos(argumentos(cuerpo), prueba, 'documento')}
)
print(m.content[0].text)` })
    }
    return lista
  }
  return [
    { id: 'python', titulo: () => 'Python · OpenAI', codigo: (k: string) => `from openai import OpenAI

cliente = OpenAI(base_url="${url}/openai/v1", api_key="${k}")
r = cliente.chat.completions.create(
    model="${modelo}",
    messages=[{"role": "user", "content": "${t('conectar.hola')}"}],
)
print(r.choices[0].message.content)` },
    { id: 'node', titulo: () => 'Node · OpenAI', codigo: (k: string) => `import OpenAI from 'openai'

const cliente = new OpenAI({ baseURL: '${url}/openai/v1', apiKey: '${k}' })
const r = await cliente.chat.completions.create({
  model: '${modelo}',
  messages: [{ role: 'user', content: '${t('conectar.hola')}' }],
})
console.log(r.choices[0].message.content)` },
    { id: 'anthropic', titulo: () => 'Python · Anthropic', codigo: (k: string) => `import anthropic

cliente = anthropic.Anthropic(base_url="${url}/anthropic", api_key="${k}")
m = cliente.messages.create(
    model="${modelo}", max_tokens=1024,
    messages=[{"role": "user", "content": "${t('conectar.hola')}"}],
)
print(m.content[0].text)` },
    { id: 'curl', titulo: () => 'curl', codigo: (k: string) => `curl ${url}/openai/v1/chat/completions \\
  -H "Authorization: Bearer ${k}" \\
  -H "Content-Type: application/json" \\
  -d '{"model": "${modelo}", "messages": [{"role": "user", "content": "${t('conectar.hola')}"}]}'` },
    { id: 'python-herramientas', titulo: () => `Python · ${t('ejemplos.herramientas')}`, codigo: (k: string) => herramientasPython(url, modelo, k) },
    { id: 'entorno', titulo: () => t('conectar.entorno'), codigo: (k: string) => `${t('conectar.entornoComentario')}
${[contratos.includes('openai') ? `OPENAI_BASE_URL=${url}/openai/v1\nOPENAI_API_KEY=${k}` : '',
    contratos.includes('anthropic') ? `ANTHROPIC_BASE_URL=${url}/anthropic\nANTHROPIC_API_KEY=${k}` : ''].filter(Boolean).join('\n')}` },
  ].filter((e) => (e.id === 'anthropic' ? contratos.includes('anthropic') : e.id === 'entorno'
    ? contratos.includes('openai') || contratos.includes('anthropic') : contratos.includes('openai'))).concat(nativos)
}
