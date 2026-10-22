import type { Tema } from '../tipos'

// conceptos es el glosario: una definición corta por palabra, con el tema que la explica.
export const conceptos: Tema = {
  id: 'conceptos',
  icono: 'o_menu_book',
  titulo: 'ayuda.conceptos.titulo',
  resumen: 'ayuda.conceptos.resumen',
  conDiagrama: false,
  relacionados: ['empezar', 'agentes', 'recetas'],
  bloques: [
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'agente', pregunta: 'ayuda.cAgente', respuesta: 'ayuda.cAgenteTexto', tema: 'agentes' },
        { id: 'modelo', pregunta: 'ayuda.cModelo', respuesta: 'ayuda.cModeloTexto', tema: 'agentes' },
        { id: 'perfil', pregunta: 'ayuda.cPerfil', respuesta: 'ayuda.cPerfilTexto', tema: 'agentes' },
        { id: 'esfuerzo', pregunta: 'ayuda.cEsfuerzo', respuesta: 'ayuda.cEsfuerzoTexto', tema: 'probar' },
        { id: 'receta', pregunta: 'ayuda.cReceta', respuesta: 'ayuda.cRecetaTexto', tema: 'recetas' },
        { id: 'respaldo', pregunta: 'ayuda.cRespaldo', respuesta: 'ayuda.cRespaldoTexto', tema: 'recetas' },
        { id: 'apis', pregunta: 'ayuda.cApis', respuesta: 'ayuda.cApisTexto', tema: 'conectar' },
        { id: 'peticion', pregunta: 'ayuda.cPeticion', respuesta: 'ayuda.cPeticionTexto', tema: 'historial' },
        { id: 'esquema', pregunta: 'ayuda.cEsquema', respuesta: 'ayuda.cEsquemaTexto', tema: 'json' },
        { id: 'contexto', pregunta: 'ayuda.cContexto', respuesta: 'ayuda.cContextoTexto', tema: 'archivos' },
        { id: 'espacio', pregunta: 'ayuda.cEspacio', respuesta: 'ayuda.cEspacioTexto' },
        { id: 'presupuesto', pregunta: 'ayuda.cPresupuesto', respuesta: 'ayuda.cPresupuestoTexto', tema: 'presupuestos' },
      ],
    },
  ],
}
