<template>
  <div class="pagina apilado">
    <CabeceraPagina tema="ejecutar" :titulo="t('menu.ejecutar')" :descripcion="t('ejecutar.subtitulo')" />

    <div class="row q-col-gutter-lg">
      <div class="col-12 col-md-5">
        <q-card data-captura="receta">
          <q-card-section class="q-pb-sm row items-start no-wrap q-gutter-x-sm">
            <div class="col">
              <h2 class="titulo-seccion">{{ t('ejecutar.receta') }}</h2>
              <div class="texto-suave text-body2">{{ t('ejecutar.recetaTexto') }}</div>
            </div>
            <q-btn data-recorrido="ejecutar.codigo" flat dense no-caps color="primary" icon="o_code" :label="$q.screen.gt.xs ? t('codigo.boton') : undefined" :aria-label="t('codigo.boton')"
              @click="verCodigo" />
          </q-card-section>
          <q-card-section v-if="editando" class="q-py-none">
            <q-banner rounded dense class="info-caja">
              <template #avatar><q-icon name="o_edit" color="primary" /></template>
              {{ t('ejecutar.editando', { nombre: editando }) }}
              <template #action><q-btn flat dense no-caps color="primary" :label="t('ejecutar.dejarEditar')" @click="editando = ''" /></template>
            </q-banner>
          </q-card-section>
          <!-- el ancla del recorrido va en el selector de la receta, no en la tarjeta entera (mide más que la pantalla) -->
          <q-card-section class="q-pt-sm" data-recorrido="ejecutar.receta">
            <EditorReceta v-model="receta" :catalogo="catalogo" plegar-opciones />
          </q-card-section>
          <q-separator />
          <q-card-section class="q-gutter-y-md">
            <h2 class="titulo-seccion">{{ t('ejecutar.mensajePrueba') }}</h2>
            <!-- el ancla va en un div: en el q-input caería en el <textarea> interno y no se resaltaría el campo entero -->
            <div data-recorrido="ejecutar.mensaje">
              <q-input v-model="mensaje" type="textarea" outlined :label="t('ejecutar.mensaje')" :placeholder="t('ejecutar.placeholder')"
                :input-style="{ minHeight: '120px' }" autogrow @keydown.ctrl.enter="ejecutar" @keydown.meta.enter="ejecutar" />
            </div>

            <q-file v-model="archivos" multiple append use-chips clearable outlined dense :label="t('ejecutar.adjuntar')" :accept="aceptar(limites)"
              :hint="limites ? t('ejecutar.adjuntarLimites', { n: limites.cargas_por_peticion, mb: limites.carga_mb, texto: limites.texto_convertido_mb }) : t('ejecutar.adjuntarHint')"
              @rejected="rechazados">
              <template #prepend><q-icon name="o_attach_file" /></template>
            </q-file>
            <div v-if="listaFormatos(limites)" class="text-caption texto-suave q-mt-lg">{{ t('ejecutar.formatos', { lista: listaFormatos(limites) }) }}</div>
            <AvisoConversor v-if="avisoConversor" />

            <q-expansion-item dense icon="o_tune" :label="t('comun.opcionesAvanzadas')" header-class="q-px-none text-body2 texto-suave" class="q-mt-sm">
              <div class="q-gutter-y-md q-pt-sm">
                <div class="text-caption texto-suave">{{ t('ejecutar.avanzadasTexto') }}</div>
                <div class="row q-col-gutter-sm">
                  <div class="col-7">
                    <q-input v-model="perfil" outlined dense :label="t('ejecutar.perfilTrabajo')" :hint="t('ejecutar.perfilHint')" />
                  </div>
                  <div class="col-5">
                    <q-input v-model="tiempo" outlined dense inputmode="numeric" :label="t('ejecutar.tiempo')" suffix="s" :hint="t('ejecutar.tiempoHint')"
                      :rules="[(v: string) => !v || /^\d+$/.test(v) || t('ejecutar.soloNumeros')]" />
                  </div>
                </div>
              </div>
            </q-expansion-item>
          </q-card-section>
          <q-separator />
          <q-card-actions class="q-pa-md">
            <span class="text-caption texto-suave gt-xs">{{ t('ejecutar.atajo') }}</span>
            <q-space />
            <q-btn v-if="activo" flat color="negative" icon="o_stop_circle" :label="t('comun.cancelar')" no-caps @click="cancelar" />
            <q-btn v-if="editando" outline color="primary" icon="o_save" :label="t('ejecutar.actualizarApi', { nombre: editando })" no-caps
              :loading="guardando" @click="actualizarApi" />
            <q-btn v-else outline color="primary" icon="o_hub" :label="t('ejecutar.guardarApi')" no-caps @click="guardarApi?.abrirGuardar()" />
            <q-btn unelevated color="primary" icon="o_send" :label="t('ejecutar.enviar')" no-caps :loading="enviando" :disable="activo || !mensaje.trim() || bloqueaConversor" @click="ejecutar" />
          </q-card-actions>
        </q-card>
        <!-- Las pruebas recientes van con la receta: «Usar de nuevo» la rellena. -->
        <q-card class="q-mt-md">
          <q-card-section class="row items-center no-wrap q-pb-sm">
            <h2 class="titulo-seccion col">{{ t('ejecutar.recientes') }}</h2>
            <q-btn v-if="recientes.length" flat dense no-caps color="primary" icon-right="o_chevron_right" :label="t('ejecutar.verTodo')" @click="verTodo" />
          </q-card-section>
          <div v-if="!recientes.length" class="q-px-md q-pb-md text-body2 texto-suave">{{ t('ejecutar.sinRecientes') }}</div>
          <q-list v-else separator>
            <q-item v-for="x in recientes" :key="x.id" clickable :active="trabajo?.id === x.id" @click="seguir(x)">
              <q-item-section>
                <q-item-label lines="1">{{ mensajeDe(x) || t('trabajo.entradaEliminada') }}</q-item-label>
                <q-item-label caption lines="1">
                  <span :class="estadoInfo(x.estado).clase">{{ estadoInfo(x.estado).etiqueta }}</span>
                  {{ haceCuanto(x.creado_en) }} · {{ modeloDe(x) }}
                </q-item-label>
              </q-item-section>
              <q-item-section side>
                <q-btn flat dense round icon="o_replay" color="primary" :disable="!mensajeDe(x)" :aria-label="t('ejecutar.usarDeNuevo')" @click.stop="usarDeNuevo(x)">
                  <q-tooltip>{{ mensajeDe(x) ? t('ejecutar.usarDeNuevo') : t('trabajo.entradaEliminada') }}</q-tooltip>
                </q-btn>
              </q-item-section>
            </q-item>
          </q-list>
        </q-card>
      </div>

      <div class="col-12 col-md-7 respuesta-fija">
        <q-card class="column">
          <q-card-section class="row items-center no-wrap q-gutter-x-sm">
            <h2 class="titulo-seccion">{{ t('ejecutar.respuesta') }}</h2>
            <q-space />
            <span v-if="trabajo" :class="estadoInfo(estado).clase">{{ estadoInfo(estado).etiqueta }}</span>
            <q-btn v-if="salida" flat dense round icon="o_content_copy" :aria-label="t('ejecutar.copiarRespuesta')" @click="copiar(salida)">
              <q-tooltip>{{ t('comun.copiar') }}</q-tooltip>
            </q-btn>
            <q-btn v-if="final" flat dense no-caps color="primary" icon="o_add" :label="t('ejecutar.nueva')" @click="nuevaPrueba" />
          </q-card-section>
          <q-card-section v-if="trabajo" class="q-pt-none q-pb-md">
            <Ruta :estado="estado" />
          </q-card-section>
          <q-separator />
          <q-card-section class="salida-caja col" style="min-height: 300px">
            <div v-if="!trabajo" class="vacio">
              <EstadoVacio :titulo="t('ejecutar.vacio')" />
              <div class="q-mt-lg text-caption">{{ t('ejecutar.ejemplos') }}</div>
              <div class="column q-gutter-sm q-mt-xs" style="max-width: 460px; margin-left: auto; margin-right: auto">
                <div v-for="e in ejemplos" :key="e" class="paso text-left row items-center no-wrap" role="button" tabindex="0"
                  @click="mensaje = e" @keyup.enter="mensaje = e" @keydown.space.prevent="mensaje = e">
                  <span class="text-body2" style="color: var(--texto)">{{ e }}</span>
                </div>
              </div>
            </div>
            <template v-else>
              <q-banner v-for="(a, i) in avisos" :key="i" rounded dense class="aviso-caja q-mb-sm">
                <template #avatar><q-icon name="o_info" color="warning" /></template>{{ a }}
              </q-banner>
              <q-banner v-if="final?.codigo_error" rounded dense class="error-caja q-mb-md">
                <template #avatar><q-icon name="o_error" /></template>
                <b>{{ tituloError(final.codigo_error) }}</b><span v-if="final.mensaje_error && !problemas.length">: {{ final.mensaje_error }}</span>
                <template v-if="final.codigo_error in ERRORES" #action><BotonAyuda tema="problemas" :ancla="'e-' + final.codigo_error" que-hago /></template>
              </q-banner>
              <!-- La respuesta no cumplía la forma pedida: qué falló, la respuesta tal como llegó y «Corregir». -->
              <div v-if="problemas.length" class="q-mb-md">
                <ul class="problemas q-my-none q-pl-md">
                  <li v-for="(p, i) in problemas" :key="i">{{ p.texto }}</li>
                </ul>
                <div v-if="puedeCorregir" class="row items-center q-gutter-sm q-mt-sm">
                  <q-btn unelevated no-caps color="primary" icon="o_build" :label="t('ejecutar.corregir')" :loading="corrigiendo" @click="corregir" />
                  <span class="text-caption texto-suave col">{{ t('ejecutar.corregirTexto') }}</span>
                </div>
                <div v-if="final?.salida" class="text-caption texto-suave q-mt-md">{{ lineasMarcadas ? t('ejecutar.respuestaMarcada') : t('ejecutar.respuestaRecibida') }}</div>
              </div>
              <!-- La marca no es solo color: cada línea con problema lleva un icono con su texto para lectores de pantalla. -->
              <div v-if="lineasMarcadas" class="bloque-codigo sin-ajuste respuesta-marcada">
                <div v-for="(l, i) in lineasMarcadas" :key="i" :class="{ 'respuesta-marcada__linea': l.marcada }">{{ l.texto }}<q-icon v-if="l.marcada"
                  name="o_error_outline" color="negative" size="14px" class="q-ml-sm" role="img" :aria-label="t('ejecutar.lineaConProblema')" /></div>
              </div>
              <!-- Sin aria-live: un lector de pantalla leería cada fragmento según llega. Se anuncia solo el final. -->
              <div v-else :aria-busy="activo"><SalidaTrabajo :texto="salida" /></div>
              <span class="solo-lector" role="status">{{ final ? estadoInfo(final.estado).etiqueta : '' }}</span>
              <div v-if="activo && !salida" class="row items-center texto-suave q-gutter-x-sm">
                <q-spinner-dots color="primary" size="24px" /><span>{{ estadoInfo(estado).etiqueta }}…</span>
              </div>
            </template>
          </q-card-section>
          <template v-if="trabajo">
            <q-separator />
            <q-card-section class="q-py-xs">
              <RegistroTrabajo :trabajo="trabajo" :version="versionRegistro" @cargado="(e: EventoRegistro[]) => (eventosRegistro = e)" />
            </q-card-section>
          </template>
          <template v-if="final">
            <q-separator />
            <q-card-section class="metadatos">
              <Linea :agente="final.solicitud.agente" :modelo="final.modelo_efectivo || final.modelo_enviado" />
              <span v-if="esfuerzoFinal"><q-icon name="o_speed" />{{ esfuerzoFinal }}</span>
              <span><q-icon name="o_data_usage" />{{ final.uso.tokens_entrada }} → {{ final.uso.tokens_salida }} {{ t('comun.tokens') }}</span>
              <span v-if="final.uso.costo_usd"><q-icon name="o_payments" />US$ {{ final.uso.costo_usd.toFixed(4) }}</span>
              <span v-if="final.terminado_en"><q-icon name="o_timer" />{{ duracion(final.creado_en, final.terminado_en) }}</span>
            </q-card-section>
          </template>
        </q-card>
      </div>
    </div>

    <VerComoCodigo v-model="dialogoCodigo" :ejemplos="ejemplosCodigo" :aviso="editando ? t('codigo.recetaEnApi', { nombre: editando }) : undefined" />

    <GuardarComoApi ref="guardarApi" :receta="receta" :catalogo="catalogo" @creada="alCrear" />
  </div>
</template>

<script setup lang="ts">
import { recordarBorrador } from '../borrador'
import BotonAyuda from '../componentes/ayuda/BotonAyuda.vue'
import { ERRORES } from '../ayuda/temas/problemas'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useQuasar } from 'quasar'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import AvisoConversor from '../componentes/AvisoConversor.vue'
import { aceptar, cargarConversor, conversor, listaFormatos, necesitaConversor } from '../conversor'
import EditorReceta from '../componentes/EditorReceta.vue'
import EstadoVacio from '../componentes/EstadoVacio.vue'
import Linea from '../componentes/Linea.vue'
import Ruta from '../componentes/Ruta.vue'
import RegistroTrabajo from '../componentes/RegistroTrabajo.vue'
import SalidaTrabajo from '../componentes/SalidaTrabajo.vue'
import GuardarComoApi from '../componentes/GuardarComoApi.vue'
import VerComoCodigo from '../componentes/VerComoCodigo.vue'
import { ejemplos as ejemplosDe, type Ejemplo } from '../ejemplos'
import { marcarLineas } from '../esquemaCampos'
import { api, avisar, avisarError, copiar, tituloError } from '../api/cliente'
import type { EstadoPunto, EventoRegistro, LimitesPeticion, Modelo, Solicitud, Trabajo } from '../api/tipos'
import { modeloInicial, origenHistorial, recetaInicial, trabajoSeguido } from '../estado'
import { modeloDe, esfuerzoEspacio } from '../agentes'
import { ajustesDeReceta, aplicarModelo, esquemaDeReceta, mensajeDe, recetaDePunto, recetaDeSolicitud, recetaVacia, idModeloDe, type Receta } from '../recetas'
import { duracion, esfuerzoUsado, estadoInfo, haceCuanto, terminales } from '../formato'
import { t } from '../i18n'

const emit = defineEmits<{ ir: [pagina: string] }>()
const $q = useQuasar()

const catalogo = ref<Modelo[]>([])
// receta es lo que define la API que se está probando; desde Agentes llega con un modelo ya elegido.
const receta = ref<Receta>({ ...recetaVacia(), modelo: modeloInicial.value || null })
modeloInicial.value = ''
// editando es la API que se abrió desde Mis APIs: Guardar la actualiza en lugar de crear otra; apiEditada, cómo está
// guardada (URL, contratos y modelo) para «Ver como código».
const editando = ref('')
const apiEditada = ref<EstadoPunto>()
const ejemplos = computed(() => [t('ejecutar.ejemplo1'), t('ejecutar.ejemplo2'), t('ejecutar.ejemplo3')])

// nuevaPrueba deja la página lista para otro mensaje; la receta se conserva y el trabajo anterior queda en el Historial.
function nuevaPrueba() {
  fuente?.close()
  trabajo.value = final.value = undefined
  salida.value = mensaje.value = ''
  avisos.value = []
  archivos.value = null
}
const archivos = ref<File[] | null>(null)
const mensaje = ref('')
recordarBorrador('tramtai.borradorProbar', mensaje)
const perfil = ref('')
const tiempo = ref('')
const enviando = ref(false)
const trabajo = ref<Trabajo>()
const final = ref<Trabajo>()
const estado = ref('')
const salida = ref('')
const avisos = ref<string[]>([])
const activo = ref(false)
// versionRegistro sube con cada paso en vivo para que el registro se vuelva a pedir; eventosRegistro da el esfuerzo usado.
const versionRegistro = ref(0)
const eventosRegistro = ref<EventoRegistro[]>([])
const esfuerzoFinal = computed(() => (final.value ? esfuerzoUsado(final.value, eventosRegistro.value) : ''))
let fuente: EventSource | undefined

// avisoConversor: algún adjunto necesita el conversor y no está listo; así no se puede enviar.
const avisoConversor = computed(() => (archivos.value ?? []).some((a) => necesitaConversor(a.name)) && !!conversor.value && conversor.value.estado !== 'listo')
const bloqueaConversor = avisoConversor

// rechazados avisa de los archivos que el selector no dejó adjuntar por su formato.
function rechazados(lista: { file: File }[]) {
  avisarError(new Error(t('ejecutar.noAdmitidos', { nombres: lista.map((r) => r.file.name).join(', '), lista: listaFormatos(limites.value) })))
}

// limites son los topes de las cargas, para la ayuda bajo Adjuntar.
const limites = ref<LimitesPeticion>()
onMounted(async () => {
  cargarConversor()
  cargarRecientes()
  api.limites().then((l) => (limites.value = l)).catch(() => {})
  catalogo.value = await api.catalogo().catch((e) => (avisarError(e), []))
  if (recetaInicial.value) {
    const nombre = recetaInicial.value
    recetaInicial.value = ''
    try {
      const p = (await api.puntos()).find((x) => x.nombre === nombre)
      if (p) {
        receta.value = recetaDePunto(p)
        editando.value = p.nombre
        apiEditada.value = p
      }
    } catch (e) {
      avisarError(e)
    }
  }
  if (trabajoSeguido.value) {
    try {
      seguir(await api.trabajo(trabajoSeguido.value))
    } catch (e) {
      avisarError(e)
    } finally {
      trabajoSeguido.value = ''
    }
  }
})
// desmontado: la página se cerró mientras cargaba; seguir no debe abrir la conexión en vivo después.
let desmontado = false
onUnmounted(() => {
  desmontado = true
  fuente?.close() // el trabajo sigue en el servicio aunque se cierre la página
})

async function ejecutar() {
  if (activo.value || enviando.value || !mensaje.value.trim()) return
  let esquema: Record<string, unknown> | undefined
  try {
    esquema = esquemaDeReceta(receta.value)
  } catch (e) {
    avisarError(e)
    return
  }
  if (tiempo.value && !/^\d+$/.test(tiempo.value)) return
  enviando.value = true
  try {
    const cargas: string[] = []
    for (const archivo of archivos.value ?? []) cargas.push((await api.subir(archivo)).identificador_carga)
    const s = solicitudDe(cargas, esquema)
    // Probar conserva la respuesta que no cumple el esquema: se muestra con sus problemas y se puede corregir.
    if (s.salida) s.salida.conservar_salida_invalida = true
    seguir(await api.crear(s))
    cargarRecientes()
  } catch (e) {
    avisarError(e)
  } finally {
    enviando.value = false
  }
}

// solicitudDe arma la petición de la receta y el mensaje, tal como la envía Probar.
function solicitudDe(cargas: string[], esquema: Record<string, unknown> | undefined): Solicitud {
  const r = receta.value
  const s: Solicitud = { mensajes: [{ rol: 'usuario', contenido: mensaje.value }], cargas: cargas.length ? cargas : undefined,
    instrucciones: r.instrucciones.trim() || undefined, respaldo: r.respaldo.length ? r.respaldo : undefined }
  if (esquema) s.salida = { formato: 'json', esquema, ...(r.reintentos ? { reintentos: r.reintentos } : {}) }
  if (perfil.value.trim()) s.perfil = perfil.value.trim()
  if (tiempo.value && /^\d+$/.test(tiempo.value)) s.ejecucion = { tiempo_limite_segundos: Number(tiempo.value) }
  if (r.esfuerzo) s.esfuerzo = r.esfuerzo
  return aplicarModelo(s, r.modelo ?? '')
}

// «Ver como código»: la prueba actual por la API propia y por OpenAI, con la URL de la API que se edita o la de la
// clave del espacio. Al editar una API, su receta la aplica la propia API: el código solo lleva el mensaje.
const dialogoCodigo = ref(false)
const ejemplosCodigo = ref<Ejemplo[]>([])
async function verCodigo() {
  let esquema: Record<string, unknown> | undefined
  try {
    esquema = esquemaDeReceta(receta.value)
  } catch (e) {
    avisarError(e)
    return
  }
  const base = await api.instancia().then((i) => i.base).catch(() => location.origin)
  // Una API editada: su URL (o la de su ruta, si aún no tiene), lo que acepta y el modelo que tiene guardado, que es
  // el que usará; OpenAI y Anthropic exigen un modelo en cada petición.
  const p = editando.value ? apiEditada.value : undefined
  const url = editando.value ? p?.url || `${base}/p/${editando.value}` : base
  const contratos = p?.contratos ?? ['nativa', 'openai', 'anthropic']
  const modelo = (editando.value ? p && idModeloDe(p) : receta.value.modelo) || catalogo.value.find((m) => m.predeterminado)?.id || catalogo.value[0]?.id || ''
  let solicitud = solicitudDe([], esquema)
  if (editando.value) solicitud = { mensajes: solicitud.mensajes, perfil: solicitud.perfil, ejecucion: solicitud.ejecucion }
  else if (!receta.value.modelo && !receta.value.esfuerzo && esfuerzoEspacio(catalogo.value)) solicitud.esfuerzo = esfuerzoEspacio(catalogo.value)
  const primero = solicitud.mensajes?.[0]
  if (primero && !primero.contenido.trim()) primero.contenido = archivos.value?.length ? t('ejemplos.mensaje') : t('conectar.hola')
  ejemplosCodigo.value = ejemplosDe({ url, modelo, contratos,
    prueba: { solicitud, archivos: (archivos.value ?? []).map((a) => a.name) } })
  dialogoCodigo.value = true
}

// seguir muestra los eventos en vivo; EventSource se reconecta solo y reanuda con Last-Event-ID.
function seguir(nuevo: Trabajo) {
  if (desmontado) return
  fuente?.close()
  trabajo.value = nuevo
  final.value = undefined
  estado.value = nuevo.estado
  salida.value = ''
  avisos.value = []
  eventosRegistro.value = []
  activo.value = !terminales.includes(nuevo.estado)
  const actual = (fuente = new EventSource(`/api/v1/trabajos/${encodeURIComponent(nuevo.id)}/eventos`))
  // ultimo: el número del último evento recibido; tras reconectar se descarta lo que ya estaba.
  let ultimo = 0
  const nuevoEvento = (e: Event) => {
    const numero = Number((e as MessageEvent).lastEventId)
    if (numero && numero <= ultimo) return false
    ultimo = numero || ultimo
    return true
  }
  actual.addEventListener('texto', (e) => {
    if (nuevoEvento(e)) salida.value += (e as MessageEvent).data
  })
  // aviso: por ejemplo, que un agente llegó a su límite y sigue el del respaldo.
  actual.addEventListener('aviso', (e) => {
    if (nuevoEvento(e)) avisos.value.push((e as MessageEvent).data)
    versionRegistro.value++
  })
  actual.addEventListener('estado', (e) => {
    estado.value = (e as MessageEvent).data
    versionRegistro.value++
  })
  // Una corrección vuelve a escribir la respuesta desde cero; validacion son los problemas de la anterior.
  actual.addEventListener('intento', (e) => {
    try {
      if (nuevoEvento(e) && JSON.parse((e as MessageEvent).data).correccion) salida.value = ''
    } catch {
      // un intento sin JSON no cambia la respuesta
    }
    versionRegistro.value++
  })
  actual.addEventListener('validacion', () => versionRegistro.value++)
  actual.addEventListener('fin', () => {
    actual.close()
    terminar(nuevo.id)
  })
  // Un error con el flujo cerrado (sesión caducada, trabajo inexistente) no se reintenta: se consulta el trabajo
  // para mostrar su estado en lugar de dejar la espera abierta. Con CONNECTING, el navegador ya está reconectando.
  actual.addEventListener('error', () => {
    if (actual.readyState === EventSource.CLOSED && fuente === actual) terminar(nuevo.id)
  })
}

async function terminar(id: string) {
  try {
    const terminado = await api.trabajo(id)
    if (trabajo.value?.id !== id) return // entretanto se abrió otro trabajo: estos datos no son suyos
    estado.value = terminado.estado
    // Con salida estructurada el texto en vivo puede diferir del resultado validado: se muestra el resultado.
    if (terminado.estado === 'completado' && terminado.salida !== undefined && terminado.salida !== salida.value) salida.value = terminado.salida
    // Si no cumplía el esquema se muestra la última respuesta conservada, no el texto de todos los intentos.
    if (terminado.codigo_error === 'salida_no_cumple_esquema' && terminado.salida !== undefined) salida.value = terminado.salida
    final.value = terminado
    versionRegistro.value++
    cargarRecientes()
  } catch (e) {
    avisarError(e)
  } finally {
    if (trabajo.value?.id === id) activo.value = false
  }
}

// problemas explica por qué la respuesta no cumplía el esquema; «Corregir» la pide de nuevo al mismo agente con esos
// problemas, como un intento nuevo (solo si el trabajo conservó la respuesta).
const problemas = computed(() => (final.value?.codigo_error === 'salida_no_cumple_esquema' ? final.value.errores_salida ?? [] : []))
const puedeCorregir = computed(() => problemas.value.length > 0 && !!final.value?.salida)
const corrigiendo = ref(false)
async function corregir() {
  if (!final.value) return
  corrigiendo.value = true
  try {
    seguir(await api.corregir(final.value.id))
    cargarRecientes()
  } catch (e) {
    avisarError(e)
  } finally {
    corrigiendo.value = false
  }
}
const lineasMarcadas = computed(() => marcarLineas(salida.value, problemas.value))


// recientes son las últimas pruebas lanzadas desde Probar (las de las APIs y la terminal están en el Historial).
const recientes = ref<Trabajo[]>([])
async function cargarRecientes() {
  recientes.value = await api.trabajos(10, { origen: 'probar' }).catch(() => recientes.value)
}
function usarDeNuevo(x: Trabajo) {
  const texto = mensajeDe(x)
  if (!texto) return
  mensaje.value = texto
  receta.value = recetaDeSolicitud(x.solicitud)
  editando.value = ''
  archivos.value = null
  if (x.solicitud.cargas?.length) avisar(t('ejecutar.sinAdjuntos'))
}
function verTodo() {
  origenHistorial.value = 'o:probar'
  emit('ir', 'historial')
}

async function cancelar() {
  if (!trabajo.value) return
  try {
    await api.accion(trabajo.value.id, 'cancelar')
  } catch (e) {
    avisarError(e)
  }
}

// guardarApi crea una API con la receta tal como está en Probar; al crearla, Probar pasa a editarla.
const guardarApi = ref<InstanceType<typeof GuardarComoApi>>()
const guardando = ref(false)
function alCrear(nombre: string, p?: EstadoPunto) {
  apiEditada.value = p
  editando.value = nombre
}

async function actualizarApi() {
  guardando.value = true
  try {
    await api.ajustesPunto(editando.value, ajustesDeReceta(receta.value))
    avisar(t('ejecutar.apiActualizada', { nombre: editando.value }))
    apiEditada.value = (await api.puntos().catch(() => [])).find((x) => x.nombre === editando.value) ?? apiEditada.value
  } catch (e) {
    avisarError(e)
  } finally {
    guardando.value = false
  }
}
</script>

<style scoped>
.respuesta-marcada__linea {
  background: var(--error-suave);
  box-shadow: inset 3px 0 0 var(--error);
}
</style>
