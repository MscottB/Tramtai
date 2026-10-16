<template>
  <div class="pagina apilado">
    <div>
      <div class="titulo-pagina">{{ t('menu.ejecutar') }}</div>
      <div class="texto-suave">{{ t('ejecutar.subtitulo') }}</div>
    </div>

    <div class="row q-col-gutter-lg">
      <div class="col-12 col-md-5">
        <q-card>
          <q-card-section class="q-pb-sm">
            <div class="titulo-seccion">{{ t('ejecutar.receta') }}</div>
            <div class="texto-suave text-body2">{{ t('ejecutar.recetaTexto') }}</div>
          </q-card-section>
          <q-card-section v-if="editando" class="q-py-none">
            <q-banner rounded dense class="info-caja">
              <template #avatar><q-icon name="o_edit" color="primary" /></template>
              {{ t('ejecutar.editando', { nombre: editando }) }}
              <template #action><q-btn flat dense no-caps color="primary" :label="t('ejecutar.dejarEditar')" @click="editando = ''" /></template>
            </q-banner>
          </q-card-section>
          <q-card-section>
            <EditorReceta v-model="receta" :catalogo="catalogo" />
          </q-card-section>
          <q-separator />
          <q-card-section class="q-gutter-y-md">
            <div class="titulo-seccion">{{ t('ejecutar.mensajePrueba') }}</div>
            <q-input v-model="mensaje" type="textarea" outlined :label="t('ejecutar.mensaje')" :placeholder="t('ejecutar.placeholder')"
              :input-style="{ minHeight: '120px' }" autogrow @keydown.ctrl.enter="ejecutar" @keydown.meta.enter="ejecutar" />

            <q-file v-model="archivos" multiple append use-chips clearable outlined dense :label="t('ejecutar.adjuntar')"
              :hint="t('ejecutar.adjuntarHint')">
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
      </div>

      <div class="col-12 col-md-7">
        <q-card class="full-height column">
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
              <q-icon name="o_forum" size="44px" />
              <div class="q-mt-sm">{{ t('ejecutar.vacio') }}</div>
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
                <template #avatar><q-icon name="o_alt_route" color="warning" /></template>{{ a }}
              </q-banner>
              <q-banner v-if="final?.codigo_error" rounded dense class="error-caja q-mb-md">
                <template #avatar><q-icon name="o_error" /></template>
                <b>{{ tituloError(final.codigo_error) }}</b><span v-if="final.mensaje_error">: {{ final.mensaje_error }}</span>
              </q-banner>
              <pre class="salida" :class="{ json: esJSON(salida) }" aria-live="polite">{{ salida }}</pre>
              <div v-if="activo && !salida" class="row items-center texto-suave q-gutter-x-sm">
                <q-spinner-dots color="primary" size="24px" /><span>{{ estadoInfo(estado).etiqueta }}…</span>
              </div>
            </template>
          </q-card-section>
          <template v-if="final">
            <q-separator />
            <q-card-section class="metadatos">
              <Linea :agente="final.solicitud.agente" :modelo="final.modelo_efectivo || final.modelo_enviado" />
              <span v-if="final.solicitud.esfuerzo"><q-icon name="o_psychology" />{{ nombreEsfuerzo(final.solicitud.esfuerzo) }}</span>
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
import EditorReceta from '../componentes/EditorReceta.vue'
import Linea from '../componentes/Linea.vue'
import Ruta from '../componentes/Ruta.vue'
import { ajustesDeReceta, api, aplicarModelo, avisar, avisarError, configDeReceta, copiar, duracion, esJSON, esquemaDeReceta, estadoInfo, modeloInicial,
  nombreEsfuerzo, recetaDePunto, recetaInicial, recetaVacia, terminales, tituloError, trabajoSeguido, type Modelo, type Receta, type Solicitud,
  type Trabajo } from '../api'
import { t } from '../i18n'

const catalogo = ref<Modelo[]>([])
// receta es lo que define la API que se está probando; desde Agentes llega con un modelo ya elegido.
const receta = ref<Receta>({ ...recetaVacia(), modelo: modeloInicial.value || null })
modeloInicial.value = ''
// editando es la API que se abrió desde Mis APIs: Guardar la actualiza en lugar de crear otra.
const editando = ref('')
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
let fuente: EventSource | undefined

onMounted(async () => {
  catalogo.value = await api.catalogo().catch((e) => (avisarError(e), []))
  if (recetaInicial.value) {
    const nombre = recetaInicial.value
    recetaInicial.value = ''
    try {
      const p = (await api.puntos()).find((x) => x.nombre === nombre)
      if (p) {
        receta.value = recetaDePunto(p)
        editando.value = p.nombre
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
    const r = receta.value
    let s: Solicitud = { mensajes: [{ rol: 'usuario', contenido: mensaje.value }], cargas: cargas.length ? cargas : undefined,
      instrucciones: r.instrucciones.trim() || undefined, respaldo: r.respaldo.length ? r.respaldo : undefined }
    if (esquema) s.salida = { formato: 'json', esquema }
    if (perfil.value.trim()) s.perfil = perfil.value.trim()
    if (tiempo.value) s.ejecucion = { tiempo_limite_segundos: Number(tiempo.value) }
    if (r.esfuerzo) s.esfuerzo = r.esfuerzo
    s = aplicarModelo(s, r.modelo ?? '')
    seguir(await api.crear(s))
  } catch (e) {
    avisarError(e)
  } finally {
    enviando.value = false
  }
}

// seguir muestra los eventos en vivo; EventSource se reconecta solo y reanuda con Last-Event-ID.
function seguir(nuevo: Trabajo) {
  fuente?.close()
  trabajo.value = nuevo
  final.value = undefined
  estado.value = nuevo.estado
  salida.value = ''
  avisos.value = []
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
  })
  actual.addEventListener('estado', (e) => (estado.value = (e as MessageEvent).data))
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
  } catch (e) {
    avisarError(e)
  } finally {
    activo.value = false
  }
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
    const modelo = receta.value.modelo || catalogo.value.find((m) => m.predeterminado)?.id || 'claude'
    creada.value = { nombre, url: r.punto_acceso?.url ?? '', clave: r.token, modelo }
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
  } catch (e) {
    avisarError(e)
  } finally {
    guardando.value = false
  }
}
</script>
