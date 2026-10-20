import type { Clave } from '../../idiomas/es'
import type { Tema } from '../tipos'

// ERRORES son los más comunes, con el mismo título que muestra la app (error.<código>, salvo los que no vienen del
// servicio) y qué hacer con cada uno.
export const ERRORES: Record<string, { titulo?: Clave; solucion: Clave }> = {
  sin_conexion: { titulo: 'app.sinConexion', solucion: 'ayuda.problemas.sin_conexion' },
  no_autorizado: { solucion: 'ayuda.problemas.no_autorizado' },
  autenticacion_agente: { solucion: 'ayuda.problemas.autenticacion_agente' },
  agente_no_disponible: { solucion: 'ayuda.problemas.agente_no_disponible' },
  limite_uso_proveedor: { solucion: 'ayuda.problemas.limite_uso_proveedor' },
  modelo_no_disponible: { solucion: 'ayuda.problemas.modelo_no_disponible' },
  parametro_no_soportado: { solucion: 'ayuda.problemas.parametro_no_soportado' },
  entrada_demasiado_grande: { solucion: 'ayuda.problemas.entrada_demasiado_grande' },
  presupuesto_agotado: { titulo: 'ayuda.problemas.presupuestoTitulo', solucion: 'ayuda.problemas.presupuesto_agotado' },
  salida_no_cumple_esquema: { solucion: 'ayuda.problemas.salida_no_cumple_esquema' },
  tiempo_ejecucion_agotado: { solucion: 'ayuda.problemas.tiempo_ejecucion_agotado' },
  cola_llena: { solucion: 'ayuda.problemas.cola_llena' },
  conversion_fallida: { solucion: 'ayuda.problemas.conversion_fallida' },
  conversor_no_disponible: { solucion: 'ayuda.problemas.conversor_no_disponible' },
  error_agente: { solucion: 'ayuda.problemas.error_agente' },
  error_protocolo_agente: { solucion: 'ayuda.problemas.error_protocolo_agente' },
  tiempo_cola_agotado: { solucion: 'ayuda.problemas.tiempo_cola_agotado' },
  modelo_no_permitido: { solucion: 'ayuda.problemas.modelo_no_permitido' },
  modelo_no_configurado: { solucion: 'ayuda.problemas.modelo_no_configurado' },
  cuota_cargas_agotada: { solucion: 'ayuda.problemas.cuota_cargas_agotada' },
}
export const CODIGOS_ERROR: readonly string[] = Object.keys(ERRORES)

// problemas reúne todos los errores; «¿Qué hago?» junto a un error abre este tema en su ancla e-<código>.
export const problemas: Tema = {
  id: 'problemas',
  icono: 'o_build',
  titulo: 'ayuda.problemas.titulo',
  resumen: 'ayuda.problemas.resumen',
  conDiagrama: false,
  relacionados: ['historial', 'agentes', 'conectar'],
  bloques: [
    { tipo: 'pasos', pasos: ['ayuda.problemas.paso1', 'ayuda.problemas.paso2', 'ayuda.problemas.paso3', 'ayuda.problemas.paso4'] },
    { tipo: 'errores', codigos: [...CODIGOS_ERROR] },
    { tipo: 'ir', pagina: 'historial' },
  ],
}
