import type { Tema } from '../tipos'

// preguntas son las frecuentes que no encajan en otro tema; cada tema lleva las suyas.
export const preguntas: Tema = {
  id: 'preguntas',
  icono: 'o_quiz',
  titulo: 'ayuda.preguntas.titulo',
  resumen: 'ayuda.preguntas.resumen',
  terminal: 'ayuda.preguntas.terminal',
  conDiagrama: false,
  relacionados: ['empezar', 'problemas', 'conceptos'],
  bloques: [
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'chats', pregunta: 'ayuda.preguntas.pChats', respuesta: 'ayuda.preguntas.rChats' },
        { id: 'servidor', pregunta: 'ayuda.preguntas.pServidor', respuesta: 'ayuda.preguntas.rServidor' },
        { id: 'empresa', pregunta: 'ayuda.preguntas.pEmpresa', respuesta: 'ayuda.preguntas.rEmpresa' },
        { id: 'puerto', pregunta: 'ayuda.preguntas.pPuerto', respuesta: 'ayuda.preguntas.rPuerto' },
        { id: 'desinstalar', pregunta: 'ayuda.preguntas.pDesinstalar', respuesta: 'ayuda.preguntas.rDesinstalar' },
      ],
    },
  ],
}
