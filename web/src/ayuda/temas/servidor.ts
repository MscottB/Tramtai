import type { Tema } from '../tipos'

// servidor es el modo servidor: instalarlo, abrirlo a la red con HTTPS, el administrador, las claves y la suscripción.
// El aviso son los cinco riesgos de riesgos.* (los del servidor): el mismo texto que terminos.Riesgos, sin copiarlo.
export const servidor: Tema = {
  id: 'servidor',
  icono: 'o_dns',
  titulo: 'ayuda.servidor.titulo',
  resumen: 'ayuda.servidor.resumen',
  terminal: 'ayuda.servidor.terminal',
  conDiagrama: false,
  relacionados: ['entornos', 'ajustes', 'claves'],
  bloques: [
    { tipo: 'pasos', pasos: ['ayuda.servidor.paso1', 'ayuda.servidor.paso2', 'ayuda.servidor.paso3', 'ayuda.servidor.paso4'] },
    { tipo: 'captura', id: 'servidor', alt: 'ayuda.servidor.captura' },
    { tipo: 'nota', texto: 'ayuda.servidor.nota' },
    { tipo: 'pasos', titulo: 'ayuda.servidor.aviso', pasos: ['riesgos.1', 'riesgos.2', 'riesgos.3', 'riesgos.4', 'riesgos.5'] },
    { tipo: 'nota', texto: 'ayuda.servidor.notaTerminos' },
    { tipo: 'captura', id: 'entrar', alt: 'ayuda.servidor.capturaEntrar' },
    { tipo: 'captura', id: 'seguridad', alt: 'ayuda.servidor.capturaSeguridad' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'servidor-que-es', pregunta: 'ayuda.servidor.pQue', respuesta: 'ayuda.servidor.rQue', tema: 'entornos' },
        { id: 'servidor-https', pregunta: 'ayuda.servidor.pHttps', respuesta: 'ayuda.servidor.rHttps' },
        { id: 'servidor-proxy', pregunta: 'ayuda.servidor.pProxy', respuesta: 'ayuda.servidor.rProxy' },
        { id: 'servidor-red-local', pregunta: 'ayuda.servidor.pRedLocal', respuesta: 'ayuda.servidor.rRedLocal' },
        { id: 'servidor-mismo-equipo', pregunta: 'ayuda.servidor.pMismo', respuesta: 'ayuda.servidor.rMismo' },
        { id: 'servidor-claves', pregunta: 'ayuda.servidor.pClaves', respuesta: 'ayuda.servidor.rClaves' },
        { id: 'servidor-agentes', pregunta: 'ayuda.servidor.pAgentes', respuesta: 'ayuda.servidor.rAgentes' },
        { id: 'servidor-suscripcion', pregunta: 'ayuda.servidor.pSuscripcion', respuesta: 'ayuda.servidor.rSuscripcion' },
        { id: 'servidor-codex', pregunta: 'ayuda.servidor.pCodex', respuesta: 'ayuda.servidor.rCodex' },
        { id: 'servidor-actualizar', pregunta: 'ayuda.servidor.pActualizar', respuesta: 'ayuda.servidor.rActualizar' },
        { id: 'servidor-quitar', pregunta: 'ayuda.servidor.pQuitar', respuesta: 'ayuda.servidor.rQuitar' },
      ],
    },
    { tipo: 'ir', pagina: 'configuracion' },
  ],
}
