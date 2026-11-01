// La regla del estilo de la Ayuda hecha prueba: textos cortos, diagrama donde hay un recorrido, un tema por página
// de la app y cada clave escrita en los dos idiomas.
import { test } from 'node:test'
import { CODIGOS_ERROR } from './temas/problemas.ts'
import assert from 'node:assert/strict'
import es from '../idiomas/es.ts'
import en from '../idiomas/en.ts'
import ayudaEs from '../idiomas/ayuda.es.ts'
import { CLAVES_UI, temas, temaDe, temaPorId } from './indice.ts'
import { PAGINAS } from './tipos.ts'
import type { Bloque } from './tipos.ts'

const idiomas: Record<string, Record<string, string>> = { es, en }
const problemas = await import('./temas/problemas.ts').catch(() => null)

// cortas son las claves de un bloque que deben caber en 200 caracteres.
function cortas(b: Bloque): string[] {
  switch (b.tipo) {
    case 'diagrama':
      return [...b.paradas.flatMap((p) => [p.titulo, ...(p.nota ? [p.nota] : [])]), ...(b.ramas ?? []).map((r) => r.etiqueta)]
    case 'pasos':
      return [...(b.titulo ? [b.titulo] : []), ...b.pasos]
    case 'captura':
      return [b.alt]
    case 'tabla':
      return [...b.columnas, ...b.filas.flatMap((f) => [f.titulo, ...f.celdas])]
    case 'preguntas':
      return b.preguntas.flatMap((p) => [p.pregunta, p.respuesta])
    case 'nota':
      return [b.texto]
    case 'recorrido':
      return b.pasos.flatMap((p) => [p.titulo, p.texto])
    default:
      return []
  }
}

test('cada tema tiene un resumen de una línea en los dos idiomas', () => {
  for (const tema of temas)
    for (const [idioma, textos] of Object.entries(idiomas)) {
      for (const clave of [tema.titulo, tema.resumen]) assert.ok(textos[clave], `${tema.id}: falta ${clave} en ${idioma}`)
      assert.ok(textos[tema.resumen].length <= 140, `${tema.id}: el resumen en ${idioma} pasa de 140 caracteres`)
    }
})

// Hallazgo 69: lo que la Ayuda dice de la Web UI (el botón ?, el panel) no vale en la terminal; cada tema dice cómo se
// hace allí, con sus teclas, en una línea que solo enseña la terminal.
test('cada tema dice cómo se hace en la terminal, en una línea corta y en los dos idiomas', () => {
  const inicio: Record<string, string> = { es: 'En la terminal: ', en: 'In the terminal: ' }
  for (const tema of temas)
    for (const [idioma, textos] of Object.entries(idiomas)) {
      const linea = textos[tema.terminal]
      assert.ok(linea?.startsWith(inicio[idioma]), `${tema.id}: falta ${tema.terminal} en ${idioma}`)
      assert.ok(linea.length <= 200, `${tema.id}: ${tema.terminal} en ${idioma} tiene ${linea.length} caracteres`)
    }
})

// v0.56: Empezar dice cómo usar Tramtai solo con la terminal, en su propia sección y con los dos comandos.
test('Empezar tiene la sección «Solo con la terminal» con el servicio en segundo plano y tramtai terminal', () => {
  const seccion = temaPorId('empezar')?.bloques.find((b) => b.tipo === 'pasos' && b.titulo)
  assert.ok(seccion?.tipo === 'pasos' && seccion.titulo, 'falta la sección')
  assert.equal(es[seccion.titulo], 'Solo con la terminal')
  assert.equal(en[seccion.titulo], 'Terminal only')
  for (const textos of [es, en] as Record<string, string>[]) {
    const pasos = seccion.pasos.map((p) => textos[p]).join(' ')
    for (const orden of ['tramtai servicio iniciar -d', 'tramtai terminal', 'SSH', 'NO_COLOR', 'TRAMTAI_ASCII']) assert.ok(pasos.includes(orden), orden)
  }
})

test('pasos, notas, paradas y respuestas caben en 200 caracteres en los dos idiomas', () => {
  for (const tema of temas)
    for (const bloque of tema.bloques)
      for (const clave of cortas(bloque))
        for (const [idioma, textos] of Object.entries(idiomas)) {
          assert.ok(textos[clave], `${tema.id}: falta ${clave} en ${idioma}`)
          assert.ok(textos[clave].length <= 200, `${tema.id}: ${clave} en ${idioma} tiene ${textos[clave].length} caracteres`)
        }
})

test('los títulos de los pasos de un recorrido y de las secciones de pasos caben en 60 caracteres en los dos idiomas', () => {
  const titulos = [
    ...temas.flatMap((t) => t.bloques.flatMap((b) => (b.tipo === 'recorrido' ? b.pasos.map((p) => p.titulo) : []))),
    ...temas.flatMap((t) => t.bloques.flatMap((b) => (b.tipo === 'pasos' && b.titulo ? [b.titulo] : []))),
    'recorrido.finTitulo' as const,
  ]
  for (const clave of titulos)
    for (const [idioma, textos] of Object.entries(idiomas))
      assert.ok(textos[clave] && textos[clave].length <= 60, `${clave} en ${idioma} tiene ${textos[clave]?.length} caracteres`)
})

test('el texto del paso de cierre de la bienvenida cabe en 200 caracteres en los dos idiomas', () => {
  for (const [idioma, textos] of Object.entries(idiomas)) assert.ok(textos['recorrido.finTexto'].length <= 200, `recorrido.finTexto en ${idioma}`)
})

test('un tema que explica un recorrido tiene su diagrama', () => {
  for (const tema of temas.filter((t) => t.conDiagrama)) assert.ok(tema.bloques.some((b) => b.tipo === 'diagrama'), tema.id)
})

test('cada página principal de la app tiene su tema', () => {
  for (const pagina of PAGINAS) assert.ok(temaDe(pagina), `la página ${pagina} no tiene tema`)
})

test('relacionados, errores e ids son válidos', () => {
  const ids = temas.map((t) => t.id)
  assert.equal(new Set(ids).size, ids.length, 'ids de tema repetidos')
  for (const tema of temas) {
    assert.match(tema.id, /^[a-z]+(-[a-z]+)*$/)
    for (const b of tema.bloques)
      if (b.tipo === 'errores' && problemas)
        for (const c of b.codigos) assert.ok(problemas.CODIGOS_ERROR.includes(c), `${tema.id}: error ${c} desconocido`)
  }
})

// v0.58: «Servidor» lleva el aviso con los cinco riesgos de riesgos.* (los del servidor), sin copiarlos; «Dos entornos»
// compara los dos Tramtai en una tabla, y Ajustes cuenta las tres formas de arrancar.
test('Servidor tiene el aviso con riesgos.1 a riesgos.5, Dos entornos su tabla y Ajustes las tres formas de arrancar', () => {
  const aviso = temaPorId('servidor')?.bloques.find((b) => b.tipo === 'pasos' && b.titulo === 'ayuda.servidor.aviso')
  assert.ok(aviso?.tipo === 'pasos', 'falta la sección Aviso en Servidor')
  assert.deepEqual(aviso.pasos, ['riesgos.1', 'riesgos.2', 'riesgos.3', 'riesgos.4', 'riesgos.5'])
  assert.ok(temaPorId('entornos')?.bloques.some((b) => b.tipo === 'tabla'), 'Dos entornos sin tabla')
  for (const textos of [es, en] as Record<string, string>[]) {
    const ajustes = temaPorId('ajustes')!.bloques.flatMap((b) => (b.tipo === 'pasos' ? b.pasos : b.tipo === 'preguntas' ? b.preguntas.map((p) => p.respuesta) : []))
      .map((c) => textos[c]).join(' ')
    for (const forma of [textos['arranque.app'], textos['arranque.servicio'], 'tramtai-servidor instalar']) assert.ok(ajustes.includes(forma), forma)
  }
})

test('están los 19 temas y sus relacionados existen', () => {
  assert.equal(temas.length, 19)
  for (const tema of temas) for (const r of tema.relacionados) assert.ok(temaPorId(r), `${tema.id}: relacionado ${r} no existe`)
})

test('cada pregunta tiene un ancla única en toda la Ayuda y su tema existe', () => {
  const anclas = temas.flatMap((t) => t.bloques.flatMap((b) => (b.tipo === 'preguntas' ? b.preguntas : [])))
  const ids = anclas.map((p) => p.id)
  assert.equal(new Set(ids).size, ids.length, `anclas repetidas: ${ids.filter((x, i) => ids.indexOf(x) !== i)}`)
  for (const p of anclas) {
    assert.match(p.id, /^[a-z][a-z0-9-]*$/)
    assert.ok(!p.id.startsWith('e-'), `${p.id}: e- es de los errores`)
    if (p.tema) assert.ok(temaPorId(p.tema), `${p.id}: el tema ${p.tema} no existe`)
  }
})

test('ningún texto de la Ayuda queda sin usar', () => {
  const usadas = new Set<string>(CLAVES_UI)
  for (const tema of temas) {
    usadas.add(tema.titulo)
    usadas.add(tema.resumen)
    usadas.add(tema.terminal)
    for (const b of tema.bloques) for (const c of cortas(b)) usadas.add(c)
  }
  for (const e of Object.values(problemas?.ERRORES ?? {})) {
    usadas.add(e.solucion)
    if (e.titulo) usadas.add(e.titulo)
  }
  // La variante .terminal de un texto usado la usa la Ayuda de la terminal (web/scripts/terminal-datos.ts).
  for (const k of [...usadas]) usadas.add(k + '.terminal')
  assert.deepEqual(Object.keys(ayudaEs).filter((k) => !usadas.has(k)), [])
})

test('no se pierde lo que decía la ayuda vieja (rutas, pendrive, claves, créditos)', () => {
  const anclas = new Set(temas.flatMap((t) => t.bloques.flatMap((b) => (b.tipo === 'preguntas' ? b.preguntas.map((p) => p.id) : []))))
  for (const id of ['pendrive', 'datos-ruta', 'portable-datos', 'copilot-creditos', 'gemini-clave', 'linux-claves'])
    assert.ok(anclas.has(id), `falta la pregunta ${id}`)
})

test('los errores más comunes del servicio tienen su solución', () => {
  for (const c of ['error_agente', 'tiempo_cola_agotado', 'modelo_no_permitido', 'modelo_no_configurado', 'error_protocolo_agente', 'cuota_cargas_agotada'])
    assert.ok(problemas?.CODIGOS_ERROR.includes(c), `falta ${c}`)
})

test('cada error tiene título y una solución corta en los dos idiomas', () => {
  assert.ok(problemas, 'falta temas/problemas.ts')
  for (const [codigo, e] of Object.entries(problemas.ERRORES))
    for (const [idioma, textos] of Object.entries(idiomas)) {
      if (e.titulo) assert.ok(textos[e.titulo], `${codigo}: falta el título en ${idioma}`)
      else assert.ok(textos['error.' + codigo], `${codigo}: no hay error.${codigo} en ${idioma}`)
      assert.ok(textos[e.solucion] && textos[e.solucion].length <= 200, `${codigo}: solución en ${idioma}`)
    }
})

test('los errores de espacios y agentes tienen su respuesta', () => {
  for (const codigo of ['agente_desconectado', 'confianza_requerida', 'espacio_detenido']) assert.ok(CODIGOS_ERROR.includes(codigo), codigo)
})

// Revisión final web (M7): la Ayuda cita los botones con su texto exacto, en los dos idiomas.
test('la Ayuda nombra los botones del recorrido y de soporte con el texto que tienen en la app', () => {
  const cita = { es: (x: string) => `«${x}»`, en: (x: string) => `“${x}”` }
  for (const [idioma, textos] of Object.entries(idiomas) as ['es' | 'en', Record<string, string>][]) {
    assert.ok(textos['ayuda.empezar.rRecorrido'].includes(cita[idioma](textos['bienvenida.repetir'])), `${idioma}: recorrido`)
    assert.ok(textos['ayuda.ajustes.rVersion'].includes(cita[idioma](textos['acerca.copiar'])), `${idioma}: soporte`)
  }
})

// v0.56, Tarea 16: los números se escriben como los enseña la pantalla (numero() en Ajustes), no con otro separador.
test('los textos escriben los números como la pantalla', () => {
  const casos: [string, number][] = [['ayuda.ajustes.rRetencion', 10000], ['historialAjustes.maximoHint', 100000]]
  for (const [idioma, textos] of Object.entries(idiomas)) {
    for (const [clave, n] of casos) assert.ok(textos[clave].includes(n.toLocaleString(idioma)), `${idioma}: ${clave} debe decir ${n.toLocaleString(idioma)}`)
  }
})

// Revisión final web (M8, hallazgo 36): DESIGN.md prohíbe robots, cerebros, destellos y bombillas para decir «IA».
test('la Ayuda no usa iconos de robot, cerebro, destello ni bombilla', () => {
  for (const tema of temas) assert.doesNotMatch(JSON.stringify(tema), /o_smart_toy|o_lightbulb|o_psychology|o_auto_awesome/, tema.id)
})
