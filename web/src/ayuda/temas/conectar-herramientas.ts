import type { Tema } from '../tipos'

// conectarHerramientas cuenta cómo apunta Tramtai Claude Code, Codex, Continue, opencode y Kilo a sí mismo (pestaña Herramientas de
// Mis APIs): qué archivo toca en cada una, dónde queda la clave, los estados y cómo se deshace. No lleva paginaApp: la página de
// Mis APIs ya abre el tema «conectar», y este no tiene recorrido. Su id no es «herramientas» porque ese es el de las herramientas
// de un agente.
export const conectarHerramientas: Tema = {
  id: 'conectar-herramientas',
  icono: 'o_extension',
  titulo: 'ayuda.conectarHerramientas.titulo',
  resumen: 'ayuda.conectarHerramientas.resumen',
  terminal: 'ayuda.conectarHerramientas.terminal',
  conDiagrama: false,
  relacionados: ['claves', 'conectar'],
  bloques: [
    {
      tipo: 'tabla',
      columnas: ['ayuda.conectarHerramientas.cArchivo', 'ayuda.conectarHerramientas.cClave'],
      filas: [
        { titulo: 'ayuda.conectarHerramientas.fClaudeCode', celdas: ['ayuda.conectarHerramientas.archivoClaudeCode', 'ayuda.conectarHerramientas.claveClaudeCode'] },
        { titulo: 'ayuda.conectarHerramientas.fCodex', celdas: ['ayuda.conectarHerramientas.archivoCodex', 'ayuda.conectarHerramientas.claveCodex'] },
        { titulo: 'ayuda.conectarHerramientas.fContinue', celdas: ['ayuda.conectarHerramientas.archivoContinue', 'ayuda.conectarHerramientas.claveArchivo'] },
        { titulo: 'ayuda.conectarHerramientas.fOpencode', celdas: ['ayuda.conectarHerramientas.archivoOpencode', 'ayuda.conectarHerramientas.claveArchivo'] },
        { titulo: 'ayuda.conectarHerramientas.fKilo', celdas: ['ayuda.conectarHerramientas.archivoKilo', 'ayuda.conectarHerramientas.claveArchivo'] },
      ],
    },
    { tipo: 'nota', texto: 'ayuda.conectarHerramientas.api' },
    {
      tipo: 'pasos',
      pasos: ['ayuda.conectarHerramientas.paso1', 'ayuda.conectarHerramientas.paso2', 'ayuda.conectarHerramientas.paso3', 'ayuda.conectarHerramientas.paso4'],
    },
    { tipo: 'captura', id: 'herramientas', alt: 'ayuda.conectarHerramientas.captura' },
    {
      tipo: 'tabla',
      columnas: ['ayuda.conectarHerramientas.cSignifica'],
      filas: [
        { titulo: 'ayuda.conectarHerramientas.fNoDetectada', celdas: ['ayuda.conectarHerramientas.noDetectada'] },
        { titulo: 'ayuda.conectarHerramientas.fDetectada', celdas: ['ayuda.conectarHerramientas.detectada'] },
        { titulo: 'ayuda.conectarHerramientas.fActiva', celdas: ['ayuda.conectarHerramientas.activa'] },
        { titulo: 'ayuda.conectarHerramientas.fDesactivada', celdas: ['ayuda.conectarHerramientas.desactivada'] },
        { titulo: 'ayuda.conectarHerramientas.fDesactualizada', celdas: ['ayuda.conectarHerramientas.desactualizada'] },
        { titulo: 'ayuda.conectarHerramientas.fSoloGlobal', celdas: ['ayuda.conectarHerramientas.soloGlobal'] },
      ],
    },
    { tipo: 'nota', texto: 'ayuda.conectarHerramientas.motivos' },
    {
      tipo: 'pasos',
      titulo: 'ayuda.conectarHerramientas.deshacer',
      pasos: ['ayuda.conectarHerramientas.des1', 'ayuda.conectarHerramientas.des2', 'ayuda.conectarHerramientas.des3'],
    },
    { tipo: 'nota', texto: 'ayuda.conectarHerramientas.copias' },
    { tipo: 'nota', texto: 'ayuda.conectarHerramientas.sincronizar' },
    { tipo: 'nota', texto: 'ayuda.conectarHerramientas.gestionada' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'clave-en-archivo', pregunta: 'ayuda.conectarHerramientas.pArchivo', respuesta: 'ayuda.conectarHerramientas.rArchivo' },
        { id: 'herramienta-no-activa', pregunta: 'ayuda.conectarHerramientas.pNoActiva', respuesta: 'ayuda.conectarHerramientas.rNoActiva' },
        { id: 'herramienta-solo-proyecto', pregunta: 'ayuda.conectarHerramientas.pProyecto', respuesta: 'ayuda.conectarHerramientas.rProyecto' },
        { id: 'continue-config-json', pregunta: 'ayuda.conectarHerramientas.pContinue', respuesta: 'ayuda.conectarHerramientas.rContinue' },
        { id: 'codex-proveedor-ajeno', pregunta: 'ayuda.conectarHerramientas.pProveedor', respuesta: 'ayuda.conectarHerramientas.rProveedor' },
        { id: 'codex-cambiar-modelo', pregunta: 'ayuda.conectarHerramientas.pModelo', respuesta: 'ayuda.conectarHerramientas.rModelo' },
        { id: 'herramienta-editada-a-mano', pregunta: 'ayuda.conectarHerramientas.pMano', respuesta: 'ayuda.conectarHerramientas.rMano' },
        { id: 'herramienta-api-mismo-nombre', pregunta: 'ayuda.conectarHerramientas.pNombre', respuesta: 'ayuda.conectarHerramientas.rNombre' },
        { id: 'herramientas-orden-clave', pregunta: 'ayuda.conectarHerramientas.pOrdenClave', respuesta: 'ayuda.conectarHerramientas.rOrdenClave' },
        { id: 'herramientas-cli', pregunta: 'ayuda.conectarHerramientas.pCli', respuesta: 'ayuda.conectarHerramientas.rCli' },
        { id: 'herramientas-varias', pregunta: 'ayuda.conectarHerramientas.pVarias', respuesta: 'ayuda.conectarHerramientas.rVarias' },
      ],
    },
    { tipo: 'ir', pagina: 'conectar' },
  ],
}
