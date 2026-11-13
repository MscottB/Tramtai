import type { Tema } from '../tipos'

// preparar cuenta cómo instala, conecta, actualiza y desinstala Tramtai los CLI de los agentes (v0.60): la receta de cada
// uno, las versiones comprobadas (la lista firmada) y las órdenes de la CLI. Comparte la página Agentes con el tema
// «agentes», que va antes en indice.ts y es el que abre su botón ?; sus pasos del recorrido se suman a los de esa página.
export const preparar: Tema = {
  id: 'preparar',
  icono: 'o_download',
  titulo: 'ayuda.preparar.titulo',
  resumen: 'ayuda.preparar.resumen',
  terminal: 'ayuda.preparar.terminal',
  paginaApp: 'agentes',
  conDiagrama: true,
  relacionados: ['agentes', 'empezar', 'servidor'],
  bloques: [
    {
      tipo: 'diagrama',
      paradas: [
        { icono: 'o_download', titulo: 'ayuda.preparar.d1', nota: 'ayuda.preparar.d1Nota' },
        { icono: 'o_login', titulo: 'ayuda.preparar.d2', nota: 'ayuda.preparar.d2Nota' },
        { icono: 'o_update', titulo: 'ayuda.preparar.d3', nota: 'ayuda.preparar.d3Nota' },
      ],
    },
    { tipo: 'pasos', pasos: ['ayuda.preparar.paso1', 'ayuda.preparar.paso2', 'ayuda.preparar.paso3', 'ayuda.preparar.paso4'] },
    {
      tipo: 'recorrido',
      pasos: [
        { ancla: 'agentes.instalar', titulo: 'ayuda.preparar.rec1Titulo', texto: 'ayuda.preparar.rec1Texto' },
        { ancla: 'agentes.preparar', titulo: 'ayuda.preparar.rec2Titulo', texto: 'ayuda.preparar.rec2Texto' },
      ],
    },
    { tipo: 'captura', id: 'preparar', alt: 'ayuda.preparar.captura' },
    {
      tipo: 'tabla',
      columnas: ['ayuda.preparar.cSesion'],
      filas: [
        { titulo: 'ayuda.preparar.fClaude', celdas: ['ayuda.preparar.sClaude'] },
        { titulo: 'ayuda.preparar.fCodex', celdas: ['ayuda.preparar.sCodex'] },
        { titulo: 'ayuda.preparar.fCopilot', celdas: ['ayuda.preparar.sCopilot'] },
        { titulo: 'ayuda.preparar.fAntigravity', celdas: ['ayuda.preparar.sAntigravity'] },
      ],
    },
    { tipo: 'nota', texto: 'ayuda.preparar.nota' },
    {
      tipo: 'pasos',
      titulo: 'ayuda.preparar.versiones',
      pasos: ['ayuda.preparar.v1', 'ayuda.preparar.v2', 'ayuda.preparar.v3', 'ayuda.preparar.v4'],
    },
    {
      tipo: 'pasos',
      titulo: 'ayuda.preparar.cli',
      pasos: ['ayuda.preparar.c1', 'ayuda.preparar.c2', 'ayuda.preparar.c3', 'ayuda.preparar.cSuscripcion', 'ayuda.preparar.c4'],
    },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'preparar-donde', pregunta: 'ayuda.preparar.pDonde', respuesta: 'ayuda.preparar.rDonde' },
        { id: 'preparar-node', pregunta: 'ayuda.preparar.pNode', respuesta: 'ayuda.preparar.rNode' },
        { id: 'preparar-cerrar', pregunta: 'ayuda.preparar.pCerrar', respuesta: 'ayuda.preparar.rCerrar' },
        { id: 'preparar-desactivada', pregunta: 'ayuda.preparar.pDesactivada', respuesta: 'ayuda.preparar.rDesactivada' },
        { id: 'preparar-no-avanza', pregunta: 'ayuda.preparar.pNoAvanza', respuesta: 'ayuda.preparar.rNoAvanza' },
        { id: 'preparar-sitios', pregunta: 'ayuda.preparar.pSitios', respuesta: 'ayuda.preparar.rSitios' },
        { id: 'preparar-sin-consulta', pregunta: 'ayuda.preparar.pSinConsulta', respuesta: 'ayuda.preparar.rSinConsulta' },
        { id: 'preparar-peticiones', pregunta: 'ayuda.preparar.pPeticiones', respuesta: 'ayuda.preparar.rPeticiones' },
        { id: 'preparar-desinstalar', pregunta: 'ayuda.preparar.pDesinstalar', respuesta: 'ayuda.preparar.rDesinstalar' },
        { id: 'preparar-servidor', pregunta: 'ayuda.preparar.pServidor', respuesta: 'ayuda.preparar.rServidor', tema: 'servidor' },
      ],
    },
    { tipo: 'ir', pagina: 'agentes' },
  ],
}
