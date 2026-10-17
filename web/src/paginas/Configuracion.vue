<template>
  <div class="pagina apilado">
    <CabeceraPagina :titulo="t('menu.configuracion')" :descripcion="t('config.subtitulo')" />

    <q-card>
      <q-card-section class="row items-center q-col-gutter-md">
        <div class="col-12 col-md">
          <div class="row items-start no-wrap">
            <div>
              <div class="titulo-seccion">{{ t('predeterminado.titulo') }}</div>
              <div class="texto-suave text-body2 q-mt-xs">{{ t('predeterminado.texto') }}</div>
              <div v-if="sesion?.ambito === 'proyecto'" class="text-caption texto-suave q-mt-xs">{{ t('predeterminado.proyecto') }}</div>
            </div>
          </div>
        </div>
        <div class="col-12 col-md-5">
          <SelectorModelo :model-value="actualPredeterminado" :opciones="opcionesModelo" :dense="false" :loading="guardandoModelo"
            :label="t('predeterminado.titulo')" @update:model-value="(id) => id && cambiarPredeterminado(id)" />
        </div>
      </q-card-section>
    </q-card>

    <q-card v-if="sesion?.servicio">
      <q-card-section class="row items-start q-col-gutter-md">
        <div class="col-12 col-md">
          <div class="row items-start no-wrap">
            <div>
              <div class="titulo-seccion">{{ t('puerto.titulo') }}</div>
              <div class="texto-suave text-body2 q-mt-xs">{{ t('puerto.texto') }}</div>
              <div class="text-caption q-mt-xs">{{ t('puerto.actual', { url: direccionActual }) }}</div>
              <div v-if="sesion.servicio.puerto !== sesion.servicio.puerto_configurado" class="aviso text-body2 q-mt-sm">
                {{ t('puerto.distinto', { configurado: sesion.servicio.puerto_configurado, actual: sesion.servicio.puerto }) }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-12 col-md-5">
          <div class="row q-col-gutter-sm items-start">
            <div class="col">
              <q-input v-model="puerto" outlined dense inputmode="numeric" :label="t('puerto.campo')"
                :rules="[(v: string) => (/^\d+$/.test(v) && +v >= 1024 && +v <= 65535) || t('puerto.regla')]" />
            </div>
            <div class="col-auto">
              <q-btn unelevated color="primary" no-caps :label="t('comun.guardar')" :loading="guardandoPuerto" :disable="!cambioPuerto" @click="guardarPuerto" />
            </div>
          </div>
          <q-toggle v-model="automatico" :label="t('puerto.automatico')" />
        </div>
      </q-card-section>
    </q-card>

    <q-card v-if="actualizacion">
      <q-card-section class="row items-start no-wrap">
        <div class="col">
          <div class="titulo-seccion">{{ t('actualizacion.ajustesTitulo') }}</div>
          <div class="texto-suave text-body2 q-mt-xs">{{ t('actualizacion.ajustesTexto') }}</div>
          <div class="text-body2 q-mt-sm">{{ textoActualizacion }}</div>
          <div class="row items-center q-gutter-sm q-mt-xs">
            <q-btn flat dense no-caps color="primary" icon="o_refresh" :label="t('actualizacion.buscarAhora')" :loading="buscandoAhora"
              @click="buscarAhora" />
            <span v-if="comprobado" class="texto-suave text-caption">{{ t('actualizacion.comprobado', { hora: comprobado }) }}</span>
          </div>
        </div>
        <q-toggle :model-value="actualizacion.buscar" :disable="!sesion?.local || cambiandoBusqueda" :aria-label="t('actualizacion.ajustesTitulo')"
          @update:model-value="cambiarBusqueda" />
      </q-card-section>
    </q-card>

    <div class="row q-col-gutter-md">
      <div v-if="sesion?.autoarranque !== undefined" class="col-12">
        <q-card>
          <q-card-section class="row items-start no-wrap">
            <div class="col">
              <div class="titulo-seccion">{{ t('arranque.titulo', { sistema }) }}</div>
              <div class="texto-suave text-body2 q-mt-xs">{{ t('arranque.texto', { sistema }) }}</div>
            </div>
            <q-toggle :model-value="sesion.autoarranque" :disable="cambiandoArranque" :aria-label="t('arranque.titulo', { sistema })" @update:model-value="cambiarArranque" />
          </q-card-section>
        </q-card>
      </div>
      <div class="col-12">
        <q-card>
          <q-card-section>
            <div class="titulo-seccion">{{ t('historialAjustes.titulo') }}</div>
            <div class="texto-suave text-body2 q-mt-xs">{{ t('historialAjustes.texto') }}</div>
          </q-card-section>
          <q-card-section class="row q-col-gutter-md q-pt-none">
            <div class="col-12 col-sm-4">
              <q-input v-model.number="retencion.horas_resultado" type="number" min="0" max="720" outlined dense :label="t('historialAjustes.respuestas')" :hint="t('historialAjustes.respuestasHint')" />
            </div>
            <div class="col-12 col-sm-4">
              <q-input v-model.number="diasRegistro" type="number" min="0" max="90" outlined dense :label="t('historialAjustes.registro')" :hint="t('historialAjustes.registroHint')" />
            </div>
            <div class="col-12 col-sm-4">
              <q-input v-model.number="retencion.max_trabajos" type="number" min="100" max="100000" outlined dense :label="t('historialAjustes.maximo')" :hint="t('historialAjustes.maximoHint')" />
            </div>
          </q-card-section>
          <q-card-actions class="q-px-md q-pb-md">
            <q-btn unelevated color="primary" no-caps :label="t('comun.guardar')" :loading="guardandoRetencion" @click="guardarRetencion" />
            <span v-if="usoHistorial" class="text-caption texto-suave q-ml-md">{{ t('historialAjustes.uso', usoHistorial) }}</span>
          </q-card-actions>
        </q-card>
      </div>
      <div class="col-12">
        <q-card>
          <q-card-section class="row items-start no-wrap q-pb-sm">
            <div class="col">
              <div class="titulo-seccion">{{ t('datos.titulo') }}</div>
              <div class="texto-suave text-body2 q-mt-xs">{{ t('datos.texto') }}</div>
            </div>
          </q-card-section>
          <q-list dense>
            <q-item tag="label">
              <q-item-section avatar><q-checkbox v-model="borrarHistorial" :disable="!terminados" /></q-item-section>
              <q-item-section>
                <q-item-label>{{ t('datos.historial') }}</q-item-label>
                <q-item-label caption>{{ t('datos.historialDetalle', { n: terminados }) }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item tag="label">
              <q-item-section avatar><q-checkbox v-model="borrarPuntos" :disable="!puntos.length" /></q-item-section>
              <q-item-section>
                <q-item-label>{{ t('datos.puntos') }}</q-item-label>
                <q-item-label caption>{{ t('datos.puntosDetalle', { n: puntos.length }) }}</q-item-label>
              </q-item-section>
            </q-item>
          </q-list>
          <q-card-actions class="q-px-md q-pb-md">
            <q-btn outline color="negative" no-caps icon="o_delete" :label="t('datos.boton')" :loading="borrando"
              :disable="!(borrarHistorial && terminados) && !(borrarPuntos && puntos.length)" @click="borrar" />
          </q-card-actions>
        </q-card>
      </div>
    </div>

    <q-card>
      <q-expansion-item icon="o_code" :label="t('config.valoresTecnicos')" :caption="t('config.valoresTexto')" header-class="q-py-md">
        <q-separator />
        <q-card-section class="row items-center q-col-gutter-md">
          <div class="col-12 col-md">
            <div class="row q-gutter-xs">
              <q-chip v-for="o in origenes" :key="o.id" dense outline :color="o.color">
                {{ o.nombre }}<q-tooltip>{{ o.detalle }}</q-tooltip>
              </q-chip>
            </div>
          </div>
          <div class="col-12 col-md-4">
            <q-input v-model="filtro" dense outlined clearable debounce="150" :placeholder="t('config.buscar')">
              <template #prepend><q-icon name="search" /></template>
            </q-input>
          </div>
        </q-card-section>
        <template v-for="g in grupos" :key="g.titulo">
          <div class="q-px-md q-pt-md text-weight-medium">{{ g.titulo }}</div>
          <q-list dense separator>
            <q-item v-for="e in g.entradas" :key="e.clave" class="q-py-sm">
              <q-item-section>
                <q-item-label class="codigo">{{ e.clave }}</q-item-label>
              </q-item-section>
              <q-item-section side class="text-right" style="max-width: 55%">
                <q-item-label class="codigo" style="word-break: break-all">{{ valor(e.valor) }}</q-item-label>
              </q-item-section>
              <q-item-section side style="min-width: 90px">
                <q-chip dense outline :color="origen(e.origen).color">{{ origen(e.origen).nombre }}</q-chip>
              </q-item-section>
            </q-item>
          </q-list>
        </template>
        <div v-if="!cargando && !grupos.length" class="vacio">{{ t('config.sinCoincidencias') }}</div>
      </q-expansion-item>
    </q-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import { useQuasar } from 'quasar'
import { actualizacion, api, avisar, avisarError, buscarActualizacionAhora, opcionModelo, reiniciarYSeguir, sesion, sistema, terminales, type EntradaConfiguracion, type EstadoPunto, type Modelo } from '../api'
import { t } from '../i18n'
import type { Clave } from '../idiomas/es'
import SelectorModelo from '../componentes/SelectorModelo.vue'

const $q = useQuasar()
const entradas = ref<EntradaConfiguracion[]>([])

// Historial: plazos y tope de la retención; el registro se edita en días y se guarda en horas.
const retencion = reactive({ horas_resultado: 24, max_trabajos: 10000 })
const diasRegistro = ref(7)
const usoHistorial = ref<{ trabajos: string; mb: string }>()
const guardandoRetencion = ref(false)
async function leerRetencion() {
  const r = await api.retencion()
  Object.assign(retencion, { horas_resultado: r.horas_resultado, max_trabajos: r.max_trabajos })
  diasRegistro.value = Math.round(r.horas_metadatos / 24)
  usoHistorial.value = { trabajos: r.uso.trabajos.toLocaleString(), mb: (r.uso.bytes / 1048576).toLocaleString(undefined, { maximumFractionDigits: 1 }) }
}
async function guardarRetencion() {
  guardandoRetencion.value = true
  try {
    await api.fijarRetencion({ horas_resultado: retencion.horas_resultado, horas_metadatos: diasRegistro.value * 24, max_trabajos: retencion.max_trabajos })
    avisar(t('historialAjustes.guardado'))
    await leerRetencion()
  } catch (e) {
    avisarError(e)
  } finally {
    guardandoRetencion.value = false
  }
}
const filtro = ref<string | null>('')
const cargando = ref(true)
const cambiandoArranque = ref(false)
const terminados = ref(0)
const puntos = ref<EstadoPunto[]>([])
const borrarHistorial = ref(true)
const borrarPuntos = ref(false)
const borrando = ref(false)
const direccionActual = location.origin
const puerto = ref(String(sesion.value?.servicio?.puerto_configurado ?? ''))
const automatico = ref(sesion.value?.servicio?.puerto_automatico ?? true)
const guardandoPuerto = ref(false)
const cambioPuerto = computed(() => puerto.value !== String(sesion.value?.servicio?.puerto_configurado) || automatico.value !== sesion.value?.servicio?.puerto_automatico)

// guardarPuerto lo guarda si está libre (lo comprueba el servicio) y ofrece reiniciar para aplicarlo.
async function guardarPuerto() {
  if (!/^\d+$/.test(puerto.value) || +puerto.value < 1024 || +puerto.value > 65535) return
  guardandoPuerto.value = true
  try {
    const nueva = await api.fijarPuerto(+puerto.value, automatico.value)
    avisar(t('puerto.guardado'))
    if (nueva.servicio && nueva.servicio.puerto_configurado !== nueva.servicio.puerto && nueva.servicio.reiniciable) {
      $q.dialog({ title: t('puerto.reiniciarTitulo'), message: t('puerto.reiniciarTexto', { puerto: puerto.value }),
        cancel: { flat: true, label: t('puerto.despues'), noCaps: true }, ok: { color: 'primary', label: t('puerto.reiniciarAhora'), unelevated: true, noCaps: true } })
        .onOk(() => reiniciarYSeguir().catch(avisarError))
    }
  } catch (e) {
    avisarError(e)
  } finally {
    guardandoPuerto.value = false
  }
}
const catalogo = ref<Modelo[]>([])
const guardandoModelo = ref(false)
const opcionesModelo = computed(() => catalogo.value.map(opcionModelo))
const actualPredeterminado = computed(() => catalogo.value.find((m) => m.predeterminado)?.id ?? null)

async function cambiarPredeterminado(id: string) {
  guardandoModelo.value = true
  try {
    await api.fijarPredeterminado(id)
    catalogo.value = await api.catalogo()
    const o = opcionesModelo.value.find((x) => x.value === id)
    avisar(t('predeterminado.guardado', { modelo: o?.label ?? id }))
  } catch (e) {
    avisarError(e)
  } finally {
    guardandoModelo.value = false
  }
}

// configGlobal es la ruta habitual del archivo global en cada sistema (os.UserConfigDir del servicio).
const configGlobal = { Windows: '%APPDATA%\\tramtai\\config.yaml', macOS: '~/Library/Application Support/tramtai/config.yaml', Linux: '~/.config/tramtai/config.yaml' }[sistema]
const origenes = computed(() => [
  { id: 'interno', nombre: t('origen.interno'), color: 'grey-6', detalle: t('origen.internoDetalle') },
  { id: 'global', nombre: t('origen.global'), color: 'primary', detalle: configGlobal },
  { id: 'proyecto', nombre: t('origen.proyecto'), color: 'teal-8', detalle: t('origen.proyectoDetalle') },
  { id: 'local', nombre: t('origen.local'), color: 'indigo-6', detalle: t('origen.localDetalle') },
  { id: 'entorno', nombre: t('origen.entorno'), color: 'deep-orange-7', detalle: t('origen.entornoDetalle') },
])
const origen = (id: string) => origenes.value.find((o) => o.id === id) ?? { nombre: id, color: 'grey-6' }

const secciones: Record<string, Clave> = {
  servicio: 'seccion.servicio', predeterminados: 'seccion.predeterminados', modelos: 'seccion.modelos', adaptadores: 'seccion.agentes',
  comandos_agentes: 'seccion.agentes', concurrencia: 'seccion.concurrencia', concurrencia_servicio: 'seccion.concurrencia',
  retencion: 'seccion.retencion', conversion: 'seccion.archivos', compatibilidad: 'seccion.compatibilidad',
}

const grupos = computed(() => {
  const texto = (filtro.value ?? '').toLowerCase().trim()
  const mapa = new Map<string, EntradaConfiguracion[]>()
  for (const e of entradas.value) {
    if (texto && !e.clave.toLowerCase().includes(texto) && !valor(e.valor).toLowerCase().includes(texto)) continue
    const titulo = t(secciones[e.clave.split('.')[0]] ?? 'seccion.otros')
    mapa.set(titulo, [...(mapa.get(titulo) ?? []), e])
  }
  return [...mapa.entries()].map(([titulo, lista]) => ({ titulo, entradas: lista }))
})

function valor(v: unknown): string {
  if (v === true) return t('comun.si').toLowerCase()
  if (v === false) return t('comun.no').toLowerCase()
  return typeof v === 'object' ? JSON.stringify(v) : String(v)
}

const cambiandoBusqueda = ref(false)
const textoActualizacion = computed(() => {
  const a = actualizacion.value
  if (!a) return ''
  if (a.disponible) return t('actualizacion.hayNueva', { version: a.version })
  if (!a.buscar) return t('actualizacion.desactivada', { actual: a.actual })
  return a.fallo ? t('actualizacion.falloConsulta') : t('actualizacion.alDia', { actual: a.actual })
})
// comprobado es la hora de la última consulta a GitHub, en la hora local.
const comprobado = computed(() => {
  const c = actualizacion.value?.comprobado
  return c ? new Date(c).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : ''
})
const buscandoAhora = ref(false)
async function buscarAhora() {
  buscandoAhora.value = true
  try {
    await buscarActualizacionAhora()
  } finally {
    buscandoAhora.value = false
  }
}
async function cambiarBusqueda(buscar: boolean) {
  cambiandoBusqueda.value = true
  try {
    actualizacion.value = await api.buscarActualizaciones(buscar)
  } catch (e) {
    avisarError(e)
  } finally {
    cambiandoBusqueda.value = false
  }
}

async function cambiarArranque(activo: boolean) {
  cambiandoArranque.value = true
  try {
    await api.autoarranque(activo)
    avisar(activo ? t('arranque.activado', { sistema }) : t('arranque.desactivado'))
  } catch (e) {
    avisarError(e)
  } finally {
    cambiandoArranque.value = false
  }
}

async function contar() {
  const [instancia, lista] = await Promise.all([api.instancia(), api.puntos()])
  terminados.value = terminales.reduce((n, e) => n + (instancia.trabajos[e] ?? 0), 0)
  puntos.value = lista
}

function borrar() {
  $q.dialog({ title: t('datos.titulo'), message: t('datos.confirmar', { instancia: sesion.value?.nombre || sesion.value?.instancia || '' }),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'negative', label: t('comun.eliminar'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      borrando.value = true
      try {
        if (borrarHistorial.value && terminados.value) await api.vaciarHistorial()
        if (borrarPuntos.value) for (const p of puntos.value) await api.eliminarPunto(p.nombre)
        avisar(t('datos.hecho'))
      } catch (e) {
        avisarError(e)
      } finally {
        borrando.value = false
        contar().catch(avisarError)
      }
    })
}

onMounted(async () => {
  contar().catch(avisarError)
  leerRetencion().catch(avisarError)
  api.catalogo().then((m) => (catalogo.value = m)).catch(avisarError)
  try {
    entradas.value = await api.configuracion()
  } catch (e) {
    avisarError(e)
  } finally {
    cargando.value = false
  }
})
</script>
