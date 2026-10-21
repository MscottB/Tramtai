// terminal-datos convierte las fuentes de la Web UI en los datos que la terminal en Go incrusta (internal/tui/datos): la
// Ayuda en texto plano, las plantillas de salida estructurada, los nombres del menú y los ejemplos de «Ver como código»
// como datos de prueba. Son funciones puras: terminal-datos.mjs les da los textos y escribe los archivos. El CI lo vuelve
// a correr y falla si lo guardado en el repo no coincide.
import type { Bloque as BloqueWeb, Tema as TemaWeb } from '../src/ayuda/tipos'
import type { Plantilla } from '../src/plantillas'
import { CLAVES_UI } from '../src/ayuda/indice'
import { ERRORES } from '../src/ayuda/temas/problemas'
import { aEsquema } from '../src/esquemaCampos'
import { URL_SITIO } from './sitio'

type Textos = Record<string, string>
type Idioma = 'es' | 'en'
const IDIOMAS: Idioma[] = ['es', 'en']

// Bloque es la forma que lee Go (internal/tui/datos): todo texto plano. En tabla, lineas son las columnas y filas llevan
// el título de la fila y sus celdas; en errores, preguntas y pantalla, pares son (título o pregunta, texto o respuesta).
export interface Bloque {
  tipo: 'texto' | 'pasos' | 'tabla' | 'preguntas' | 'errores' | 'pantalla' | 'web'
  titulo?: string
  lineas?: string[]
  filas?: string[][]
  pares?: [string, string][]
  url?: string
}
export interface Tema { id: string; titulo: string; pagina?: string; resumen: string; bloques: Bloque[] }

export const URL_AYUDA = URL_SITIO + 'ayuda/'

// CLAVES_TEXTOS son, además de las menu.*, los textos que la terminal toma de la Web UI: los de la Ayuda de la interfaz.
export const CLAVES_TEXTOS: string[] = [...CLAVES_UI]

// plano deja un texto sin marcas: sin ** y con los <marcadores> entre corchetes.
const plano = (s: string) => s.replace(/\*\*/g, '').replace(/<([^<>]*)>/g, '[$1]')

function bloqueDe(b: BloqueWeb, tx: (k: string) => string, url: string): Bloque {
  switch (b.tipo) {
    case 'nota': return { tipo: 'texto', lineas: [tx(b.texto)] }
    case 'pasos': return { tipo: 'pasos', lineas: b.pasos.map(tx) }
    case 'tabla': return { tipo: 'tabla', lineas: b.columnas.map(tx), filas: b.filas.map((f) => [tx(f.titulo), ...f.celdas.map(tx)]) }
    case 'preguntas': return { tipo: 'preguntas', pares: b.preguntas.map((p) => [tx(p.pregunta), tx(p.respuesta)]) }
    case 'errores':
      return { tipo: 'errores', pares: b.codigos.map((c) => [tx(ERRORES[c]?.titulo ?? 'error.' + c) + (c === 'sin_conexion' ? '' : ` (${c})`),
        ERRORES[c] ? tx(ERRORES[c].solucion) : '']) }
    case 'recorrido': return { tipo: 'pantalla', pares: b.pasos.map((p) => [tx(p.titulo), tx(p.texto)]) }
    case 'captura': return { tipo: 'web', titulo: tx(b.alt), url }
    // El diagrama conserva sus paradas como líneas: la terminal no lo dibuja, pero el texto no se pierde.
    case 'diagrama':
      return { tipo: 'web', url, lineas: [...b.paradas.map((p, i) => `${i + 1}. ${tx(p.titulo)}${p.nota ? ' · ' + tx(p.nota) : ''}`),
        ...(b.ramas ?? []).map((r) => `${r.desde + 1} → ${r.hacia + 1}: ${tx(r.etiqueta)}`)] }
    case 'ejemplo':
    case 'ir':
    case 'descargas': return { tipo: 'web', url }
  }
}

// ayudaDe convierte los temas de la Ayuda a uno de los idiomas, en el mismo orden que la app.
export function ayudaDe(temas: TemaWeb[], textos: Textos): Tema[] {
  const tx = (k: string) => plano(textos[k] ?? k)
  return temas.map((t) => {
    const url = `${URL_AYUDA}${t.id}.html`
    return { id: t.id, titulo: tx(t.titulo), ...(t.paginaApp ? { pagina: t.paginaApp } : {}), resumen: tx(t.resumen),
      bloques: t.bloques.map((b) => bloqueDe(b, tx, url)) }
  })
}

// textosDe trae los nombres que la terminal comparte con la Web UI (menu.*) y los de CLAVES_TEXTOS.
export function textosDe(porIdioma: Record<Idioma, Textos>): Record<Idioma, Textos> {
  const claves = [...Object.keys(porIdioma.es).filter((k) => k.startsWith('menu.')), ...CLAVES_TEXTOS]
  return Object.fromEntries(IDIOMAS.map((i) => [i, Object.fromEntries(claves.map((k) => [k, plano(porIdioma[i][k] ?? k)]))])) as Record<Idioma, Textos>
}

// plantillasDe da, por idioma, las plantillas de «Respuesta con forma» con su esquema JSON (el de esquemaCampos).
export function plantillasDe(plantillas: Plantilla[], porIdioma: Record<Idioma, Textos>) {
  return Object.fromEntries(IDIOMAS.map((i) => [i, plantillas.map((p) => ({
    id: p.id, nombre: porIdioma[i][p.nombre] ?? p.nombre, instrucciones: p.instrucciones(i), esquema: aEsquema(p.campos(i)),
  }))])) as Record<Idioma, { id: string; nombre: string; instrucciones: string; esquema: unknown }[]>
}

// entradaEjemplos es la prueba fija con la que se generan los ejemplos de «Ver como código»: la Tarea 8 la reproduce en Go
// y compara su código con el de ejemplos.json (en español).
export function entradaEjemplos(extraer: Plantilla) {
  return { url: 'http://127.0.0.1:17373', clave: 'tramtai_clave_de_ejemplo', modelo: 'perfil/rapido',
    solicitud: { instrucciones: extraer.instrucciones('es'),
      mensajes: [{ rol: 'usuario', contenido: 'Factura de Soporte S.L. del 2026-03-14 por 120,50 euros.' }],
      salida: { formato: 'json', esquema: aEsquema(extraer.campos('es')) } } }
}
type Entrada = ReturnType<typeof entradaEjemplos>

// EJEMPLOS son los que se guardan: la API nativa en curl, Python y JavaScript (el ejemplo node).
const EJEMPLOS = { 'curl-nativa': 'curl', 'python-nativa': 'python', 'node-nativa': 'node' }

export interface Fuente {
  temas: TemaWeb[]
  plantillas: Plantilla[]
  es: Textos
  en: Textos
  // ejemplos da el código de la entrada, en español, por cada ejemplo de ejemplos().
  ejemplos: (entrada: Entrada) => { id: string; codigo: string }[]
}

const json = (v: unknown) => JSON.stringify(v, null, 2) + '\n'

// generar devuelve cada archivo por su ruta relativa a internal/tui/datos/.
export function generar(f: Fuente): Map<string, string> {
  const textos = { es: f.es, en: f.en }
  const extraer = f.plantillas.find((p) => p.id === 'extraer')
  if (!extraer) throw new Error('falta la plantilla extraer')
  const entrada = entradaEjemplos(extraer)
  const lista = f.ejemplos(entrada)
  const ejemplos = Object.entries(EJEMPLOS).map(([nativo, id]) => {
    const e = lista.find((x) => x.id === nativo)
    if (!e) throw new Error(`falta el ejemplo ${nativo}`)
    return { id, codigo: e.codigo }
  })
  return new Map([
    ['ayuda.json', json({ es: ayudaDe(f.temas, f.es), en: ayudaDe(f.temas, f.en) })],
    ['plantillas.json', json(plantillasDe(f.plantillas, textos))],
    ['textos.json', json(textosDe(textos))],
    ['testdata/ejemplos.json', json({ entrada, ejemplos })],
  ])
}
