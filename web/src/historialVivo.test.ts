import { test } from 'node:test'
import assert from 'node:assert/strict'
import { historialVivo, type Flujo } from './historialVivo.ts'

// FlujoFalso hace de EventSource: emitir dispara sus oyentes y rechazar lo cierra con error, como cuando el servicio
// no tiene cupo.
class FlujoFalso implements Flujo {
  readyState = 1
  onerror: (() => void) | null = null
  cerrado = false
  private oyentes = new Map<string, (() => void)[]>()
  addEventListener(tipo: string, f: () => void) {
    this.oyentes.set(tipo, [...(this.oyentes.get(tipo) ?? []), f])
  }
  emitir(tipo: string) {
    for (const f of this.oyentes.get(tipo) ?? []) f()
  }
  rechazar() {
    this.readyState = 2
    this.onerror?.()
  }
  close() {
    this.cerrado = true
  }
}

// montar arma el Historial con un servicio falso: cuenta las recargas y deja terminar a mano la que está en vuelo.
function montar() {
  const flujos: FlujoFalso[] = []
  const sondeos = new Map<number, () => void>()
  let n = 0
  const m = {
    recargas: 0,
    enVuelo: 0,
    oculta: false,
    paginasExtra: false,
    flujos,
    sondeos,
    vivo: historialVivo({
      abrir: () => {
        const f = new FlujoFalso()
        flujos.push(f)
        return f
      },
      recargar: () => {
        m.recargas++
        m.enVuelo++
      },
      enVuelo: () => m.enVuelo,
      paginasExtra: () => m.paginasExtra,
      oculta: () => m.oculta,
      cadaCinco: (f) => {
        sondeos.set(++n, f)
        return n
      },
      parar: (id) => sondeos.delete(id),
    }),
    // terminar acaba la consulta en vuelo, como el finally de cargar.
    terminar() {
      m.enVuelo--
      m.vivo.ponerAlDia()
    },
  }
  return m
}

// Revisión final web (M4, V7): los casos que el lote V comprobó a mano en el navegador.
test('los eventos de la foto inicial y la primera foto no recargan', () => {
  const m = montar()
  m.vivo.seguir()
  m.flujos[0].emitir('trabajo')
  m.flujos[0].emitir('foto')
  assert.equal(m.recargas, 0)
})

test('un cambio recarga una vez', () => {
  const m = montar()
  m.vivo.seguir()
  m.flujos[0].emitir('foto')
  m.flujos[0].emitir('trabajo')
  assert.equal(m.recargas, 1)
})

test('una ráfaga de cambios con una consulta en vuelo se junta en una recarga más', () => {
  const m = montar()
  m.vivo.seguir()
  m.flujos[0].emitir('foto')
  for (let i = 0; i < 3; i++) m.flujos[0].emitir('trabajo')
  assert.equal(m.recargas, 1, 'la primera sale; las otras dos quedan pendientes')
  m.terminar()
  assert.equal(m.recargas, 2)
  m.terminar()
  assert.equal(m.recargas, 2)
})

test('la foto de una reconexión pone la lista al día', () => {
  const m = montar()
  m.vivo.seguir()
  m.flujos[0].emitir('foto')
  m.oculta = true
  m.vivo.ocultar()
  assert.ok(m.flujos[0].cerrado, 'con la pestaña oculta no ocupa un cupo')
  m.oculta = false
  m.vivo.seguir()
  m.flujos[1].emitir('foto')
  assert.equal(m.recargas, 1)
})

test('con «Ver más» cargado los cambios no recargan', () => {
  const m = montar()
  m.paginasExtra = true
  m.vivo.seguir()
  m.flujos[0].emitir('foto')
  m.flujos[0].emitir('trabajo')
  assert.equal(m.recargas, 0)
})

// Revisión final web (M3): si el flujo vuelve, el sondeo de respaldo se apaga y no conviven los dos.
test('con el flujo rechazado se consulta cada 5 s, y cuando el flujo vuelve el sondeo se apaga', () => {
  const m = montar()
  m.vivo.seguir()
  m.flujos[0].rechazar()
  assert.equal(m.sondeos.size, 1)
  const [sondeo] = m.sondeos.values()
  sondeo()
  assert.equal(m.recargas, 1)
  m.terminar()
  m.vivo.seguir()
  m.flujos[1].emitir('foto')
  assert.equal(m.sondeos.size, 0)
  m.vivo.cerrar()
  assert.ok(m.flujos[1].cerrado)
})
