// Generado por internal/contrato/gents (go generate ./internal/contrato): no lo edites a mano.
// Son los tipos que el servicio envía y recibe, sacados de los structs de Go.

export interface Trabajo {
  id: string
  instancia: string
  estado: string
  codigo_error?: string
  mensaje_error?: string
  solicitud: Solicitud
  salida?: string
  intento_de?: string
  punto_acceso?: string
  origen?: string
  modelo_enviado?: string
  modelo_efectivo?: string
  uso: Uso
  disponibilidad?: string
  pid?: number
  entradas_eliminadas?: boolean
  creado_en: string
  actualizado_en: string
  terminado_en?: string
  etapa: 'cola' | 'preparacion' | 'ejecucion' | 'validacion' | 'llegada'
  corregible: boolean
  llamadas?: Llamada[]
  errores_salida?: ProblemaSalida[]
}

export interface Solicitud {
  agente?: string
  modelo?: string
  perfil_modelo?: string
  esfuerzo?: string
  respaldo?: string[]
  perfil?: string
  modo_contexto?: string
  instrucciones?: string
  mensajes?: Mensaje[]
  cargas?: string[]
  salida?: ConfigSalida
  ejecucion?: ConfigEjecucion
  herramientas?: Herramienta[]
  eleccion_herramienta?: 'auto' | 'ninguna' | 'obligatoria' | { nombre: string }
  llamadas_paralelas?: boolean
}

export interface EventoRegistro {
  numero: number
  tipo: string
  datos: string
  creado_en: string
  etapa?: 'cola' | 'preparacion' | 'ejecucion' | 'validacion' | 'llegada'
}

export interface EventoTrabajo {
  id: string
  estado: string
  etapa: 'cola' | 'preparacion' | 'ejecucion' | 'validacion' | 'llegada'
  agente: string
  modelo: string
  api: string
  origen: string
  intento: number
  codigo_error: string
  creado: string
  actualizado: string
}

export interface Capacidades {
  version: string
  formato_serializacion: string
  entradas: string[]
  adaptadores: Adaptador[]
  limites: LimitesPeticion
  conversor?: EstadoConversor
}

export interface PrimerosPasos {
  pasos: Paso[]
  adaptadores: Adaptador[]
}

export interface ResumenInstancia {
  id: string
  ambito: string
  raiz?: string
  archivo_token: string
  agente_predeterminado: string
  perfil_predeterminado?: string
  trabajos: Record<string, number>
  ultimo_uso?: string
  base: string
  app_con_clave_24h: number
  sugerir_api_propia: boolean
}

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
  claves?: string[]
}

export interface ListaPuntos {
  puntos_acceso: EstadoPunto[]
  uso: Record<string, UsoPunto>
  presupuestos: Record<string, EstadoPresupuesto>
}

export interface ConfigPunto {
  nombre: string
  grupo?: string
  instrucciones?: string
  puerto?: number
  ruta_base?: string
  instancia?: string
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
  activo?: boolean
  ejecucion?: EjecucionPunto
}

export interface Retencion {
  horas_resultado: number
  horas_metadatos: number
  max_trabajos: number
  uso: UsoHistorial
}

export interface EntradaConfiguracion {
  clave: string
  valor: unknown
  origen: string
}

export interface Modelo {
  id: string
  agente?: string
  modelo?: string
  perfil?: string
  origen: string
  resuelto?: string
  version?: string
  predeterminado?: boolean
  esfuerzos?: string[]
  esfuerzo_predeterminado?: string
  contexto_tokens?: number
  contexto_origen?: string
  esfuerzo_espacio?: string
}

export interface Uso {
  tokens_entrada: number
  tokens_salida: number
  costo_usd?: number
}

export interface Llamada {
  id: string
  nombre: string
  argumentos: Record<string, unknown>
}

export interface ProblemaSalida {
  campo: string
  problema: 'falta' | 'tipo' | 'valor_no_permitido' | 'sobra' | 'formato' | 'tamaño' | 'otro'
  texto: string
}

export interface Mensaje {
  rol: string
  contenido: string
  llamadas?: Llamada[]
  id_llamada?: string
  es_error?: boolean
}

export interface ConfigSalida {
  formato?: string
  validacion_estricta?: boolean
  esquema?: unknown
  conservar_salida_invalida?: boolean
  reintentos?: number
}

export interface ConfigEjecucion {
  tiempo_limite_segundos?: number
  persistir_sesion?: boolean
}

export interface Herramienta {
  nombre: string
  descripcion?: string
  parametros?: unknown
}

export interface Adaptador {
  agente: string
  estado: 'listo' | 'desconectado' | 'no_instalado' | 'falta_clave' | 'sin_sesion' | 'usa_clave' | 'sesion_no_responde' | 'revisar'
  desconectado: boolean
  version_detectada?: string
  autenticacion?: string
  terminos?: string
  cuenta?: string
  problema?: string
  problema_codigo?: string
  capacidades: CapacidadesAgente
}

export interface LimitesPeticion {
  extensiones_texto: string[]
  extensiones_conversor: string[]
  cargas_por_peticion: number
  carga_mb: number
  texto_convertido_mb: number
  cuota_cargas_mb: number
  cuerpo_mb: number
  esquema_kb: number
  mensajes: number
  reserva_salida_tokens: number
}

export interface EstadoConversor {
  estado: 'listo' | 'preparando' | 'falta' | 'error'
  origen?: 'configurado' | 'paquete' | 'preparado' | 'sistema'
  version?: string
  etapa?: 'python' | 'paquetes' | 'comprobacion'
  progreso?: number
  mensaje?: string
  preparar_al_iniciar: boolean
}

export interface Paso {
  pagina: string
  hecho: boolean
  listos: number
  total: number
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

export interface UsoPunto {
  peticiones: number
  errores: number
  ultima?: string
}

export interface EstadoPresupuesto {
  diario: EstadoPeriodo
  mensual: EstadoPeriodo
}

export interface Presupuesto {
  diario?: Limites
  mensual?: Limites
}

export interface EjecucionPunto {
  modo_contexto?: string
  formato_salida?: string
  tiempo_limite_segundos?: number
  maximo_trabajos?: number
}

export interface UsoHistorial {
  trabajos: number
  bytes: number
}

export interface CapacidadesAgente {
  texto: boolean
  streaming: string
  salida_estructurada: boolean
  uso_tokens: string
  modelo_efectivo_informado: boolean
  tool_calling_compatible: boolean
  imagenes: boolean
  sesiones_persistentes: boolean
}

export interface EstadoPeriodo {
  limites: Limites
  consumo: Consumo
  reinicio: string
}

export interface Limites {
  peticiones?: number
  tokens?: number
  usd?: number
}

export interface Consumo {
  peticiones: number
  terminados: number
  tokens: number
  usd: number
}

// estadosTerminales son los estados de un trabajo que ya acabó (nucleo.Terminales).
export const estadosTerminales: readonly string[] = ['completado', 'fallido', 'cancelado', 'interrumpido']

// extensionesConversor son las que pasan por el conversor de archivos (documentos.Formatos).
export const extensionesConversor: readonly string[] = ['.docx', '.htm', '.html', '.pdf', '.pptx', '.xlsx']
