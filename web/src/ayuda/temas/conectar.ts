import type { Bloque, Tema } from '../tipos'

// tablaApis compara lo que permite cada contrato; la dibujan este tema y «¿Qué URL uso?» de Mis APIs (TablaApis.vue).
export const tablaApis: Extract<Bloque, { tipo: 'tabla' }> = {
  tipo: 'tabla',
  columnas: ['ayuda.conectar.colOpenai', 'ayuda.conectar.colAnthropic', 'ejemplos.nativa'],
  filas: [
    { titulo: 'guia.fSdk', celdas: ['guia.si', 'guia.si', 'guia.noHttp'] },
    { titulo: 'guia.fVivo', celdas: ['guia.si', 'guia.si', 'guia.siRetoma'] },
    { titulo: 'guia.fJson', celdas: ['guia.siResponseFormat', 'guia.no', 'guia.siEsquema'] },
    { titulo: 'guia.fArchivos', celdas: ['guia.siBase64', 'guia.siBase64', 'guia.si50'] },
    { titulo: 'guia.fHerramientas', celdas: ['guia.si', 'guia.si', 'guia.si'] },
    { titulo: 'guia.fTareas', celdas: ['guia.no', 'guia.no', 'guia.si'] },
    { titulo: 'guia.fRespaldo', celdas: ['guia.soloEsfuerzo', 'guia.no', 'guia.si'] },
    { titulo: 'guia.fRegistro', celdas: ['guia.no', 'guia.no', 'guia.si'] },
    { titulo: 'guia.fLimites', celdas: ['guia.no', 'guia.no', 'guia.si'] },
  ],
}

// conectar lleva una app a Tramtai: URL, clave y el campo model.
export const conectar: Tema = {
  id: 'conectar',
  icono: 'o_hub',
  titulo: 'ayuda.conectar.titulo',
  resumen: 'ayuda.conectar.resumen',
  paginaApp: 'conectar',
  conDiagrama: true,
  relacionados: ['archivos', 'herramientas', 'presupuestos'],
  bloques: [
    {
      tipo: 'diagrama',
      paradas: [
        { icono: 'o_apps', titulo: 'ayuda.conectar.d1', nota: 'ayuda.conectar.d1Nota' },
        { icono: 'o_link', titulo: 'ayuda.conectar.d2', nota: 'ayuda.conectar.d2Nota' },
        { icono: 'o_hub', titulo: 'ayuda.conectar.d3', nota: 'ayuda.conectar.d3Nota' },
        { icono: 'o_smart_toy', titulo: 'ayuda.conectar.d4', nota: 'ayuda.conectar.d4Nota' },
      ],
    },
    { tipo: 'pasos', pasos: ['ayuda.conectar.paso1', 'ayuda.conectar.paso2', 'ayuda.conectar.paso3', 'ayuda.conectar.paso4'] },
    { tipo: 'ejemplo', contratos: ['openai', 'anthropic', 'nativa'], archivos: [] },
    { tipo: 'captura', id: 'mis-apis', alt: 'ayuda.conectar.captura' },
    tablaApis,
    { tipo: 'nota', texto: 'ayuda.conectar.porQue' },
    { tipo: 'nota', texto: 'ayuda.conectar.local' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'que-url', pregunta: 'ayuda.conectar.pUrl', respuesta: 'ayuda.conectar.rUrl' },
        { id: 'api-por-app', pregunta: 'ayuda.conectar.pPorApp', respuesta: 'ayuda.conectar.rPorApp' },
        { id: 'clave-nueva', pregunta: 'ayuda.conectar.pClave', respuesta: 'ayuda.conectar.rClave' },
        { id: 'prueba-a-api', pregunta: 'ayuda.conectar.pPruebaApi', respuesta: 'ayuda.conectar.rPruebaApi' },
        { id: 'clave-filtrada', pregunta: 'ayuda.conectar.pFiltrada', respuesta: 'ayuda.conectar.rFiltrada' },
      ],
    },
    { tipo: 'ir', pagina: 'conectar' },
  ],
}
