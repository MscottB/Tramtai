// node --test --import ./pruebas.mjs: la Web UI importa sin extensión (como resuelve Vite); este gancho prueba con .ts y
// .js cuando Node no encuentra el módulo, para que las pruebas carguen el código tal cual.
import { registerHooks } from 'node:module'

registerHooks({
  resolve(especificador, contexto, siguiente) {
    try {
      return siguiente(especificador, contexto)
    } catch (e) {
      if (e?.code !== 'ERR_MODULE_NOT_FOUND') throw e
      for (const extension of ['.ts', '.js']) {
        try {
          return siguiente(especificador + extension, contexto)
        } catch {
          // se prueba la siguiente
        }
      }
      throw e
    }
  },
})
