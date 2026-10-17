// ejemplos genera el código para copiar de Mis APIs (los ejemplos de cada contrato) y de Probar («Ver como código»).
import { aplicarModelo, type Solicitud } from './api'
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
      `  ID=$(curl -s ${url}/api/v1/cargas ${auth} -F "archivo=@$ARCHIVO" | jq -r .identificador_carga)`,
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
    '    eventos.encoding = "utf-8"',
    '    for linea in eventos.iter_lines(decode_unicode=True):',
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

const rolesOpenAI: Record<string, string> = { usuario: 'user', asistente: 'assistant', sistema: 'system' }
const esfuerzosOpenAI: Record<string, string> = { bajo: 'low', medio: 'medium', alto: 'high', muy_alto: 'xhigh', maximo: 'xhigh' }

// chatOpenAI es la misma prueba por /openai/v1/chat/completions: instrucciones como mensaje de sistema, esfuerzo
// como reasoning_effort y el esquema como response_format; nota dice lo que solo admite la API propia.
function chatOpenAI(p: Prueba, modelo: string) {
  const s = p.solicitud
  const cuerpo: Record<string, unknown> = { model: modelo, messages: [
    ...(s.instrucciones ? [{ role: 'system', content: s.instrucciones }] : []),
    ...s.mensajes.map((m) => ({ role: rolesOpenAI[m.rol] ?? m.rol, content: m.contenido })),
  ] }
  if (s.esfuerzo) cuerpo.reasoning_effort = esfuerzosOpenAI[s.esfuerzo]
  if (s.salida?.esquema) cuerpo.response_format = { type: 'json_schema', json_schema: { name: 'salida', schema: s.salida.esquema } }
  const faltan = [p.archivos.length && t('codigo.archivos'), s.respaldo?.length && t('codigo.respaldo'), s.perfil && t('codigo.perfil'),
    s.ejecucion?.tiempo_limite_segundos && t('codigo.tiempo'), s.esfuerzo === 'maximo' && t('codigo.maximo')].filter(Boolean)
  return { cuerpo, nota: faltan.length ? t('codigo.soloNativa', { que: faltan.join(', ') }) : undefined }
}

// esquemaEjemplo es el de los ejemplos de Mis APIs: un JSON con un resumen.
const esquemaEjemplo = { type: 'object', properties: { resumen: { type: 'string' } }, required: ['resumen'] }

// ejemplos son los de lo que acepta la API (una que solo habla Anthropic no ofrece los de OpenAI); con prueba, el
// código de esa petición por la API propia y por OpenAI.
export function ejemplos(o: { url: string; modelo: string; contratos: string[]; prueba?: Prueba }): Ejemplo[] {
  const { url, modelo, contratos } = o
  const prueba = o.prueba ?? { archivos: ['informe.pdf'],
    solicitud: aplicarModelo({ mensajes: [{ rol: 'usuario', contenido: t('ejemplos.mensaje') }], salida: { formato: 'json', esquema: esquemaEjemplo } }, modelo) }
  const nativos: Ejemplo[] = contratos.includes('nativa') ? [
    { id: 'curl-nativa', titulo: () => `curl · ${t('ejemplos.nativa')}`, codigo: (k) => curlNativa(url, prueba, k) },
    { id: 'python-nativa', titulo: () => `Python · ${t('ejemplos.nativa')}`, codigo: (k) => pythonNativa(url, prueba, k) },
    { id: 'node-nativa', titulo: () => `Node · ${t('ejemplos.nativa')}`, codigo: (k) => nodeNativa(url, prueba, k) },
  ] : []
  if (o.prueba) {
    if (!contratos.includes('openai')) return nativos
    const { cuerpo, nota } = chatOpenAI(o.prueba, modelo)
    return [...nativos,
      { id: 'python', titulo: () => 'Python · OpenAI', nota, codigo: (k) => `from openai import OpenAI

cliente = OpenAI(base_url="${url}/openai/v1", api_key="${k}")
r = cliente.chat.completions.create(
${Object.entries(cuerpo).map(([c, v]) => `    ${c}=${py(v, '    ')},`).join('\n')}
)
print(r.choices[0].message.content)` },
      { id: 'curl', titulo: () => 'curl · OpenAI', nota, codigo: (k) => `curl ${url}/openai/v1/chat/completions \\
  -H "Authorization: Bearer ${k}" \\
  -H "Content-Type: application/json" \\
  -d ${sh(JSON.stringify(cuerpo, null, 2))}` },
    ]
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
    { id: 'entorno', titulo: () => t('conectar.entorno'), codigo: (k: string) => `${t('conectar.entornoComentario')}
${[contratos.includes('openai') ? `OPENAI_BASE_URL=${url}/openai/v1\nOPENAI_API_KEY=${k}` : '',
    contratos.includes('anthropic') ? `ANTHROPIC_BASE_URL=${url}/anthropic\nANTHROPIC_API_KEY=${k}` : ''].filter(Boolean).join('\n')}` },
  ].filter((e) => (e.id === 'anthropic' ? contratos.includes('anthropic') : e.id === 'entorno'
    ? contratos.includes('openai') || contratos.includes('anthropic') : contratos.includes('openai'))).concat(nativos)
}
