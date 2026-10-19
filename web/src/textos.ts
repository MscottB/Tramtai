// buscarTexto elige el texto de una clave: con n igual a 1, la variante en singular «<clave>Uno» si existe. Una clave que
// no existe devuelve la propia clave, para que se vea el fallo en lugar de romper quien la use (un undefined).
export function buscarTexto(textos: Record<string, string>, clave: string, n?: number | string): string {
  return (n === 1 && textos[clave + 'Uno']) || textos[clave] || clave
}
