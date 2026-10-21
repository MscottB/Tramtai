// recordarBorrador guarda en sessionStorage lo que se escribe en un campo, para que una recarga (al cambiar de espacio
// en otra pestaña) no lo pierda; vacío, lo borra. Sin almacenamiento, o si falla, el campo funciona igual.
import { watch, type Ref } from 'vue'

type Almacen = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

function sesionDelNavegador(): Almacen | undefined {
  try {
    return typeof sessionStorage === 'undefined' ? undefined : sessionStorage
  } catch {
    return undefined
  }
}

export function recordarBorrador(clave: string, valor: Ref<string>, almacen: Almacen | undefined = sesionDelNavegador()) {
  try {
    const guardado = almacen?.getItem(clave)
    if (guardado && !valor.value) valor.value = guardado
  } catch {
    // sin borrador
  }
  watch(valor, (v) => {
    try {
      if (v) almacen?.setItem(clave, v)
      else almacen?.removeItem(clave)
    } catch {
      // se sigue sin borrador
    }
  }, { flush: 'sync' })
}
