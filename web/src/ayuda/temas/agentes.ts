import type { Tema } from '../tipos'

// agentes explica cómo se conecta cada agente (suscripción o clave) y cómo se eligen modelo y esfuerzo.
export const agentes: Tema = {
  id: 'agentes',
  icono: 'o_smart_toy',
  titulo: 'ayuda.agentes.titulo',
  resumen: 'ayuda.agentes.resumen',
  paginaApp: 'agentes',
  conDiagrama: true,
  relacionados: ['probar', 'empezar', 'conceptos'],
  bloques: [
    {
      tipo: 'diagrama',
      paradas: [
        { icono: 'o_key', titulo: 'ayuda.agentes.d1', nota: 'ayuda.agentes.d1Nota' },
        { icono: 'o_smart_toy', titulo: 'ayuda.agentes.d2', nota: 'ayuda.agentes.d2Nota' },
        { icono: 'o_memory', titulo: 'ayuda.agentes.d3', nota: 'ayuda.agentes.d3Nota' },
      ],
    },
    { tipo: 'pasos', pasos: ['ayuda.agentes.paso1', 'ayuda.agentes.paso2', 'ayuda.agentes.paso3', 'ayuda.agentes.paso4'] },
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
      ],
    },
    { tipo: 'ir', pagina: 'agentes' },
  ],
}
