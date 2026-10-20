import type { Tema } from '../tipos'

// empezar explica qué hace Tramtai y lleva de la instalación a la primera respuesta.
export const empezar: Tema = {
  id: 'empezar',
  icono: 'o_flag',
  titulo: 'ayuda.empezar.titulo',
  resumen: 'ayuda.empezar.resumen',
  paginaApp: 'inicio',
  conDiagrama: true,
  relacionados: ['agentes', 'probar', 'conectar'],
  bloques: [
    {
      tipo: 'diagrama',
      paradas: [
        { icono: 'o_apps', titulo: 'ayuda.empezar.d1', nota: 'ayuda.empezar.d1Nota' },
        { icono: 'o_vpn_key', titulo: 'ayuda.empezar.d2', nota: 'ayuda.empezar.d2Nota' },
        { icono: 'o_hourglass_top', titulo: 'ayuda.empezar.d3', nota: 'ayuda.empezar.d3Nota' },
        { icono: 'o_smart_toy', titulo: 'ayuda.empezar.d4', nota: 'ayuda.empezar.d4Nota' },
        { icono: 'o_task_alt', titulo: 'ayuda.empezar.d5', nota: 'ayuda.empezar.d5Nota' },
      ],
    },
    { tipo: 'descargas' },
    { tipo: 'pasos', pasos: ['ayuda.empezar.paso1', 'ayuda.empezar.paso2', 'ayuda.empezar.paso3', 'ayuda.empezar.paso4'] },
    { tipo: 'captura', id: 'inicio', alt: 'ayuda.empezar.captura' },
    { tipo: 'nota', texto: 'ayuda.empezar.nota' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'tranvias', pregunta: 'ayuda.empezar.pTranvias', respuesta: 'ayuda.empezar.rTranvias' },
        { id: 'entrar', pregunta: 'ayuda.empezar.pEntrar', respuesta: 'ayuda.empezar.rEntrar' },
        { id: 'en-marcha', pregunta: 'ayuda.empezar.pMarcha', respuesta: 'ayuda.empezar.rMarcha' },
        { id: 'windows', pregunta: 'ayuda.empezar.pWindows', respuesta: 'ayuda.empezar.rWindows' },
        { id: 'macos', pregunta: 'ayuda.empezar.pMacos', respuesta: 'ayuda.empezar.rMacos' },
        { id: 'linux', pregunta: 'ayuda.empezar.pLinux', respuesta: 'ayuda.empezar.rLinux' },
        { id: 'portable', pregunta: 'ayuda.empezar.pPortable', respuesta: 'ayuda.empezar.rPortable' },
        { id: 'portable-datos', pregunta: 'ayuda.empezar.pPortableDatos', respuesta: 'ayuda.empezar.rPortableDatos' },
        { id: 'pendrive', pregunta: 'ayuda.empezar.pPendrive', respuesta: 'ayuda.empezar.rPendrive' },
        { id: 'solo-terminal', pregunta: 'ayuda.empezar.pTerminal', respuesta: 'ayuda.empezar.rTerminal' },
      ],
    },
    { tipo: 'ir', pagina: 'agentes' },
  ],
}
