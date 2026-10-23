import type { Tema } from '../tipos'

// json explica la respuesta con forma: esquema, validación y correcciones.
export const json: Tema = {
  id: 'json',
  icono: 'o_data_object',
  titulo: 'ayuda.json.titulo',
  resumen: 'ayuda.json.resumen',
  conDiagrama: true,
  relacionados: ['probar', 'recetas', 'conectar'],
  bloques: [
    {
      tipo: 'diagrama',
      paradas: [
        { icono: 'o_schema', titulo: 'ayuda.json.d1', nota: 'ayuda.json.d1Nota' },
        { icono: 'o_route', titulo: 'ayuda.json.d2', nota: 'ayuda.json.d2Nota' },
        { icono: 'o_rule', titulo: 'ayuda.json.d3', nota: 'ayuda.json.d3Nota' },
        { icono: 'o_build', titulo: 'ayuda.json.d4', nota: 'ayuda.json.d4Nota' },
      ],
    },
    { tipo: 'pasos', pasos: ['ayuda.json.paso1', 'ayuda.json.paso2', 'ayuda.json.paso3', 'ayuda.json.paso4', 'ayuda.json.paso5'] },
    { tipo: 'nota', texto: 'ayuda.json.nota' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'formatos-json', pregunta: 'ayuda.json.pFormatos', respuesta: 'ayuda.json.rFormatos' },
        { id: 'esquema-a-mano', pregunta: 'ayuda.json.pMano', respuesta: 'ayuda.json.rMano' },
      ],
    },
    { tipo: 'errores', codigos: ['salida_no_cumple_esquema'] },
    { tipo: 'ir', pagina: 'ejecutar' },
  ],
}
