import type { Tema } from '../tipos'

// entornos explica que tu Tramtai y el del servidor no comparten nada: qué es de cada uno y cómo saber en cuál estás.
export const entornos: Tema = {
  id: 'entornos',
  icono: 'o_compare_arrows',
  titulo: 'ayuda.entornos.titulo',
  resumen: 'ayuda.entornos.resumen',
  terminal: 'ayuda.entornos.terminal',
  conDiagrama: false,
  relacionados: ['servidor', 'ajustes', 'espacios'],
  bloques: [
    {
      tipo: 'tabla',
      columnas: ['ayuda.entornos.cTuyo', 'ayuda.entornos.cServidor'],
      filas: [
        { titulo: 'ayuda.entornos.fArranca', celdas: ['ayuda.entornos.tuyoArranca', 'ayuda.entornos.servidorArranca'] },
        { titulo: 'ayuda.entornos.fCuenta', celdas: ['ayuda.entornos.tuyoCuenta', 'ayuda.entornos.servidorCuenta'] },
        { titulo: 'ayuda.entornos.fDatos', celdas: ['ayuda.entornos.tuyoDatos', 'ayuda.entornos.servidorDatos'] },
        { titulo: 'ayuda.entornos.fClaves', celdas: ['ayuda.entornos.tuyoClaves', 'ayuda.entornos.servidorClaves'] },
        { titulo: 'ayuda.entornos.fOrdenes', celdas: ['ayuda.entornos.tuyoOrdenes', 'ayuda.entornos.servidorOrdenes'] },
        { titulo: 'ayuda.entornos.fEntrar', celdas: ['ayuda.entornos.tuyoEntrar', 'ayuda.entornos.servidorEntrar'] },
      ],
    },
    { tipo: 'nota', texto: 'ayuda.entornos.nota' },
    { tipo: 'captura', id: 'marca', alt: 'ayuda.entornos.captura' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'entorno-cual', pregunta: 'ayuda.entornos.pCual', respuesta: 'ayuda.entornos.rCual' },
        { id: 'entorno-copiar', pregunta: 'ayuda.entornos.pCopiar', respuesta: 'ayuda.entornos.rCopiar' },
        { id: 'entorno-aviso', pregunta: 'ayuda.entornos.pAviso', respuesta: 'ayuda.entornos.rAviso' },
        { id: 'entorno-cli', pregunta: 'ayuda.entornos.pCli', respuesta: 'ayuda.entornos.rCli' },
        { id: 'entorno-terminos', pregunta: 'ayuda.entornos.pTerminos', respuesta: 'ayuda.entornos.rTerminos' },
      ],
    },
  ],
}
