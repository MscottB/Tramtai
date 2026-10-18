<template>
  <div class="pagina apilado">
    <CabeceraPagina :titulo="t('menu.ejecutar')" :descripcion="t('ejecutar.subtitulo')" />

    <div class="row q-col-gutter-lg">
      <div class="col-12 col-md-5">
        <q-card>
          <q-card-section class="q-pb-sm row items-start no-wrap q-gutter-x-sm">
            <div class="col">
              <div class="titulo-seccion">{{ t('ejecutar.receta') }}</div>
              <div class="texto-suave text-body2">{{ t('ejecutar.recetaTexto') }}</div>
            </div>
            <q-btn flat dense no-caps color="primary" icon="o_code" :label="$q.screen.gt.xs ? t('codigo.boton') : undefined" :aria-label="t('codigo.boton')"
              @click="verCodigo" />
          </q-card-section>
          <q-card-section v-if="editando" class="q-py-none">
            <q-banner rounded dense class="info-caja">
              <template #avatar><q-icon name="o_edit" color="primary" /></template>
              {{ t('ejecutar.editando', { nombre: editando }) }}
              <template #action><q-btn flat dense no-caps color="primary" :label="t('ejecutar.dejarEditar')" @click="editando = ''" /></template>
            </q-banner>
          </q-card-section>
          <q-card-section class="q-pt-sm">
            <EditorReceta v-model="receta" :catalogo="catalogo" plegar-opciones />
          </q-card-section>
          <q-separator />
          <q-card-section class="q-gutter-y-md">
            <div class="titulo-seccion">{{ t('ejecutar.mensajePrueba') }}</div>
            <q-input v-model="mensaje" type="textarea" outlined :label="t('ejecutar.mensaje')" :placeholder="t('ejecutar.placeholder')"
              :input-style="{ minHeight: '120px' }" autogrow @keydown.ctrl.enter="ejecutar" @keydown.meta.enter="ejecutar" />

            <q-file v-model="archivos" multiple append use-chips clearable outlined dense :label="t('ejecutar.adjuntar')"
              :hint="limites ? t('ejecutar.adjuntarLimites', { n: limites.cargas_por_peticion, mb: limites.carga_mb, texto: limites.texto_convertido_mb }) : t('ejecutar.adjuntarHint')">
              <template #prepend><q-icon name="o_attach_file" /></template>
            </q-file>

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
            <q-btn v-else outline color="primary" icon="o_hub" :label="t('ejecutar.guardarApi')" no-caps @click="abrirGuardar" />
            <q-btn unelevated color="primary" icon="o_send" :label="t('ejecutar.enviar')" no-caps :loading="enviando" :disable="activo || !mensaje.trim()" @click="ejecutar" />
          </q-card-actions>
        </q-card>
        <!-- Las pruebas recientes van con la receta: «Usar de nuevo» la rellena. -->
        <q-card class="q-mt-md">
          <q-card-section class="row items-center no-wrap q-pb-sm">
            <div class="titulo-seccion col">{{ t('ejecutar.recientes') }}</div>
            <q-btn v-if="recientes.length" flat dense no-caps color="primary" icon-right="o_chevron_right" :label="t('ejecutar.verTodo')" @click="verTodo" />
          </q-card-section>
          <div v-if="!recientes.length" class="q-px-md q-pb-md text-body2 texto-suave">{{ t('ejecutar.sinRecientes') }}</div>
          <q-list v-else separator>
            <q-item v-for="x in recientes" :key="x.id" clickable :active="trabajo?.id === x.id" @click="seguir(x)">
              <q-item-section>
                <q-item-label lines="1">{{ mensajeDe(x) || t('trabajo.entradaEliminada') }}</q-item-label>
                <q-item-label caption lines="1">
                  <span :class="estadoInfo(x.estado).clase"><q-icon :name="estadoInfo(x.estado).icono" size="12px" />{{ estadoInfo(x.estado).etiqueta }}</span>
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
            <div class="titulo-seccion">{{ t('ejecutar.respuesta') }}</div>
            <q-space />
            <span v-if="trabajo" :class="estadoInfo(estado).clase"><q-icon :name="estadoInfo(estado).icono" size="14px" />{{ estadoInfo(estado).etiqueta }}</span>
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
                  <q-icon name="o_lightbulb" color="primary" size="18px" class="q-mr-sm" /><span class="text-body2" style="color: var(--texto)">{{ e }}</span>
                </div>
              </div>
            </div>
            <template v-else>
              <q-banner v-for="(a, i) in avisos" :key="i" rounded dense class="aviso-caja q-mb-sm">
                <template #avatar><q-icon name="o_info" color="warning" /></template>{{ a }}
              </q-banner>
              <q-banner v-if="final?.codigo_error" rounded dense class="error-caja q-mb-md">
                <template #avatar><q-icon name="o_error" /></template>
                <b>{{ tituloError(final.codigo_error) }}</b><span v-if="final.mensaje_error">: {{ final.mensaje_error }}</span>
              </q-banner>
              <div aria-live="polite"><SalidaTrabajo :texto="salida" /></div>
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
              <span v-if="esfuerzoFinal"><q-icon name="o_psychology" />{{ esfuerzoFinal }}</span>
              <span><q-icon name="o_data_usage" />{{ final.uso.tokens_entrada }} → {{ final.uso.tokens_salida }} tokens</span>
              <span v-if="final.uso.costo_usd"><q-icon name="o_payments" />US$ {{ final.uso.costo_usd.toFixed(4) }}</span>
              <span v-if="final.terminado_en"><q-icon name="o_timer" />{{ duracion(final.creado_en, final.terminado_en) }}</span>
            </q-card-section>
          </template>
        </q-card>
      </div>
    </div>

    <q-dialog v-model="dialogoGuardar">
      <q-card style="width: 520px; max-width: 95vw">
        <q-card-section>
          <div class="titulo-seccion">{{ t('ejecutar.guardarApi') }}</div>
          <div class="texto-suave text-body2">{{ t('ejecutar.guardarTexto') }}</div>
        </q-card-section>
        <q-form @submit="guardarComoApi">
          <q-card-section class="q-gutter-y-md q-pt-none">
            <q-input v-model="nueva.nombre" outlined dense autofocus :label="t('puntos.nombre')" :hint="t('puntos.nombreHint')"
              :rules="[(v: string) => /^[a-z0-9][a-z0-9_-]{0,39}$/.test(v) || t('puntos.nombreRegla')]" />
            <q-input v-model="nueva.grupo" outlined dense maxlength="40" :label="t('puntos.grupo')" :hint="t('puntos.grupoHint')" />
          </q-card-section>
          <q-card-actions align="right" class="q-pa-md">
            <q-btn v-close-popup flat no-caps :label="t('comun.cancelar')" />
            <q-btn unelevated color="primary" no-caps type="submit" :label="t('comun.crear')" :loading="guardando" />
          </q-card-actions>
        </q-form>
      </q-card>
    </q-dialog>

    <VerComoCodigo v-model="dialogoCodigo" :ejemplos="ejemplosCodigo" :aviso="editando ? t('codigo.recetaEnApi', { nombre: editando }) : undefined" />

    <!-- Al cerrarse se olvida la clave mostrada: no queda en memoria de la página. -->
    <q-dialog v-model="dialogoCreada" @hide="creada = undefined">
      <q-card style="width: 640px; max-width: 95vw">
        <q-card-section>
          <div class="titulo-seccion">{{ t('ejecutar.apiCreada', { nombre: creada?.nombre ?? '' }) }}</div>
          <div class="texto-suave text-body2">{{ t('ejecutar.apiCreadaTexto') }}</div>
          <q-banner rounded dense class="aviso-caja q-mt-sm">
            <template #avatar><q-icon name="o_lock" color="warning" /></template>{{ t('puntos.claveUnaVez') }}
          </q-banner>
        </q-card-section>
        <q-card-section class="q-gutter-y-md q-pt-none">
          <div v-for="c in camposCreada" :key="c.titulo">
            <div class="text-caption texto-suave q-mb-xs">{{ c.titulo }}</div>
            <div class="bloque-codigo" :class="{ 'sin-ajuste': c.codigo }">{{ c.valor }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar"
              :aria-label="t('comun.copiar') + ' ' + c.titulo" @click="copiar(c.valor)" /></div>
          </div>
        </q-card-section>
        <q-card-actions align="right" class="q-pa-md"><q-btn v-close-popup unelevated color="primary" no-caps :label="t('comun.hecho')" /></q-card-actions>
      </q-card>
    </q-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useQuasar } from 'quasar'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import EditorReceta from '../componentes/EditorReceta.vue'
import EstadoVacio from '../componentes/EstadoVacio.vue'
import Linea from '../componentes/Linea.vue'
import Ruta from '../componentes/Ruta.vue'
import RegistroTrabajo from '../componentes/RegistroTrabajo.vue'
import SalidaTrabajo from '../componentes/SalidaTrabajo.vue'
import VerComoCodigo from '../componentes/VerComoCodigo.vue'
import { ejemplos as ejemplosDe, type Ejemplo } from '../ejemplos'
import { ajustesDeReceta, api, aplicarModelo, avisar, avisarError, configDeReceta, copiar, duracion, esquemaDeReceta, esfuerzoUsado, estadoInfo, haceCuanto, mensajeDe, modeloDe, modeloInicial,
  origenHistorial, recetaDePunto, recetaDeSolicitud, recetaInicial, recetaVacia, terminales, tituloError, trabajoSeguido, idModeloDe, esfuerzoEspacio, type EstadoPunto, type EventoRegistro, type LimitesPeticion, type Modelo, type Receta, type Solicitud,
  type Trabajo } from '../api'
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

// limites son los topes de las cargas, para la ayuda bajo Adjuntar.
const limites = ref<LimitesPeticion>()
onMounted(async () => {
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
onUnmounted(() => fuente?.close()) // el trabajo sigue en el servicio aunque se cierre la página

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
    seguir(await api.crear(solicitudDe(cargas, esquema)))
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
  if (esquema) s.salida = { formato: 'json', esquema }
  if (perfil.value.trim()) s.perfil = perfil.value.trim()
  if (tiempo.value && /^\d+$/.test(tiempo.value)) s.ejecucion = { tiempo_limite_segundos: Number(tiempo.value) }
  if (r.esfuerzo) s.esfuerzo = r.esfuerzo
  return aplicarModelo(s, r.modelo ?? '')
}

// «Ver como código»: la prueba actual por la API propia y por OpenAI, con la URL de la API que se edita o la de la
// API principal. Al editar una API, su receta la aplica la propia API: el código solo lleva el mensaje.
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
  if (!solicitud.mensajes[0].contenido.trim()) solicitud.mensajes[0].contenido = archivos.value?.length ? t('ejemplos.mensaje') : t('conectar.hola')
  ejemplosCodigo.value = ejemplosDe({ url, modelo, contratos,
    prueba: { solicitud, archivos: (archivos.value ?? []).map((a) => a.name) } })
  dialogoCodigo.value = true
}

// seguir muestra los eventos en vivo; EventSource se reconecta solo y reanuda con Last-Event-ID.
function seguir(nuevo: Trabajo) {
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
  actual.addEventListener('intento', () => versionRegistro.value++)
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
    estado.value = terminado.estado
    // Con salida estructurada el texto en vivo puede diferir del resultado validado: se muestra el resultado.
    if (terminado.estado === 'completado' && terminado.salida !== undefined && terminado.salida !== salida.value) salida.value = terminado.salida
    final.value = terminado
    versionRegistro.value++
    cargarRecientes()
  } catch (e) {
    avisarError(e)
  } finally {
    activo.value = false
  }
}

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

// Guardar como API crea un punto de acceso con la receta tal como está en Probar.
const dialogoGuardar = ref(false)
const dialogoCreada = ref(false)
const guardando = ref(false)
const nueva = reactive({ nombre: '', grupo: '' })
const creada = ref<{ nombre: string; url: string; clave: string; modelo: string }>()

function abrirGuardar() {
  try {
    esquemaDeReceta(receta.value)
  } catch (e) {
    avisarError(e)
    return
  }
  Object.assign(nueva, { nombre: '', grupo: '' })
  dialogoGuardar.value = true
}

async function guardarComoApi() {
  guardando.value = true
  try {
    const nombre = nueva.nombre.trim()
    const r = await api.crearPunto({ nombre, grupo: nueva.grupo.trim() || undefined, ...configDeReceta(receta.value) })
    const modelo = receta.value.modelo || catalogo.value.find((m) => m.predeterminado)?.id || catalogo.value[0]?.id || ''
    creada.value = { nombre, url: r.punto_acceso?.url ?? '', clave: r.token, modelo }
    apiEditada.value = r.punto_acceso
    dialogoGuardar.value = false
    dialogoCreada.value = true
    editando.value = nombre
  } catch (e) {
    avisarError(e)
  } finally {
    guardando.value = false
  }
}

const camposCreada = computed(() => {
  const c = creada.value
  if (!c) return []
  return [
    { titulo: t('conectar.urlOpenai'), valor: c.url + '/openai/v1', codigo: false },
    { titulo: t('conectar.clave'), valor: c.clave, codigo: false },
    { titulo: 'curl', codigo: true, valor: `curl ${c.url}/openai/v1/chat/completions \\
  -H "Authorization: Bearer ${c.clave}" \\
  -H "Content-Type: application/json" \\
  -d '{"model": "${c.modelo}", "messages": [{"role": "user", "content": "${t('conectar.hola')}"}]}'` },
  ]
})

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
