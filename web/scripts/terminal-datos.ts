// terminal-datos convierte las fuentes de la Web UI en los datos que la terminal en Go incrusta (internal/tui/datos): la
// Ayuda en texto plano, las plantillas de salida estructurada, los nombres del menú y los ejemplos de «Ver como código»
// como datos de prueba. Son funciones puras: terminal-datos.mjs les da los textos y escribe los archivos. El CI lo vuelve
// a correr y falla si lo guardado en el repo no coincide.
import type { Bloque as BloqueWeb, Tema as TemaWeb } from '../src/ayuda/tipos'
import type { Plantilla } from '../src/plantillas'
import { CLAVES_UI } from '../src/ayuda/indice'
import { ERRORES } from '../src/ayuda/temas/problemas'
import { aEsquema, ejemploDe, type Campo } from '../src/esquemaCampos'
import { URL_SITIO } from './sitio'

type Textos = Record<string, string>
type Idioma = 'es' | 'en'
const IDIOMAS: Idioma[] = ['es', 'en']

// Bloque es la forma que lee Go (internal/tui/datos): todo texto plano. En tabla, lineas son las columnas y filas llevan
// el título de la fila y sus celdas; en errores, preguntas y pantalla, pares son (título o pregunta, texto o respuesta).
export interface Bloque {
  tipo: 'texto' | 'pasos' | 'tabla' | 'preguntas' | 'errores' | 'pantalla' | 'web'
  titulo?: string
  lineas?: string[]
  filas?: string[][]
  pares?: [string, string][]
  url?: string
}
// url es la del tema en el sitio («Ver en la web») y terminal, la línea «En la terminal: …» con sus teclas.
export interface Tema { id: string; titulo: string; pagina?: string; resumen: string; url: string; terminal: string; bloques: Bloque[] }

export const URL_AYUDA = URL_SITIO + 'ayuda/'

// CLAVES_AGENTES son los textos de la página Agentes que la terminal repite tal cual (estados, etiquetas, detalles y
// confirmaciones). Los {placeholders} se quedan como están: Go los rellena con datos.Textof.
export const CLAVES_AGENTES: string[] = [
  'agente.listo', 'agente.noInstalado', 'agente.faltaClave', 'agente.revisar', 'agente.desconectado',
  'agentes.subtitulo', 'agentes.filtroTodos', 'agentes.filtroListos', 'agentes.filtroConectar', 'agentes.filtroInstalar',
  'agentes.suscripcion', 'agentes.claveApi', 'agentes.listoSuscripcion', 'agentes.listoClave', 'agentes.cuenta',
  'agentes.opcionSuscripcionTexto', 'agentes.desconectadoTexto', 'agentes.desconectarTitulo', 'agentes.desconectarTexto',
  'agentes.conectado', 'agentes.desconectado', 'agentes.suscripcionActivada', 'agentes.claveActivada',
  'agentes.confirmarTitulo', 'agentes.riesgoSuscripcion', 'agentes.aceptoRiesgo', 'agentes.usaClave', 'agentes.sesionNoResponde',
  'agentes.version', 'agentes.sinVersion', 'agentes.autenticacion', 'agentes.enVivo', 'agentes.fragmentos', 'agentes.alTerminar',
  'agentes.json', 'agentes.informaModelo', 'agentes.esfuerzo', 'agentes.predeterminadoCli', 'agentes.comoInstalar',
  'comun.si', 'comun.no', 'comun.cargando',
]

// CLAVES_INICIO son los textos de la página Inicio que la terminal repite tal cual: la dirección, la red en vivo, los
// contadores, Primeros pasos y los avisos. Los {placeholders} los rellena Go con datos.Textof.
export const CLAVES_INICIO: string[] = [
  'inicio.direccion', 'inicio.red', 'inicio.redTexto', 'inicio.detalles', 'inicio.primerosPasos', 'inicio.guiaProgreso',
  'inicio.paso1', 'inicio.paso1Detalle', 'inicio.comprobando', 'inicio.paso2', 'inicio.paso2Detalle', 'inicio.paso3', 'inicio.paso3Detalle',
  'inicio.paso4', 'inicio.todoListo', 'inicio.todoListoTexto',
  'inicio.tablero', 'inicio.completados', 'inicio.completadosDetalle', 'inicio.enCurso', 'inicio.enCursoDetalle', 'inicio.conError', 'inicio.conErrorDetalle',
  'inicio.espaciosConProblema', 'inicio.espaciosConProblemaUno', 'inicio.verEspacios', 'claves.sugerencia', 'claves.crearApi',
  'conversor.falta', 'conversor.error', 'conversor.redEmpresa', 'conversor.preparar', 'conversor.irAjustes',
  'comun.ultimoDia', 'comun.ultimosDias', 'mapa.descripcion', 'mapa.descripcionUno', 'mapa.principal',
  'vivo.etapaCola', 'vivo.etapaPreparacion', 'vivo.etapaEjecucion', 'vivo.etapaValidacion', 'vivo.etapaLlegada', 'vivo.etapaFallida', 'vivo.etapaCancelada',
  'vivo.enCurso', 'vivo.enCursoUno', 'vivo.hace',
  'inicio.recientes', 'inicio.verHistorial', 'inicio.sinTrabajos', 'trabajo.entradaEliminada',
  'estado.en_cola', 'estado.preparando_entrada', 'estado.esperando_agente', 'estado.ejecutando', 'estado.validando_salida',
  'estado.cancelacion_solicitada', 'estado.completado', 'estado.fallido', 'estado.cancelado', 'estado.interrumpido',
]

// CLAVES_HISTORIAL son los textos del Historial que la terminal repite tal cual: filtros, orígenes, el detalle de una petición,
// su registro, el esfuerzo, la ruta y las confirmaciones. Los títulos de error (error.*) entran todos, en textosDe. Los
// {placeholders} los rellena Go con datos.Textof.
export const CLAVES_HISTORIAL: string[] = [
  'historial.todos', 'historial.sinFiltro', 'historial.vacio', 'historial.quitarFiltros', 'historial.irProbar', 'historial.estado',
  'historial.solicitud', 'historial.agenteModelo', 'historial.api', 'historial.tokens', 'historial.expirada', 'historial.sinRespuesta',
  'historial.todaviaEnCurso', 'historial.coste', 'historial.duracion', 'historial.identificador', 'historial.reintentar',
  'historial.eliminarTitulo', 'historial.eliminarTexto', 'historial.vaciar', 'historial.vaciarTitulo', 'historial.vaciarTexto',
  'historial.vaciado', 'historial.origen', 'historial.origenPrincipal', 'historial.todosOrigenes', 'historial.origenProbar',
  'historial.origenTerminal', 'historial.origenCli', 'historial.origenApp', 'historial.grupo', 'historial.mostrando', 'historial.mostrandoUno',
  'historial.verMas', 'historial.esfuerzo',
  'ejecutar.respuesta', 'ejecutar.corregir', 'historial.cancelar', 'comun.eliminar',
  'ruta.cola', 'ruta.preparacion', 'ruta.ejecucion', 'ruta.validacion', 'ruta.llegada',
  'registro.titulo', 'registro.noDisponible', 'registro.aviso', 'registro.delAgente', 'registro.entrada', 'registro.contexto',
  'registro.correccion', 'registro.validacion', 'registro.validacionMas',
  'esfuerzo.agente', 'esfuerzo.bajo', 'esfuerzo.medio', 'esfuerzo.alto', 'esfuerzo.muy_alto', 'esfuerzo.maximo', 'esfuerzo.delAgente',
]

// CLAVES_EJECUTAR son los textos de Probar que la terminal repite tal cual: la receta, el mensaje y los archivos, el esfuerzo,
// la respuesta, «Ver como código» (también los comentarios de sus ejemplos), «Guardar como API» y las pruebas recientes. Los
// {placeholders} los rellena Go con datos.Textof.
export const CLAVES_EJECUTAR: string[] = [
  'ejecutar.receta', 'ejecutar.mensaje', 'ejecutar.adjuntar', 'ejecutar.adjuntarLimites', 'ejecutar.formatos',
  'ejecutar.perfilTrabajo', 'ejecutar.perfilHint', 'ejecutar.tiempo', 'ejecutar.tiempoHint', 'ejecutar.enviar', 'ejecutar.vacio',
  'ejecutar.ejemplos', 'ejecutar.ejemplo1', 'ejecutar.ejemplo2', 'ejecutar.ejemplo3', 'ejecutar.nueva', 'ejecutar.recientes',
  'ejecutar.verTodo', 'ejecutar.sinRecientes', 'ejecutar.usarDeNuevo', 'ejecutar.sinAdjuntos', 'ejecutar.corregirTexto',
  'ejecutar.respuestaRecibida', 'ejecutar.guardarApi', 'ejecutar.actualizarApi', 'ejecutar.editando', 'ejecutar.dejarEditar',
  'ejecutar.guardarTexto', 'ejecutar.apiCreada', 'ejecutar.apiCreadaTexto', 'ejecutar.apiActualizada', 'ejecutar.avanzadasTexto',
  'receta.opciones', 'receta.ejemplo', 'comun.opcionesAvanzadas', 'receta.instruccionesEjemplo', 'ejecutar.placeholder',
  'receta.instrucciones', 'receta.instruccionesHint', 'receta.modelo', 'receta.forzar', 'receta.respaldo', 'receta.respaldoHint',
  'receta.salidaEstructurada', 'receta.salidaEstructuradaTexto', 'receta.plantillaCorta', 'receta.plantillaReemplazar',
  'receta.plantillaReemplazarTexto', 'receta.esquema', 'receta.esquemaHint', 'receta.esquemaInvalido', 'receta.esquemaVacio',
  'receta.reintentos', 'receta.reintentosHint', 'receta.reintentos0', 'receta.reintentosN', 'receta.reintentosNUno',
  'esfuerzo.titulo', 'esfuerzo.hint', 'esfuerzo.agenteCon', 'esfuerzo.espacioCon', 'esfuerzo.noAdmite', 'modelo.esPredeterminado',
  'codigo.boton', 'codigo.titulo', 'codigo.texto', 'codigo.tuClave', 'codigo.recetaEnApi',
  'ejemplos.nativa', 'ejemplos.subir', 'ejemplos.jq', 'ejemplos.crear', 'ejemplos.seguir', 'ejemplos.seguirCurl', 'ejemplos.resultado',
  'ejemplos.mensaje', 'conectar.hola', 'conectar.urlOpenai', 'conectar.clave',
  'puntos.nombre', 'puntos.nombreHint', 'puntos.nombreRegla', 'puntos.grupo', 'puntos.grupoHint', 'puntos.claveUnaVez',
  'comun.tokens',
  // El editor de campos de «Respuesta con forma» (CamposEsquema.vue).
  'receta.modo', 'receta.modoCampos', 'receta.modoJson', 'receta.esquemaAvanzado',
  'campos.nombre', 'campos.tipo', 'campos.quitar', 'campos.descripcion', 'campos.descripcionEjemplo', 'campos.obligatorio',
  'campos.opciones', 'campos.elemento', 'campos.minimoElementos', 'campos.maximoElementos', 'campos.minimo', 'campos.maximo',
  'campos.anadir', ...['texto', 'numero', 'entero', 'si_no', 'fecha', 'opcion', 'lista', 'objeto'].map((t) => 'campos.tipo.' + t),
  ...['texto', 'numero', 'objeto'].map((e) => 'campos.elemento.' + e),
  ...['sin_nombre', 'repetido', 'sin_opciones', 'rango', 'sin_campos'].map((p) => 'campos.problema.' + p),
]

// CLAVES_APIS son los textos de Mis APIs que la terminal repite tal cual: las pestañas, la lista con sus filtros, el detalle
// de una API, su formulario, la clave del espacio, el presupuesto y los avisos al cerrar. Los {placeholders} los rellena Go
// con datos.Textof.
export const CLAVES_APIS: string[] = [
  'conectar.subtitulo', 'conectar.directa', 'conectar.directaTexto', 'conectar.urlAnthropic', 'conectar.apiPropia',
  'conectar.mostrarClave', 'conectar.ocultarClave', 'conectar.regenerarClave', 'conectar.regenerarTitulo', 'conectar.regenerarTexto',
  'conectar.claveRegenerada', 'conectar.modeloAntes', 'conectar.modeloDespues', 'conectar.perfiles', 'guia.titulo',
  'comun.errorCarga', 'comun.reintentar', 'modelo.esPredeterminado', 'modelo.libreRegla', 'puntos.permitidosHint',
  'puntos.filtroCaracteristicas', 'puntos.carJson', 'puntos.carRespaldo', 'puntos.carPresupuesto', 'puntos.carInstrucciones',
  'puntos.carForzado', 'puntos.carRestringida',
  'puntos.pestana', 'puntos.principal', 'puntos.activo', 'puntos.desactivado', 'puntos.sinGrupo', 'puntos.vacio', 'puntos.verClave',
  'puntos.nuevo', 'puntos.nuevoTexto', 'puntos.sinResultados', 'puntos.limpiarFiltros', 'puntos.buscarApi',
  'puntos.filtroGrupo', 'puntos.todosGrupos', 'puntos.filtroEstado', 'puntos.estadoTodas', 'puntos.estadoActivas',
  'puntos.estadoDetenidas', 'puntos.estadoError', 'puntos.filtroAgente', 'puntos.agentesTodos', 'puntos.agentePredeterminado',
  'puntos.filtroContrato', 'puntos.contratosTodos', 'puntos.filtroUso', 'puntos.usoTodas', 'puntos.usoCon', 'puntos.usoSin',
  'puntos.orden', 'puntos.ordenNombre', 'puntos.ordenUso', 'puntos.ordenUltima',
  'puntos.uso', 'puntos.usoUno', 'puntos.usoErrores', 'puntos.usoUltima', 'puntos.sinUso',
  'puntos.listo', 'puntos.conectar', 'puntos.conectarTexto', 'puntos.claveOculta', 'puntos.activaParaConectar',
  'puntos.regenerarClave', 'puntos.regenerarTitulo', 'puntos.regenerarTexto', 'puntos.ajustes', 'puntos.ajustesDe',
  'puntos.ajustesGuardados', 'puntos.abrirProbar', 'puntos.verPeticiones', 'puntos.accionActivar', 'puntos.accionDesactivar',
  'puntos.eliminarTitulo', 'puntos.eliminarTexto', 'puntos.direccion', 'puntos.ruta', 'puntos.puertoPropio', 'puntos.puerto',
  'puntos.puertoRegla', 'puntos.compatibilidad', 'puntos.permitidos', 'puntos.alias', 'puntos.aliasInvalido', 'contrato.nativa',
  'presupuesto.diario', 'presupuesto.mensual', 'presupuesto.peticiones', 'presupuesto.tokens', 'presupuesto.usd',
  'presupuesto.reinicios', 'presupuesto.reiniciar', 'presupuesto.reiniciarTitulo', 'presupuesto.reiniciarTexto',
  'presupuesto.reiniciarEspacio', 'presupuesto.reiniciarEspacioTexto', 'presupuesto.reiniciado', 'presupuesto.espacio',
  'presupuesto.espacioTexto', 'presupuesto.editorTexto',
  'cierre.titulo', 'cierre.texto', 'cierre.claveTitulo', 'cierre.clave',
]

// CLAVES_ESPACIOS son los textos de Espacios que la terminal repite tal cual: estados, etiquetas, acciones y sus
// confirmaciones, y el diálogo de abrir un proyecto con la pregunta de confianza. Los {placeholders} los rellena Go con
// datos.Textof.
export const CLAVES_ESPACIOS: string[] = [
  'espacios.subtitulo', 'espacios.actualizar', 'espacios.vacio', 'espacios.estadoActivo', 'espacios.estadoDetenido',
  'espacios.estadoNoEncontrado', 'espacios.confiado', 'espacios.sinConfiar', 'espacios.actual', 'espacios.apis', 'espacios.apisUno',
  'espacios.peticiones', 'espacios.peticionesUno', 'espacios.ultimoUso', 'espacios.abrir', 'espacios.detener', 'espacios.activar',
  'espacios.buscar', 'espacios.olvidar', 'espacios.confiar', 'espacios.dejarDeConfiar', 'espacios.detenerTitulo',
  'espacios.detenerTexto', 'espacios.confiarTitulo', 'espacios.confiarClaves', 'espacios.confiarTexto',
  'espacios.dejarDeConfiarTitulo', 'espacios.dejarDeConfiarTexto', 'espacios.buscarTitulo', 'espacios.buscarTexto',
  'espacios.olvidarTitulo', 'espacios.olvidarTexto', 'espacios.olvidado', 'app.sinProyecto',
  'proyecto.abrir', 'proyecto.titulo', 'proyecto.texto', 'proyecto.ruta', 'proyecto.rutaHint', 'proyecto.rutaRegla',
  'proyecto.confiarTitulo', 'proyecto.movido', 'proyecto.confiarTexto', 'proyecto.sinClaves', 'proyecto.confiarYAbrir',
  'proyecto.abrirSinConfiar', 'proyecto.dentro', 'proyecto.abrirEse', 'proyecto.crearAqui', 'proyecto.abierto',
]

// CLAVES_AJUSTES son los textos de Ajustes que la terminal repite tal cual: las pestañas, la herencia, el modelo
// predeterminado, el historial, borrar datos, los valores técnicos, el puerto, el conversor, el arranque, las
// actualizaciones con sus novedades y su avance, y Acerca de. Los {placeholders} los rellena Go con datos.Textof.
export const CLAVES_AJUSTES: string[] = [
  'config.pestanaEspacio', 'config.pestanaTramtai', 'config.avisoGlobal', 'config.avisoProyecto', 'config.avisoTramtai',
  'config.valoresTecnicos', 'config.valoresTexto', 'config.buscar', 'config.sinCoincidencias',
  'herencia.propio', 'herencia.global', 'herencia.volver', 'herencia.vuelto',
  ...['interno', 'global', 'proyecto', 'local', 'entorno'].map((o) => 'origen.' + o),
  'origen.internoDetalle', 'origen.proyectoDetalle', 'origen.localDetalle', 'origen.entornoDetalle',
  ...['servicio', 'predeterminados', 'modelos', 'agentes', 'concurrencia', 'retencion', 'archivos', 'compatibilidad', 'otros'].map((s) => 'seccion.' + s),
  'predeterminado.titulo', 'predeterminado.texto', 'predeterminado.proyecto', 'predeterminado.guardado', 'predeterminado.esfuerzoGuardado',
  'historialAjustes.titulo', 'historialAjustes.texto', 'historialAjustes.respuestas', 'historialAjustes.respuestasHint',
  'historialAjustes.registro', 'historialAjustes.registroHint', 'historialAjustes.registroMenor', 'historialAjustes.maximo',
  'historialAjustes.maximoHint', 'historialAjustes.uso', 'historialAjustes.usoUno', 'historialAjustes.guardado', 'historialAjustes.rango',
  'datos.titulo', 'datos.texto', 'datos.historial', 'datos.historialDetalle', 'datos.historialDetalleUno', 'datos.puntos',
  'datos.puntosDetalle', 'datos.puntosDetalleUno', 'datos.boton', 'datos.confirmar', 'datos.hecho',
  'puerto.titulo', 'puerto.texto', 'puerto.actual', 'puerto.distinto', 'puerto.campo', 'puerto.regla', 'puerto.automatico', 'puerto.guardado',
  'puerto.reiniciarTitulo', 'puerto.reiniciarTexto', 'puerto.reiniciarAhora',
  'conversor.titulo', 'conversor.texto', 'conversor.listo', 'conversor.listoSinVersion', 'conversor.ultimoError', 'conversor.preparando',
  'conversor.prepararDeNuevo', 'conversor.alIniciar', 'conversor.preparandoNuevo',
  ...['python', 'paquetes', 'comprobacion'].map((e) => 'conversor.etapa.' + e),
  ...['preparado', 'paquete', 'sistema', 'configurado'].map((o) => 'conversor.origen.' + o),
  'arranque.titulo', 'arranque.texto', 'arranque.activado', 'arranque.desactivado',
  'actualizacion.boton', 'actualizacion.titulo', 'actualizacion.detalle', 'actualizacion.comoInstala', 'actualizacion.manual',
  'actualizacion.novedadesTitulo', 'actualizacion.novedadesVarias', 'actualizacion.novedadesRecientes', 'actualizacion.instalar',
  'actualizacion.ajustesTitulo', 'actualizacion.ajustesTexto', 'actualizacion.alDia', 'actualizacion.hayNueva', 'actualizacion.buscarAhora',
  'actualizacion.progresoTitulo', 'actualizacion.descargando', 'actualizacion.descargandoSinTotal', 'actualizacion.verificando',
  'actualizacion.instalando', 'actualizacion.reiniciaraEn', 'actualizacion.reiniciarAhora', 'actualizacion.cerrara', 'actualizacion.fallo',
  'actualizacion.reintentar', 'actualizacion.comprobado', 'actualizacion.falloConsulta', 'actualizacion.desactivada',
  'acerca.titulo', 'acerca.sinComprobar', 'acerca.desarrollado', 'acerca.alDia', 'acerca.hayNueva', 'acerca.sinBuscar', 'acerca.sitio',
  'acerca.repositorio', 'acerca.problema', 'acerca.licencia', 'acerca.terminos', 'acerca.detalles', 'acerca.sinDatos', 'acerca.version',
  'acerca.portable', 'acerca.commit', 'acerca.modificado', 'acerca.sistema', 'acerca.servicio', 'acerca.desde', 'acerca.claves',
  'acerca.datos', 'acerca.configuracion', 'acerca.registro', 'acerca.copiar', 'acerca.copiarTexto',
  'comun.guardar', 'comun.cancelar',
]

// CLAVES_TEXTOS son, además de las menu.* y las error.*, los textos que la terminal toma de la Web UI: los de la Ayuda de la
// interfaz, los de Agentes, los de Inicio, los del Historial, los de Probar, los de Mis APIs, los de Espacios y los de Ajustes.
export const CLAVES_TEXTOS: string[] = [...CLAVES_UI, ...CLAVES_AGENTES, ...CLAVES_INICIO, ...CLAVES_HISTORIAL, ...CLAVES_EJECUTAR, ...CLAVES_APIS,
  ...CLAVES_ESPACIOS, ...CLAVES_AJUSTES]

// plano deja un texto sin marcas: sin ** y con los <marcadores> entre corchetes.
const plano = (s: string) => s.replace(/\*\*/g, '').replace(/<([^<>]*)>/g, '[$1]')

function bloqueDe(b: BloqueWeb, tx: (k: string) => string, url: string): Bloque {
  switch (b.tipo) {
    case 'nota': return { tipo: 'texto', lineas: [tx(b.texto)] }
    case 'pasos': return { tipo: 'pasos', lineas: b.pasos.map(tx) }
    case 'tabla': return { tipo: 'tabla', lineas: b.columnas.map(tx), filas: b.filas.map((f) => [tx(f.titulo), ...f.celdas.map(tx)]) }
    case 'preguntas': return { tipo: 'preguntas', pares: b.preguntas.map((p) => [tx(p.pregunta), tx(p.respuesta)]) }
    case 'errores':
      return { tipo: 'errores', pares: b.codigos.map((c) => [tx(ERRORES[c]?.titulo ?? 'error.' + c) + (c === 'sin_conexion' ? '' : ` (${c})`),
        ERRORES[c] ? tx(ERRORES[c].solucion) : '']) }
    case 'recorrido': return { tipo: 'pantalla', pares: b.pasos.map((p) => [tx(p.titulo), tx(p.texto)]) }
    case 'captura': return { tipo: 'web', titulo: tx(b.alt), url }
    // El diagrama conserva sus paradas como líneas: la terminal no lo dibuja, pero el texto no se pierde.
    case 'diagrama':
      return { tipo: 'web', url, lineas: [...b.paradas.map((p, i) => `${i + 1}. ${tx(p.titulo)}${p.nota ? ' · ' + tx(p.nota) : ''}`),
        ...(b.ramas ?? []).map((r) => `${r.desde + 1} → ${r.hacia + 1}: ${tx(r.etiqueta)}`)] }
    case 'ejemplo':
    case 'ir':
    case 'descargas': return { tipo: 'web', url }
    // Un tipo nuevo tiene que mapearse aquí: scripts/ no pasa por vue-tsc, así que el fallo es en tiempo de ejecución.
    default: {
      const nuevo: never = b
      throw new Error(`tipo de bloque sin mapear: ${(nuevo as { tipo: string }).tipo}`)
    }
  }
}

// ayudaDe convierte los temas de la Ayuda a uno de los idiomas, en el mismo orden que la app.
export function ayudaDe(temas: TemaWeb[], textos: Textos): Tema[] {
  const tx = (k: string) => plano(textos[k] ?? k)
  return temas.map((t) => {
    const url = `${URL_AYUDA}${t.id}.html`
    return { id: t.id, titulo: tx(t.titulo), ...(t.paginaApp ? { pagina: t.paginaApp } : {}), resumen: tx(t.resumen), url,
      terminal: tx(t.terminal), bloques: t.bloques.map((b) => bloqueDe(b, tx, url)) }
  })
}

// textosDe trae los nombres que la terminal comparte con la Web UI (menu.*), los títulos de error (error.*) con la
// solución de la Ayuda de cada uno (qué hacer) y los de CLAVES_TEXTOS.
export function textosDe(porIdioma: Record<Idioma, Textos>): Record<Idioma, Textos> {
  const claves = [...Object.keys(porIdioma.es).filter((k) => k.startsWith('menu.') || k.startsWith('error.')),
    ...Object.values(ERRORES).map((e) => e.solucion), ...CLAVES_TEXTOS]
  return Object.fromEntries(IDIOMAS.map((i) => [i, Object.fromEntries(claves.map((k) => [k, plano(porIdioma[i][k] ?? k)]))])) as Record<Idioma, Textos>
}

// plantillasDe da, por idioma, las plantillas de «Respuesta con forma» con su esquema JSON (el de esquemaCampos).
export function plantillasDe(plantillas: Plantilla[], porIdioma: Record<Idioma, Textos>) {
  return Object.fromEntries(IDIOMAS.map((i) => [i, plantillas.map((p) => ({
    id: p.id, nombre: porIdioma[i][p.nombre] ?? p.nombre, instrucciones: p.instrucciones(i), esquema: aEsquema(p.campos(i)),
  }))])) as Record<Idioma, { id: string; nombre: string; instrucciones: string; esquema: unknown }[]>
}

// entradaEjemplos es la prueba fija con la que se generan los ejemplos de «Ver como código»: TestEjemplosComoLaWeb
// (internal/tui/paginas/ejecutar/codigo_test.go) la reproduce en Go y compara su código con el de ejemplos.json.
export function entradaEjemplos(extraer: Plantilla) {
  return { url: 'http://127.0.0.1:17373', clave: 'tramtai_clave_de_ejemplo', modelo: 'perfil/rapido',
    solicitud: { instrucciones: extraer.instrucciones('es'),
      mensajes: [{ rol: 'usuario', contenido: 'Factura de Soporte S.L. del 2026-03-14 por 120,50 euros.' }],
      salida: { formato: 'json', esquema: aEsquema(extraer.campos('es')) } } }
}
type Entrada = ReturnType<typeof entradaEjemplos>

// EJEMPLOS son los que se guardan: la API nativa en curl, Python y JavaScript (el ejemplo node).
const EJEMPLOS = { 'curl-nativa': 'curl', 'python-nativa': 'python', 'node-nativa': 'node' }

export interface Fuente {
  temas: TemaWeb[]
  plantillas: Plantilla[]
  es: Textos
  en: Textos
  // ejemplos da el código de la entrada, en español, por cada ejemplo de ejemplos().
  ejemplos: (entrada: Entrada) => { id: string; codigo: string }[]
}

// CAMPOS_EJEMPLO son los límites de cada tipo para «Ver un ejemplo de respuesta»: números con mínimo o solo máximo, el
// entero con mínimo decimal, la opción vacía, listas con mínimo o máximo 0 y el objeto vacío. Con los campos de las
// plantillas, TestEjemploComoLaWeb (internal/tui/paginas/ejecutar) los pasa por ejemploDe en Go y compara con la Web UI.
export const CAMPOS_EJEMPLO: Campo[] = [
  { nombre: 't', tipo: 'texto', obligatorio: true, descripcion: '' },
  { nombre: 'f', tipo: 'fecha', obligatorio: true, descripcion: '' },
  { nombre: 'total', tipo: 'numero', obligatorio: true, descripcion: '', minimo: 5 },
  { nombre: 'n', tipo: 'entero', obligatorio: true, descripcion: '', maximo: -3 },
  { nombre: 'e', tipo: 'entero', obligatorio: true, descripcion: '', minimo: 2.5 },
  { nombre: 's', tipo: 'si_no', obligatorio: true, descripcion: '' },
  { nombre: 'o', tipo: 'opcion', obligatorio: true, descripcion: '', opciones: [' ', 'alta'] },
  { nombre: 'l', tipo: 'lista', obligatorio: true, descripcion: '', elemento: 'objeto', minimo: 2,
    campos: [{ nombre: 'a', tipo: 'si_no', obligatorio: true, descripcion: '' }] },
  { nombre: 'v', tipo: 'lista', obligatorio: true, descripcion: '', elemento: 'numero', maximo: 0 },
  { nombre: 'x', tipo: 'objeto', obligatorio: true, descripcion: '' },
]

// ejemplosDeCampos son los casos de TestEjemploComoLaWeb: los campos y la muestra que da la Web UI, en JSON compacto.
export function ejemplosDeCampos(plantillas: Plantilla[]) {
  const casos = [CAMPOS_EJEMPLO, ...plantillas.map((p) => p.campos('es'))]
  return { casos: casos.map((campos) => ({ campos, ejemplo: JSON.stringify(ejemploDe(campos)) })) }
}

const json = (v: unknown) => JSON.stringify(v, null, 2) + '\n'

// generar devuelve cada archivo por su ruta relativa a internal/tui/datos/.
export function generar(f: Fuente): Map<string, string> {
  const textos = { es: f.es, en: f.en }
  const extraer = f.plantillas.find((p) => p.id === 'extraer')
  if (!extraer) throw new Error('falta la plantilla extraer')
  const entrada = entradaEjemplos(extraer)
  const lista = f.ejemplos(entrada)
  const ejemplos = Object.entries(EJEMPLOS).map(([nativo, id]) => {
    const e = lista.find((x) => x.id === nativo)
    if (!e) throw new Error(`falta el ejemplo ${nativo}`)
    return { id, codigo: e.codigo }
  })
  return new Map([
    ['ayuda.json', json({ es: ayudaDe(f.temas, f.es), en: ayudaDe(f.temas, f.en) })],
    ['plantillas.json', json(plantillasDe(f.plantillas, textos))],
    ['textos.json', json(textosDe(textos))],
    ['testdata/ejemplos.json', json({ entrada, ejemplos })],
    ['testdata/ejemplos-campos.json', json(ejemplosDeCampos(f.plantillas))],
  ])
}
