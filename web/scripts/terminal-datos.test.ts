// node --test web/scripts/*.test.ts: los datos que la terminal en Go incrusta salen de las fuentes de la Web UI.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import es from '../src/idiomas/es.ts'
import en from '../src/idiomas/en.ts'
import { temas } from '../src/ayuda/indice.ts'
import { plantillas } from '../src/plantillas.ts'
import { aEsquema, ejemploDe } from '../src/esquemaCampos.ts'
import { ayudaDe, CLAVES_AGENTES, CLAVES_EJECUTAR, CLAVES_HISTORIAL, CLAVES_INICIO, ejemplosDeCampos, generar, textosDe, plantillasDe, URL_AYUDA,
  type Tema } from './terminal-datos.ts'

const fuente = { temas, plantillas, es, en, ejemplos: () => [{ id: 'curl-nativa', codigo: 'a' }, { id: 'python-nativa', codigo: 'b' },
  { id: 'node-nativa', codigo: 'c' }, { id: 'go-nativa', codigo: 'd' }, { id: 'python', codigo: 'e' }] }
const ayuda = { es: ayudaDe(temas, es), en: ayudaDe(temas, en) }

// cadenas recorre todas las cadenas de un valor JSON.
function* cadenas(v: unknown): Generator<string> {
  if (typeof v === 'string') yield v
  else if (Array.isArray(v)) for (const x of v) yield* cadenas(x)
  else if (v && typeof v === 'object') for (const x of Object.values(v)) yield* cadenas(x)
}

test('los 18 temas están en es y en con los mismos bloques', () => {
  assert.equal(ayuda.es.length, 18)
  assert.deepEqual(ayuda.en.map((t) => t.id), ayuda.es.map((t) => t.id))
  ayuda.es.forEach((t, i) => assert.deepEqual(ayuda.en[i].bloques.map((b) => b.tipo), t.bloques.map((b) => b.tipo), t.id))
})

test('la Ayuda es texto plano: ninguna cadena lleva < ni **', () => {
  for (const idioma of ['es', 'en'] as const) for (const s of cadenas(ayuda[idioma])) assert.doesNotMatch(s, /<|\*\*/, s)
})

test('el recorrido pasa como bloque pantalla sin sus textos, que describen la pantalla de la Web UI', () => {
  const tema = temas.find((t) => t.paginaApp === 'ejecutar')!
  for (const lengua of ['es', 'en'] as const) {
    const t = ayuda[lengua].find((x) => x.id === tema.id) as Tema
    assert.equal(t.pagina, 'ejecutar')
    assert.deepEqual(t.bloques.find((b) => b.tipo === 'pantalla'), { tipo: 'pantalla' })
  }
})

// FRASES_SOLO_WEB son las que solo valen en la Web UI (hallazgo 69); la misma lista que internal/tui/datos/datos_test.go.
const FRASES_SOLO_WEB = ['Haz el recorrido', 'tranvía', 'mapa de Inicio', 'menú lateral', 'botón \\?', '\\b(el|un) panel\\b',
  'sin abrir ninguna terminal', 'selector de arriba', 'arriba a la derecha',
  'Take the guided tour', '\\btrams?\\b', 'Home map', 'side menu', '\\? button', '\\b(the|a) panel\\b', 'without opening a terminal',
  'selector at the top', 'top right']

test('ninguna frase que solo vale en la Web UI llega a la terminal', () => {
  const archivos = generar(fuente)
  for (const nombre of ['ayuda.json', 'textos.json'])
    for (const f of FRASES_SOLO_WEB) assert.doesNotMatch(archivos.get(nombre)!, new RegExp(f, 'i'), `${nombre}: ${f}`)
})

test('la terminal usa la variante .terminal de un texto y deja fuera las preguntas soloWeb', () => {
  for (const [lengua, textos] of [['es', es], ['en', en]] as const) {
    const t = textos as Record<string, string>
    const empezar = ayuda[lengua].find((x) => x.id === 'empezar')!
    const preguntas = empezar.bloques.find((b) => b.tipo === 'preguntas')!.pares!
    assert.ok(preguntas.some(([, r]) => r === t['ayuda.empezar.rMarcha.terminal']), lengua)
    assert.ok(!preguntas.some(([p]) => p === t['ayuda.empezar.pRecorrido']), lengua)
    assert.equal(textosDe({ es, en })[lengua]['arranque.texto'], t['arranque.texto.terminal'].replace(/\*\*/g, ''))
  }
})

test('capturas, diagramas, descargas y enlaces ir pasan a un bloque web con la URL del tema', () => {
  const tema = temas.find((t) => t.bloques.some((b) => b.tipo === 'captura'))!
  const web = ayuda.es.find((t) => t.id === tema.id)!.bloques.filter((b) => b.tipo === 'web')
  assert.ok(web.length > 0)
  for (const b of web) assert.equal(b.url, `${URL_AYUDA}${tema.id}.html`)
  assert.equal(URL_AYUDA, 'https://mscottb.github.io/Tramtai/ayuda/')
})

test('cada tema lleva su URL en el sitio y la línea «En la terminal» de su fuente, en los dos idiomas', () => {
  for (const [lengua, textos, inicio] of [['es', es, 'En la terminal:'], ['en', en, 'In the terminal:']] as const)
    temas.forEach((fuenteTema, i) => {
      const t = ayuda[lengua][i]
      assert.equal(t.url, `${URL_AYUDA}${t.id}.html`)
      assert.equal(t.terminal, (textos as Record<string, string>)[fuenteTema.terminal], `${lengua}: ${t.id}`)
      assert.ok(t.terminal.startsWith(inicio), `${lengua}: ${t.id}: ${t.terminal}`)
    })
})

test('un bloque que el generador no conoce lo hace fallar en lugar de salir como null', () => {
  const nuevo = { ...temas[0], bloques: [{ tipo: 'nuevo' }] } as unknown as typeof temas[number]
  assert.throws(() => ayudaDe([nuevo], es), /tipo de bloque sin mapear: nuevo/)
})

test('los errores llevan su título y qué hacer; las preguntas, pregunta y respuesta', () => {
  const problemas = ayuda.es.find((t) => t.id === 'problemas')!
  const errores = problemas.bloques.find((b) => b.tipo === 'errores')!
  assert.ok(errores.pares!.length > 10)
  assert.ok(errores.pares!.every(([titulo, solucion]) => titulo && solucion))
  const preguntas = ayuda.es.find((t) => t.id === 'preguntas')!.bloques.find((b) => b.tipo === 'preguntas')!
  assert.ok(preguntas.pares!.every(([p, r]) => p && r))
})

test('textos trae todas las menu.* y la Ayuda de la interfaz', () => {
  const textos = textosDe({ es, en })
  for (const [lengua, fuenteTextos] of [['es', es], ['en', en]] as const) {
    const menu = Object.keys(fuenteTextos).filter((k) => k.startsWith('menu.'))
    assert.ok(menu.length >= 11)
    for (const k of menu) assert.equal(textos[lengua][k], (fuenteTextos as Record<string, string>)[k], k)
    assert.ok(textos[lengua]['ayuda.buscar'])
  }
  assert.equal(textos.es['menu.inicio'], 'Inicio')
  assert.equal(textos.es['menu.grupo.empezar'], 'Empezar')
})

test('textos trae la solución de cada error de la Ayuda, para decir qué hacer junto al error', () => {
  const textos = textosDe({ es, en })
  for (const lengua of ['es', 'en'] as const) assert.ok(textos[lengua]['ayuda.problemas.agente_desconectado'], lengua)
  assert.equal(textos.es['ayuda.problemas.agente_desconectado'], es['ayuda.problemas.agente_desconectado'].replace(/\*\*/g, ''))
})

test('los textos de Agentes existen en los dos idiomas y salen tal cual', () => {
  const textos = textosDe({ es, en })
  for (const [lengua, fuenteTextos] of [['es', es], ['en', en]] as const) {
    for (const k of CLAVES_AGENTES) {
      const origen = (fuenteTextos as Record<string, string>)[k]
      assert.ok(origen, `${lengua}: falta ${k}`)
      assert.equal(textos[lengua][k], origen.replace(/\*\*/g, '').replace(/<([^<>]*)>/g, '[$1]'), `${lengua}: ${k}`)
    }
  }
  assert.equal(textos.en['agente.listo'], 'Ready')
  assert.match(textos.es['agentes.usaClave'], /\{cli\}/)
})

test('los textos de Inicio existen en los dos idiomas y salen tal cual', () => {
  const textos = textosDe({ es, en })
  for (const [lengua, fuenteTextos] of [['es', es], ['en', en]] as const) {
    for (const k of CLAVES_INICIO) {
      const origen = (fuenteTextos as Record<string, string>)[k]
      assert.ok(origen, `${lengua}: falta ${k}`)
      assert.equal(textos[lengua][k], origen.replace(/\*\*/g, '').replace(/<([^<>]*)>/g, '[$1]'), `${lengua}: ${k}`)
    }
  }
  assert.equal(textos.es['inicio.paso1'], 'Conecta al menos un agente')
  assert.match(textos.es['inicio.paso4'], /\{sistema\}/)
})

test('los textos del Historial y los títulos de error existen en los dos idiomas y salen tal cual', () => {
  const textos = textosDe({ es, en })
  for (const [lengua, fuenteTextos] of [['es', es], ['en', en]] as const) {
    const origen = fuenteTextos as Record<string, string>
    for (const k of CLAVES_HISTORIAL) {
      assert.ok(origen[k], `${lengua}: falta ${k}`)
      assert.equal(textos[lengua][k], origen[k].replace(/\*\*/g, '').replace(/<([^<>]*)>/g, '[$1]'), `${lengua}: ${k}`)
    }
    const errores = Object.keys(origen).filter((k) => k.startsWith('error.'))
    assert.ok(errores.length >= 40)
    for (const k of errores) assert.equal(textos[lengua][k], origen[k], `${lengua}: ${k}`)
  }
  assert.equal(textos.es['historial.solicitud'], 'Mensaje')
  assert.equal(textos.es['error.correccion_no_disponible'], 'Esta petición no se puede corregir: vuelve a enviarla')
  assert.match(textos.es['historial.vaciado'], /\{n\}/)
})

test('los textos de Probar existen en los dos idiomas y salen tal cual', () => {
  const textos = textosDe({ es, en })
  for (const [lengua, fuenteTextos] of [['es', es], ['en', en]] as const) {
    const origen = fuenteTextos as Record<string, string>
    for (const k of CLAVES_EJECUTAR) {
      assert.ok(origen[k], `${lengua}: falta ${k}`)
      assert.equal(textos[lengua][k], origen[k].replace(/\*\*/g, '').replace(/<([^<>]*)>/g, '[$1]'), `${lengua}: ${k}`)
    }
  }
  assert.equal(textos.es['codigo.tuClave'], '[tu clave]')
  assert.match(textos.es['ejecutar.adjuntarLimites'], /\{n\}.*\{mb\}.*\{texto\}/)
})

test('las plantillas salen con su nombre, instrucciones y el esquema de esquemaCampos', () => {
  const lista = plantillasDe(plantillas, { es, en })
  assert.equal(lista.es.length, 7)
  assert.equal(lista.en.length, 7)
  lista.en.forEach((p, i) => {
    assert.equal(p.id, plantillas[i].id)
    assert.equal(p.nombre, (en as Record<string, string>)[plantillas[i].nombre])
    assert.equal(p.instrucciones, plantillas[i].instrucciones('en'))
    assert.deepEqual(p.esquema, aEsquema(plantillas[i].campos('en')))
  })
})

test('las muestras de respuesta salen de ejemploDe, con los límites de cada tipo y las plantillas', () => {
  const { casos } = ejemplosDeCampos(plantillas)
  assert.equal(casos.length, 1 + plantillas.length)
  assert.equal(casos[0].ejemplo, '{"t":"…","f":"2026-01-31","total":5,"n":-3,"e":3,"s":true,"o":"alta","l":[{"a":true},{"a":true}],"v":[],"x":{}}')
  for (const c of casos) assert.equal(c.ejemplo, JSON.stringify(ejemploDe(c.campos)))
})

test('generar escribe los cinco archivos con LF y los ejemplos curl, python y node', () => {
  const archivos = generar(fuente)
  assert.deepEqual([...archivos.keys()], ['ayuda.json', 'plantillas.json', 'textos.json', 'testdata/ejemplos.json', 'testdata/ejemplos-campos.json'])
  for (const texto of archivos.values()) assert.ok(texto.endsWith('}\n') && !texto.includes('\r'))
  const ejemplos = JSON.parse(archivos.get('testdata/ejemplos.json')!)
  assert.deepEqual(ejemplos.ejemplos, [{ id: 'curl', codigo: 'a' }, { id: 'python', codigo: 'b' }, { id: 'node', codigo: 'c' }])
  assert.deepEqual(Object.keys(ejemplos.entrada), ['url', 'clave', 'modelo', 'solicitud'])
  assert.equal(generar(fuente).get('ayuda.json'), archivos.get('ayuda.json'), 'misma salida cada vez')
})
