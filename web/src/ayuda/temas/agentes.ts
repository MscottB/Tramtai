import type { Tema } from '../tipos'

// agentes explica cómo se conecta cada agente (suscripción o clave) y cómo se eligen modelo y esfuerzo.
export const agentes: Tema = {
  id: 'agentes',
  icono: 'o_route',
  titulo: 'ayuda.agentes.titulo',
  resumen: 'ayuda.agentes.resumen',
  terminal: 'ayuda.agentes.terminal',
  paginaApp: 'agentes',
  conDiagrama: true,
  relacionados: ['probar', 'empezar', 'conceptos'],
  bloques: [
    {
      tipo: 'diagrama',
      paradas: [
        { icono: 'o_key', titulo: 'ayuda.agentes.d1', nota: 'ayuda.agentes.d1Nota' },
        { icono: 'o_route', titulo: 'ayuda.agentes.d2', nota: 'ayuda.agentes.d2Nota' },
        { icono: 'o_memory', titulo: 'ayuda.agentes.d3', nota: 'ayuda.agentes.d3Nota' },
      ],
    },
    { tipo: 'pasos', pasos: ['ayuda.agentes.paso1', 'ayuda.agentes.paso2', 'ayuda.agentes.paso3', 'ayuda.agentes.paso4'] },
    {
      tipo: 'recorrido',
      pasos: [
        { ancla: 'agentes.tarjeta', titulo: 'ayuda.agentes.rec1Titulo', texto: 'ayuda.agentes.rec1Texto', bienvenida: true },
        { ancla: 'agentes.conectar', titulo: 'ayuda.agentes.rec2Titulo', texto: 'ayuda.agentes.rec2Texto', bienvenida: true },
        { ancla: 'agentes.recomprobar', titulo: 'ayuda.agentes.rec3Titulo', texto: 'ayuda.agentes.rec3Texto' },
      ],
    },
    { tipo: 'captura', id: 'agente', alt: 'ayuda.agentes.captura' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'que-agentes', pregunta: 'ayuda.agentes.pCuales', respuesta: 'ayuda.agentes.rCuales' },
        { id: 'terminos', pregunta: 'ayuda.agentes.pTerminos', respuesta: 'ayuda.agentes.rTerminos' },
        { id: 'donde-clave', pregunta: 'ayuda.agentes.pClave', respuesta: 'ayuda.agentes.rClave' },
        { id: 'credenciales', pregunta: 'ayuda.agentes.pCredenciales', respuesta: 'ayuda.agentes.rCredenciales' },
        { id: 'copilot', pregunta: 'ayuda.agentes.pCopilot', respuesta: 'ayuda.agentes.rCopilot' },
        { id: 'copilot-creditos', pregunta: 'ayuda.agentes.pCreditos', respuesta: 'ayuda.agentes.rCreditos' },
        { id: 'gemini-clave', pregunta: 'ayuda.agentes.pGemini', respuesta: 'ayuda.agentes.rGemini' },
        { id: 'linux-claves', pregunta: 'ayuda.agentes.pLinux', respuesta: 'ayuda.agentes.rLinux' },
        { id: 'antigravity', pregunta: 'ayuda.agentes.pAntigravity', respuesta: 'ayuda.agentes.rAntigravity' },
        { id: 'dejar-suscripcion', pregunta: 'ayuda.agentes.pDejar', respuesta: 'ayuda.agentes.rDejar' },
        { id: 'sesion-cli', pregunta: 'ayuda.agentes.pSesion', respuesta: 'ayuda.agentes.rSesion' },
        { id: 'desconectar', pregunta: 'ayuda.agentes.pDesconectar', respuesta: 'ayuda.agentes.rDesconectar' },
      ],
    },
    { tipo: 'ir', pagina: 'agentes' },
  ],
}
