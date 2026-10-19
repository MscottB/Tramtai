// enlaceSeguro devuelve la URL solo si es https: un enlace que llega del servicio (la página de una versión) nunca puede
// ser javascript: ni otro esquema. Vue no lo filtra en :href.
export function enlaceSeguro(url: string | undefined): string | undefined {
  return url?.startsWith('https://') ? url : undefined
}
