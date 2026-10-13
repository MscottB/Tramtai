<template>
  <div class="pagina apilado">
    <div>
      <div class="titulo-pagina">{{ t('menu.ejecutar') }}</div>
      <div class="texto-suave">{{ t('ejecutar.subtitulo') }}</div>
    </div>

    <div class="row q-col-gutter-lg">
      <div class="col-12 col-md-5">
        <q-card>
          <q-card-section class="row items-center no-wrap q-pb-sm">
            <div class="icono-suave icono-suave--primary icono-suave--chico q-mr-md"><q-icon name="o_edit_note" size="20px" /></div>
            <div class="titulo-seccion">{{ t('ejecutar.tuMensaje') }}</div>
          </q-card-section>
          <q-card-section class="q-gutter-y-md q-pt-sm">
            <q-select v-model="modelo" :options="opciones" emit-value map-options outlined dense options-dense :label="t('ejecutar.agenteModelo')">
              <template #prepend><q-icon name="o_smart_toy" /></template>
              <template #option="scope">
                <q-item v-bind="scope.itemProps">
                  <q-item-section>
                    <q-item-label>{{ scope.opt.label }}</q-item-label>
                    <q-item-label caption>{{ scope.opt.caption }}</q-item-label>
                  </q-item-section>
                  <q-item-section v-if="scope.opt.predeterminado" side><span class="etiqueta etiqueta--info">{{ t('modelo.esPredeterminado') }}</span></q-item-section>
                </q-item>
              </template>
            </q-select>
            <SelectorEsfuerzo v-model="esfuerzo" :niveles="niveles" />

            <q-input v-model="mensaje" type="textarea" outlined :label="t('ejecutar.mensaje')" :placeholder="t('ejecutar.placeholder')"
              :input-style="{ minHeight: '160px' }" autogrow @keydown.ctrl.enter="ejecutar" @keydown.meta.enter="ejecutar" />

            <q-file v-model="archivos" multiple append use-chips clearable outlined dense :label="t('ejecutar.adjuntar')"
              :hint="t('ejecutar.adjuntarHint')">
              <template #prepend><q-icon name="o_attach_file" /></template>
            </q-file>

            <q-expansion-item dense icon="o_tune" :label="t('comun.opcionesAvanzadas')" header-class="q-px-none text-body2 texto-suave" class="q-mt-sm">
              <div class="q-gutter-y-md q-pt-sm">
                <div class="row q-col-gutter-sm">
                  <div class="col-7">
                    <q-input v-model="perfil" outlined dense :label="t('ejecutar.perfilTrabajo')" :hint="t('ejecutar.perfilHint')" />
                  </div>
                  <div class="col-5">
                    <q-input v-model="tiempo" outlined dense inputmode="numeric" :label="t('ejecutar.tiempo')" suffix="s" :hint="t('ejecutar.tiempoHint')"
                      :rules="[(v: string) => !v || /^\d+$/.test(v) || t('ejecutar.soloNumeros')]" />
                  </div>
                </div>
                <q-input v-model="esquema" type="textarea" autogrow outlined dense :label="t('ejecutar.esquema')"
                  input-class="codigo" placeholder='{"type":"object","properties":{"resumen":{"type":"string"}},"required":["resumen"]}'
                  :hint="t('ejecutar.esquemaHint')" />
              </div>
            </q-expansion-item>
          </q-card-section>
          <q-separator />
          <q-card-actions class="q-pa-md">
            <span class="text-caption texto-suave gt-xs">{{ t('ejecutar.atajo') }}</span>
            <q-space />
            <q-btn v-if="activo" flat color="negative" icon="o_stop_circle" :label="t('comun.cancelar')" no-caps @click="cancelar" />
            <q-btn unelevated color="primary" icon="o_send" :label="t('ejecutar.enviar')" no-caps :loading="enviando" :disable="activo || !mensaje.trim()" @click="ejecutar" />
          </q-card-actions>
        </q-card>
      </div>

      <div class="col-12 col-md-7">
        <q-card class="full-height column">
          <q-card-section class="row items-center no-wrap q-gutter-x-sm">
            <div class="icono-suave icono-suave--primary icono-suave--chico q-mr-sm"><q-icon name="o_forum" size="20px" /></div>
            <div class="titulo-seccion">{{ t('ejecutar.respuesta') }}</div>
            <q-space />
            <span v-if="trabajo" :class="estadoInfo(estado).clase"><q-icon :name="estadoInfo(estado).icono" size="14px" />{{ estadoInfo(estado).etiqueta }}</span>
            <q-btn v-if="salida" flat dense round icon="o_content_copy" :aria-label="t('ejecutar.copiarRespuesta')" @click="copiar(salida)">
              <q-tooltip>{{ t('comun.copiar') }}</q-tooltip>
            </q-btn>
            <q-btn v-if="final" flat dense no-caps color="primary" icon="o_add" :label="t('ejecutar.nueva')" @click="nuevaPrueba" />
          </q-card-section>
          <q-separator />
          <q-card-section class="salida-caja col" style="min-height: 300px">
            <div v-if="!trabajo" class="vacio">
              <q-icon name="o_forum" size="44px" />
              <div class="q-mt-sm">{{ t('ejecutar.vacio') }}</div>
              <div class="q-mt-lg text-caption">{{ t('ejecutar.ejemplos') }}</div>
              <div class="column q-gutter-sm q-mt-xs" style="max-width: 460px; margin-left: auto; margin-right: auto">
                <div v-for="e in ejemplos" :key="e" class="paso text-left row items-center no-wrap" role="button" tabindex="0"
                  @click="mensaje = e" @keyup.enter="mensaje = e">
                  <q-icon name="o_lightbulb" color="primary" size="18px" class="q-mr-sm" /><span class="text-body2" style="color: var(--texto)">{{ e }}</span>
                </div>
              </div>
            </div>
            <template v-else>
              <q-banner v-if="final?.codigo_error" rounded dense class="error-caja q-mb-md">
                <template #avatar><q-icon name="o_error" /></template>
                {{ final.mensaje_error || final.codigo_error }}
              </q-banner>
              <pre class="salida" :class="{ json: esJSON(salida) }">{{ salida }}</pre>
              <div v-if="activo && !salida" class="row items-center texto-suave q-gutter-x-sm">
                <q-spinner-dots color="primary" size="24px" /><span>{{ estadoInfo(estado).etiqueta }}…</span>
              </div>
            </template>
          </q-card-section>
          <template v-if="final">
            <q-separator />
            <q-card-section class="metadatos">
              <span><q-icon name="o_smart_toy" />{{ modeloDe(final) }}</span>
              <span v-if="final.solicitud.esfuerzo"><q-icon name="o_psychology" />{{ nombreEsfuerzo(final.solicitud.esfuerzo) }}</span>
              <span><q-icon name="o_data_usage" />{{ final.uso.tokens_entrada }} → {{ final.uso.tokens_salida }} tokens</span>
              <span v-if="final.uso.costo_usd"><q-icon name="o_payments" />US$ {{ final.uso.costo_usd.toFixed(4) }}</span>
              <span v-if="final.terminado_en"><q-icon name="o_timer" />{{ duracion(final.creado_en, final.terminado_en) }}</span>
            </q-card-section>
          </template>
        </q-card>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import SelectorEsfuerzo from '../componentes/SelectorEsfuerzo.vue'
import { api, aplicarModelo, avisarError, copiar, descripcionPredeterminado, duracion, esfuerzosDe, esJSON, estadoInfo, modeloDe, modeloInicial, nombreEsfuerzo, opcionModelo, terminales, trabajoSeguido, type Modelo, type Solicitud, type Trabajo } from '../api'
import { t } from '../i18n'

const catalogo = ref<Modelo[]>([])
const opciones = computed(() => {
  const perfiles = catalogo.value.filter((m) => m.perfil)
  const resto = catalogo.value.filter((m) => !m.perfil)
  return [{ label: t('ejecutar.predeterminado'), caption: descripcionPredeterminado(catalogo.value) || t('ejecutar.predeterminadoDetalle'), value: '', predeterminado: false },
    ...perfiles.map(opcionModelo), ...resto.map(opcionModelo)]
})
const modelo = ref(modeloInicial.value)
modeloInicial.value = ''
// esfuerzo: '' deja el del agente; si el modelo nuevo no admite el elegido, vuelve a ''.
const esfuerzo = ref('')
const niveles = computed(() => esfuerzosDe(catalogo.value, modelo.value))
watch(niveles, (n) => {
  if (!n.includes(esfuerzo.value)) esfuerzo.value = ''
})
const ejemplos = computed(() => [t('ejecutar.ejemplo1'), t('ejecutar.ejemplo2'), t('ejecutar.ejemplo3')])

// nuevaPrueba deja la página lista para otro mensaje; el trabajo anterior queda en el Historial.
function nuevaPrueba() {
  fuente?.close()
  trabajo.value = final.value = undefined
  salida.value = mensaje.value = ''
  archivos.value = null
}
const archivos = ref<File[] | null>(null)
const mensaje = ref('')
const esquema = ref('')
const perfil = ref('')
const tiempo = ref('')
const enviando = ref(false)
const trabajo = ref<Trabajo>()
const final = ref<Trabajo>()
const estado = ref('')
const salida = ref('')
const activo = ref(false)
let fuente: EventSource | undefined

onMounted(async () => {
  catalogo.value = await api.catalogo().catch((e) => (avisarError(e), []))
  if (trabajoSeguido.value) {
    const seguido = await api.trabajo(trabajoSeguido.value)
    trabajoSeguido.value = ''
    seguir(seguido)
  }
})
onUnmounted(() => fuente?.close()) // el trabajo sigue en el servicio aunque se cierre la página

async function ejecutar() {
  if (activo.value || enviando.value || !mensaje.value.trim()) return
  let esquemaJSON: unknown
  if (esquema.value.trim()) {
    try {
      esquemaJSON = JSON.parse(esquema.value)
    } catch {
      avisarError(new Error(t('ejecutar.esquemaInvalido')))
      return
    }
  }
  if (tiempo.value && !/^\d+$/.test(tiempo.value)) return
  enviando.value = true
  try {
    const cargas: string[] = []
    for (const archivo of archivos.value ?? []) cargas.push((await api.subir(archivo)).identificador_carga)
    let s: Solicitud = { mensajes: [{ rol: 'usuario', contenido: mensaje.value }], cargas: cargas.length ? cargas : undefined }
    if (esquemaJSON) s.salida = { formato: 'json', esquema: esquemaJSON }
    if (perfil.value.trim()) s.perfil = perfil.value.trim()
    if (tiempo.value) s.ejecucion = { tiempo_limite_segundos: Number(tiempo.value) }
    if (esfuerzo.value) s.esfuerzo = esfuerzo.value
    s = aplicarModelo(s, modelo.value)
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
  activo.value = !terminales.includes(nuevo.estado)
  fuente = new EventSource(`/api/v1/trabajos/${encodeURIComponent(nuevo.id)}/eventos`)
  fuente.addEventListener('texto', (e) => (salida.value += (e as MessageEvent).data))
  fuente.addEventListener('estado', (e) => (estado.value = (e as MessageEvent).data))
  fuente.addEventListener('fin', () => {
    fuente?.close()
    terminar(nuevo.id)
  })
}

async function terminar(id: string) {
  const terminado = await api.trabajo(id)
  activo.value = false
  estado.value = terminado.estado
  // Con salida estructurada el texto en vivo puede diferir del resultado validado: se muestra el resultado.
  if (terminado.estado === 'completado' && terminado.salida !== undefined && terminado.salida !== salida.value) salida.value = terminado.salida
  final.value = terminado
}

async function cancelar() {
  if (!trabajo.value) return
  try {
    await api.accion(trabajo.value.id, 'cancelar')
  } catch (e) {
    avisarError(e)
  }
}
</script>
