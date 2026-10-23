// Navegar con datos: «abrir la página X con Y», como IrA{Pagina, Datos} en la terminal. La página de origen emite «ir»
// con una Intencion; App.vue la pasa como prop a la página de destino, que la lee al montarse. Volver atrás no la repite.
export type Intencion =
  // receta es el nombre de la API que Probar debe abrir para editarla (desde Mis APIs); modelo abre Probar con un
  // agente o modelo ya elegido (desde Agentes); seguir es el trabajo que Probar debe seguir en vivo al abrirse desde el
  // historial.
  | { pagina: 'ejecutar'; receta?: string; modelo?: string; seguir?: string }
  // filtro abre el Historial con un filtro de estado del servicio (en_curso, completado o con_error) desde los
  // contadores de Inicio; origen, con las peticiones de una API (desde Mis APIs) o un filtro ya armado («o:probar»
  // desde Probar); abrir, con el detalle de una petición (desde Inicio).
  | { pagina: 'historial'; filtro?: string; origen?: string; abrir?: string }
  // grupo abre Mis APIs filtrada por un grupo (desde el mapa de Inicio).
  | { pagina: 'conectar'; grupo?: string }

// IntencionDe es la intención que recibe una página.
export type IntencionDe<P extends Intencion['pagina']> = Extract<Intencion, { pagina: P }>

// Destino es adonde lleva «ir»: una dirección (#pagina o ayuda/<tema>) o una intención.
export type Destino = string | Intencion

// destinoDe separa la dirección a la que se va de la intención que lleva, si la hay.
export function destinoDe(destino: Destino): { id: string; intencion: Intencion | null } {
  return typeof destino === 'string' ? { id: destino, intencion: null } : { id: destino.pagina, intencion: destino }
}

// intencionPara son las props de la página que se muestra: su intención, si es para ella.
export function intencionPara(pagina: string, intencion: Intencion | null): { intencion?: Intencion } {
  return intencion?.pagina === pagina ? { intencion } : {}
}
