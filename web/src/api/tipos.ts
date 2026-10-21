// Tipos de lo que envía y recibe la API del servicio; sin código, para que cualquier módulo los use sin dependencias.

export interface InstanciaResumida {
  id: string
  ambito: string
  raiz?: string
  nombre: string
}
export interface Sesion {
  instancia: string
  ambito?: string
  raiz?: string
  nombre?: string
  csrf: string
  version: string
  // local: entró sin clave por ser el mismo usuario del equipo; puede cambiar de instancia.
  local: boolean
  instancias?: InstanciaResumida[]
  // autoarranque: si Tramtai arranca al iniciar sesión; solo en sesiones locales de sistemas que lo admiten.
  autoarranque?: boolean
  // servicio: el puerto en uso, el configurado y si se puede reiniciar desde aquí (solo sesiones locales).
  servicio?: { puerto: number; puerto_configurado: number; puerto_automatico: boolean; reiniciable: boolean }
  // almacen: dónde guarda este sistema las claves de proveedor, cifradas y fuera de Tramtai (vacío si no hay).
  almacen?: { disponible: boolean; nombre: string }
}
// EspacioUI es un espacio de trabajo en la página «Espacios»: también los detenidos y los que no encuentran su carpeta.
export interface EspacioUI {
  id: string
  ambito: 'global' | 'proyecto'
  ruta?: string
  nombre: string
  activo: boolean
  ultimo_uso?: string
  ultimo_error?: string
  estado: 'activo' | 'detenido' | 'no_encontrado'
  confiado: boolean
  apis: number
  peticiones: number
}
export interface Uso {
  tokens_entrada: number
  tokens_salida: number
  costo_usd?: number
}
export interface Mensaje {
  rol: string
  contenido: string
  llamadas?: Llamada[] // en un mensaje del asistente: las herramientas que pidió
  id_llamada?: string // con rol herramienta: la llamada a la que responde
  es_error?: boolean
}
// Llamada es una herramienta que pidió el agente, con sus argumentos ya validados.
export interface Llamada {
  id: string
  nombre: string
  argumentos: Record<string, unknown>
}
export interface Solicitud {
  agente?: string
  modelo?: string
  perfil_modelo?: string
  perfil?: string
  esfuerzo?: string
  instrucciones?: string
  respaldo?: string[]
  mensajes?: Mensaje[] // la retención la borra al terminar
  cargas?: string[]
  salida?: { formato?: string; esquema?: unknown; reintentos?: number; conservar_salida_invalida?: boolean }
  ejecucion?: { tiempo_limite_segundos?: number }
  herramientas?: { nombre: string; descripcion?: string; parametros?: unknown }[]
  eleccion_herramienta?: 'auto' | 'ninguna' | 'obligatoria' | { nombre: string }
  llamadas_paralelas?: boolean
}
export interface Trabajo {
  id: string
  instancia: string
  estado: string
  codigo_error?: string
  mensaje_error?: string
  solicitud: Solicitud
  salida?: string
  disponibilidad?: string
  entradas_eliminadas?: boolean
  intento_de?: string
  punto_acceso?: string
  origen?: string
  modelo_enviado?: string
  modelo_efectivo?: string
  uso: Uso
  creado_en: string
  terminado_en?: string
  // errores_salida: con salida_no_cumple_esquema, por qué la última respuesta no cumplía.
  errores_salida?: ProblemaSalida[]
  llamadas?: Llamada[] // con herramientas: las que pidió el agente (salida es solo el texto)
}
// ProblemaSalida es un motivo por el que la respuesta no cumple el esquema; campo es la ruta (tareas[2].fecha), vacía
// para la respuesta entera.
export interface ProblemaSalida {
  campo: string
  problema: 'falta' | 'tipo' | 'valor_no_permitido' | 'sobra' | 'formato' | 'tamaño' | 'otro'
  texto: string
}
// EventoRegistro es un paso de una petición con su hora: estado, intento (JSON con agente, modelo y esfuerzo) o aviso.
export interface EventoRegistro {
  numero: number
  tipo: string
  datos: string
  creado_en: string
}
export interface EstadoInstancia {
  id: string
  ambito: string
  raiz?: string
  archivo_token: string
  agente_predeterminado: string
  perfil_predeterminado?: string
  trabajos: Record<string, number>
  base: string
}
export interface EstadoPunto {
  nombre: string
  grupo?: string
  instrucciones?: string
  instancia: string
  url?: string
  contratos: string[]
  agente?: string
  modelo?: string
  perfil_modelo?: string
  esfuerzo?: string
  modelos_permitidos?: string[]
  forzar_modelo?: boolean
  esquema?: Record<string, unknown>
  respaldo?: string[]
  reintentos_salida?: number
  presupuesto?: Presupuesto
  activo: boolean
  error?: string
  origen: string
}
// Presupuesto son los límites diarios y mensuales de una API o del espacio; 0 o ausente = sin límite.
export interface Limites {
  peticiones?: number
  tokens?: number
  usd?: number
}
export interface Presupuesto {
  diario?: Limites
  mensual?: Limites
}
// Consumo es lo gastado en un periodo; terminados sirve para estimar lo que gastarán los trabajos en curso.
export interface Consumo {
  peticiones: number
  terminados: number
  tokens: number
  usd: number
}
export interface EstadoPresupuesto {
  diario: { limites: Limites; consumo: Consumo; reinicio: string }
  mensual: { limites: Limites; consumo: Consumo; reinicio: string }
}

// Retencion son los plazos (horas) y el tope del historial de la instancia, y lo que ocupa ahora.
export interface Retencion {
  horas_resultado: number
  horas_metadatos: number
  max_trabajos: number
  uso: { trabajos: number; bytes: number }
}

// UsoPunto son las peticiones de una API que conserva la retención: total, con error y la última.
export interface UsoPunto {
  peticiones: number
  errores: number
  ultima?: string
}
export interface ConfigPunto {
  nombre: string
  grupo?: string
  instrucciones?: string
  puerto?: number
  contratos?: string[]
  agente?: string
  modelo?: string
  perfil_modelo?: string
  esfuerzo?: string
  modelos_permitidos?: string[]
  forzar_modelo?: boolean
  alias_modelos?: Record<string, string>
  esquema?: Record<string, unknown>
  respaldo?: string[]
  reintentos_salida?: number
  presupuesto?: Presupuesto
}
// Modelo es una entrada del catálogo: agente, agente/modelo o perfil/nombre, con la versión real que informó el
// agente la última vez y si es el predeterminado del espacio.
export interface Modelo {
  id: string
  agente?: string
  modelo?: string
  perfil?: string
  // origen: configurado, predeterminado_del_cli o visto (una versión concreta que informó el agente).
  origen?: string
  resuelto?: string
  version?: string
  predeterminado?: boolean
  // esfuerzos: niveles de esfuerzo que admite su agente (vacío si no se puede elegir).
  esfuerzos?: string[]
  // esfuerzo_predeterminado es el nivel que usa el agente si no se elige; ausente si no se conoce con certeza.
  esfuerzo_predeterminado?: string
  // contexto_tokens es el contexto conocido del modelo y contexto_origen de dónde sale (configurado o informado).
  contexto_tokens?: number
  contexto_origen?: string
  // esfuerzo_espacio, solo en el modelo predeterminado: el esfuerzo que el espacio usa con él (vacío, el de su agente).
  esfuerzo_espacio?: string
}
// LimitesPeticion son los topes de una petición y de sus cargas (/api/v1/limites).
// EstadoConversor es el del conversor de archivos (markitdown): GET /api/v1/conversor.
export interface EstadoConversor {
  estado: 'listo' | 'preparando' | 'falta' | 'error'
  origen?: 'configurado' | 'paquete' | 'preparado' | 'sistema'
  version?: string
  etapa?: 'python' | 'paquetes' | 'comprobacion'
  progreso?: number
  mensaje?: string
  preparar_al_iniciar: boolean
}
export interface LimitesPeticion {
  extensiones_texto?: string[]
  extensiones_conversor?: string[]
  cargas_por_peticion: number
  carga_mb: number
  texto_convertido_mb: number
  cuota_cargas_mb: number
  mensajes: number
  reserva_salida_tokens: number
}
export interface EntradaConfiguracion {
  clave: string
  valor: unknown
  origen: string
}
export interface Adaptador {
  // cuenta: la de la suscripción, si el CLI la informa; desconectado: apagado en Agentes.
  cuenta?: string
  desconectado?: boolean
  agente: string
  version_detectada?: string
  autenticacion?: string
  terminos?: string
  problema?: string
  // problema_codigo: no_instalado, falta_clave, aviso (no impide usarlo) u otro (vacío) que hay que revisar.
  problema_codigo?: string
  capacidades: { streaming: string; salida_estructurada: boolean; modelo_efectivo_informado: boolean }
}

// EstadoActualizacion es el aviso de versión nueva, compartido por la cabecera y Ajustes. instalable indica que
// esta copia se puede actualizar desde aquí (instalada con el instalador de Windows).
export interface EstadoActualizacion {
  actual: string
  buscar: boolean
  disponible: boolean
  version: string
  url: string
  notas?: string
  instalable: boolean
  comprobado?: string // cuándo consultó el servicio a GitHub por última vez
  fallo?: boolean // si esa consulta falló
}

// Acerca son los datos de «Acerca de Tramtai»; rutas solo llega a una sesión local.
export interface Acerca {
  version: string
  commit?: string
  modificado?: boolean
  go: string
  sistema: string
  arquitectura: string
  almacen: string
  servicio: { direccion: string; pid: number; iniciado_en: string }
  rutas?: { datos: string; configuracion: string; registro: string }
  portable?: boolean // la copia guarda sus datos junto al .exe (portable.txt)
}
// Novedad es una versión publicada con sus notas, para el diálogo que sale tras actualizar.
export interface Novedad {
  version: string
  url?: string
  notas?: string
}

// ProgresoActualizacion es el avance de «Actualizar ahora»: fase vacía si no hay ninguna en curso.
export interface ProgresoActualizacion {
  fase: '' | 'descargando' | 'verificando' | 'listo' | 'instalando' | 'error'
  version: string
  recibidos: number
  total: number
  instala_en?: string // en la fase listo: cuándo instalará si nadie decide antes
  error?: string
}

// AjustesPunto son los cambios de una API (PATCH): solo cambia lo que se envía. modelo es un id del catálogo y ''
// vuelve al del espacio; esquema null lo quita.
export interface AjustesPunto {
  instrucciones?: string
  esfuerzo?: string
  modelo?: string
  forzar_modelo?: boolean
  modelos_permitidos?: string[]
  esquema?: Record<string, unknown> | null
  respaldo?: string[]
  reintentos_salida?: number
  presupuesto?: Presupuesto | null
}
