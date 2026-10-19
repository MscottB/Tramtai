// compartida devuelve una función que, mientras f está en curso, devuelve la misma promesa en lugar de lanzar otra.
// Así varias peticiones que reciben 401 a la vez abren una sola sesión: con una por petición, el token CSRF que
// quedaba podía no ser el de la cookie y la siguiente acción fallaba.
export function compartida<T>(f: () => Promise<T>): () => Promise<T> {
  let enCurso: Promise<T> | undefined
  return () => (enCurso ??= f().finally(() => (enCurso = undefined)))
}
