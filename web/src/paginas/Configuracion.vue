<template>
  <div class="pagina apilado">
    <CabeceraPagina tema="configuracion" :titulo="t('menu.configuracion')" :descripcion="t('config.subtitulo')" />

    <q-tabs v-model="pestana" data-recorrido="configuracion.pestanas" dense no-caps align="left" active-color="primary" indicator-color="primary" class="texto-suave">
      <q-tab name="espacio" icon="o_folder_special" :label="t('config.pestanaEspacio', { nombre: sesion?.nombre ?? '' })" />
      <q-tab name="tramtai" icon="o_settings" :label="t('config.pestanaTramtai')" />
      <q-tab v-if="servidor" name="servidor" icon="o_dns" :label="t('config.pestanaServidor')" />
      <q-tab v-if="servidor" name="seguridad" icon="o_shield" :label="t('config.pestanaSeguridad')" />
    </q-tabs>
    <!-- Servidor: solo se lee; cada dato con lo que lo cambia en el propio servidor (un cambio de red desde aquí podría
         dejar fuera a quien lo hace). -->
    <template v-if="servidor && pestana === 'servidor'">
      <div class="text-caption texto-suave">{{ t('servidor.texto') }}</div>
      <q-card data-captura="servidor">
        <q-card-section>
          <h2 class="titulo-seccion">{{ t('servidor.titulo') }}</h2>
          <div v-if="!estadoServidor" class="texto-suave text-body2 q-mt-sm" role="status">{{ t('comun.cargando') }}</div>
          <div v-else class="ficha text-body2 q-mt-sm">
            <template v-for="f in filas" :key="f.etiqueta">
              <div>{{ t(f.etiqueta) }}</div>
              <div>
                <div>{{ f.valor }}</div>
                <div class="text-caption texto-suave">
                  <template v-if="f.orden">{{ t('servidor.cambia') }} <code class="codigo">{{ f.orden }}</code></template>
                  <template v-else-if="f.clave">{{ t('servidor.enConfig') }} <code class="codigo">{{ f.clave }}</code></template>
                  <template v-else>{{ f.nota }}</template>
                </div>
              </div>
            </template>
          </div>
          <div class="text-caption texto-suave q-mt-sm">{{ t('servidor.cambiaEn') }}</div>
        </q-card-section>
      </q-card>
    </template>
    <template v-if="servidor && pestana === 'seguridad'">
      <div class="text-caption texto-suave">{{ t('config.avisoSeguridad') }}</div>
      <AjustesSeguridad data-captura="seguridad" />
    </template>
    <div v-show="pestana === 'espacio'" class="text-caption texto-suave">{{ sesion?.ambito === 'proyecto' ? t('config.avisoProyecto') : t('config.avisoGlobal') }}
      {{ t('config.comoGuarda') }}</div>
    <div v-show="pestana === 'tramtai'" class="text-caption texto-suave">{{ t('config.avisoTramtai') }} {{ t('config.comoGuarda') }}</div>

    <q-card v-show="pestana === 'espacio'" data-recorrido="configuracion.modelo">
      <q-card-section class="row items-center q-col-gutter-md">
        <div class="col-12 col-md">
          <div class="row items-start no-wrap">
            <div>
              <div class="row items-center">
                <h2 class="titulo-seccion">{{ t('predeterminado.titulo') }}</h2>
                <ProcedenciaValor :procedencia="procedenciaDe(clavesPredeterminado)" :ocupado="heredando" @volver="volverAGlobal(clavesPredeterminado)" />
              </div>
              <div class="texto-suave text-body2 q-mt-xs">{{ t('predeterminado.texto') }}</div>
              <div v-if="sesion?.ambito === 'proyecto'" class="text-caption texto-suave q-mt-xs">{{ t('predeterminado.proyecto') }}</div>
            </div>
          </div>
        </div>
        <div class="col-12 col-md-5">
          <SelectorModelo :model-value="actualPredeterminado" :opciones="opcionesModelo" :dense="false" :loading="guardandoModelo"
            :label="t('predeterminado.titulo')" @update:model-value="(id) => typeof id === 'string' && cambiarPredeterminado(id)" />
          <SelectorEsfuerzo v-if="actualPredeterminado" class="q-mt-md" :model-value="esfuerzoEspacio(catalogo)" :niveles="esfuerzosDe(catalogo, actualPredeterminado)"
            :predeterminado="catalogo.find((m) => m.predeterminado)?.esfuerzo_predeterminado" :hint="t('predeterminado.esfuerzoHint')"
            @update:model-value="cambiarEsfuerzo" />
        </div>
      </q-card-section>
    </q-card>

    <q-card v-if="sesion?.servicio" v-show="pestana === 'tramtai'">
      <q-card-section class="row items-start q-col-gutter-md">
        <div class="col-12 col-md">
          <div class="row items-start no-wrap">
            <div>
              <h2 class="titulo-seccion">{{ t('puerto.titulo') }}</h2>
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

    <q-card v-show="pestana === 'tramtai'" v-if="conversor" data-captura="conversor">
      <q-card-section class="row items-start no-wrap">
        <div class="col">
          <h2 class="titulo-seccion">{{ t('conversor.titulo') }}</h2>
          <div class="texto-suave text-body2 q-mt-xs">{{ t('conversor.texto') }}</div>
          <div class="text-body2 q-mt-sm" role="status">{{ textoConversor(conversor) }}</div>
          <div v-if="conversor.estado === 'error' && conversor.paquete" class="text-caption texto-suave q-mt-xs">{{ t('conversor.redEmpresa') }}</div>
          <q-linear-progress v-if="preparando(conversor)" :value="(conversor.progreso ?? 0) / 100" rounded class="q-mt-sm"
            :aria-label="t('conversor.progreso')" />
          <!-- Preparar de nuevo solo cambia algo si el que se usa es el preparado por Tramtai (no un paquete ni el del sistema). -->
          <div v-if="conversor.estado !== 'listo' || conversor.origen === 'preparado'" class="row items-center q-gutter-sm q-mt-xs">
            <q-btn flat dense no-caps color="primary" icon="o_download" :loading="preparando(conversor)"
              :label="conversor.estado === 'listo' ? t('conversor.prepararDeNuevo') : t('conversor.preparar')" @click="prepararConversor" />
          </div>
        </div>
        <q-toggle :model-value="conversor.preparar_al_iniciar" :disable="cambiandoConversor" :label="t('conversor.alIniciar')" left-label
          @update:model-value="cambiarAlIniciar" />
      </q-card-section>
    </q-card>

    <q-card v-show="pestana === 'tramtai'" v-if="actualizacion" data-captura="actualizaciones">
      <q-card-section class="row items-start no-wrap">
        <div class="col">
          <h2 class="titulo-seccion">{{ t('actualizacion.ajustesTitulo') }}</h2>
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
      <div v-if="conArranque(sesion)" v-show="pestana === 'tramtai'" class="col-12">
        <q-card data-captura="arranque">
          <q-card-section>
            <h2 id="titulo-arranque" class="titulo-seccion">{{ t('arranque.titulo', { sistema }) }}</h2>
            <div class="texto-suave text-body2 q-mt-xs">{{ t('arranque.texto', { sistema }) }}</div>
            <div class="column q-mt-sm" role="radiogroup" aria-labelledby="titulo-arranque">
              <div v-for="modo in modosArranque" :key="modo.valor" class="q-mb-xs">
                <q-radio :model-value="sesion?.modo_arranque ?? ''" :val="modo.valor" :label="t(modo.titulo)" :disable="cambiandoArranque"
                  :aria-describedby="'arranque-' + modo.titulo" @update:model-value="cambiarArranque" />
                <div :id="'arranque-' + modo.titulo" class="texto-suave text-caption q-ml-xl">{{ t(modo.texto) }}</div>
              </div>
            </div>
            <div class="texto-suave text-caption q-mt-sm">
              {{ t('arranque.servidor') }}: <code>tramtai-servidor instalar</code> ·
              <a class="enlace" href="https://mscottb.github.io/Tramtai/servidor.html" target="_blank" rel="noopener">{{ t('arranque.servidorGuia') }}</a>
            </div>
          </q-card-section>
        </q-card>
      </div>
      <div v-show="pestana === 'espacio'" class="col-12">
        <q-card>
          <q-card-section>
            <div class="row items-center">
              <h2 class="titulo-seccion">{{ t('historialAjustes.titulo') }}</h2>
              <ProcedenciaValor :procedencia="procedenciaDe(clavesRetencion)" :ocupado="heredando" @volver="volverAGlobal(clavesRetencion)" />
            </div>
            <div class="texto-suave text-body2 q-mt-xs">{{ t('historialAjustes.texto') }}</div>
          </q-card-section>
          <!-- Los límites se comprueban en cada campo (los mismos que dicen sus ayudas), no solo al guardar en el servicio. -->
          <q-form @submit="guardarRetencion">
          <q-card-section class="row q-col-gutter-md q-pt-none">
            <div class="col-12 col-sm-4">
              <q-input v-model.number="retencion.horas_resultado" type="number" min="0" max="720" outlined dense :label="t('historialAjustes.respuestas')" :hint="t('historialAjustes.respuestasHint')"
                :rules="[enRango(0, 720, true)]" />
            </div>
            <div class="col-12 col-sm-4">
              <q-input v-model.number="diasRegistro" type="number" min="0" max="90" step="any" outlined dense :label="t('historialAjustes.registro')" :hint="t('historialAjustes.registroHint')"
                :rules="[enRango(0, 90, false), registroCubre(() => retencion.horas_resultado)]" />
            </div>
            <div class="col-12 col-sm-4">
              <q-input v-model.number="retencion.max_trabajos" type="number" min="100" max="100000" outlined dense :label="t('historialAjustes.maximo')" :hint="t('historialAjustes.maximoHint')"
                :rules="[enRango(100, 100000, true)]" />
            </div>
          </q-card-section>
          <q-card-actions class="q-px-md q-pb-md">
            <q-btn unelevated color="primary" no-caps type="submit" :label="t('comun.guardar')" :loading="guardandoRetencion" />
            <span v-if="usoHistorial" class="text-caption texto-suave q-ml-md">{{ t('historialAjustes.uso', usoHistorial) }}</span>
          </q-card-actions>
          </q-form>
        </q-card>
      </div>
      <div v-show="pestana === 'espacio'" class="col-12">
        <q-card data-recorrido="configuracion.datos">
          <q-card-section class="row items-start no-wrap q-pb-sm">
            <div class="col">
              <h2 class="titulo-seccion">{{ t('datos.titulo') }}</h2>
              <div class="texto-suave text-body2 q-mt-xs">{{ t('datos.texto') }}</div>
            </div>
          </q-card-section>
          <q-list dense>
            <q-item tag="label">
              <q-item-section avatar><q-checkbox v-model="borrarHistorial" size="44px" :aria-label="t('datos.historial')" :disable="!terminados" /></q-item-section>
              <q-item-section>
                <q-item-label>{{ t('datos.historial') }}</q-item-label>
                <q-item-label caption>{{ t('datos.historialDetalle', { n: terminados }) }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item tag="label">
              <q-item-section avatar><q-checkbox v-model="borrarPuntos" size="44px" :aria-label="t('datos.puntos')" :disable="!puntos.length" /></q-item-section>
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

    <q-card v-show="pestana === 'espacio'">
      <q-expansion-item icon="o_code" :label="t('config.valoresTecnicos')" :caption="t('config.valoresTexto')" header-class="q-py-md">
        <q-separator />
        <q-card-section class="row items-center q-col-gutter-md">
          <!-- Leyenda de orígenes: el significado de cada uno se lee sin ratón (antes vivía en un tooltip). -->
          <div class="col-12 col-md">
            <ul class="leyenda-origenes">
              <li v-for="o in origenes" :key="o.id"><span :class="'etiqueta etiqueta--' + o.clase">{{ o.nombre }}</span>
                <span class="text-caption texto-suave" :class="{ codigo: o.id === 'global' }">{{ o.detalle }}</span></li>
            </ul>
          </div>
          <div class="col-12 col-md-4">
            <q-input v-model="filtro" dense outlined clearable debounce="150" :placeholder="t('config.buscar')" :aria-label="t('config.buscar')">
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
                <span :class="'etiqueta etiqueta--' + origen(e.origen).clase">{{ origen(e.origen).nombre }}</span>
              </q-item-section>
            </q-item>
          </q-list>
        </template>
        <EstadoVacio v-if="!cargando && !grupos.length" :titulo="t('config.sinCoincidencias')" :accion="filtro ? t('historial.quitarFiltros') : undefined"
          @accion="filtro = ''" />
      </q-expansion-item>
    </q-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { pestanaDeAncla, type PestanaAjustes } from '../pestanasAjustes'
import { estadoTerminos } from '../terminos'
import AjustesSeguridad from '../componentes/AjustesSeguridad.vue'
import { EVENTO_PASO } from '../recorridoDom'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import ProcedenciaValor from '../componentes/ProcedenciaValor.vue'
import EstadoVacio from '../componentes/EstadoVacio.vue'
import { cargarRutaConfig, rutaConfig } from '../rutaConfig'
import { guardar, leer } from '../componentes/accesos/comun'
import { heredable, procedencia, type Procedencia } from '../herencia'
import { useQuasar } from 'quasar'
import { api, avisar, avisarError, buscarActualizacionAhora, reiniciarYSeguir } from '../api/cliente'
import type { EntradaConfiguracion, EstadoPunto, Modelo } from '../api/tipos'
import { actualizacion, sesion, sistema } from '../estado'
import { esfuerzoEspacio, esfuerzosDe, nombreAgente, opcionModelo } from '../agentes'
import { enRango, fecha, nombreEsfuerzo, registroCubre, terminales } from '../formato'
import { conArranque, estadoServidor, filasServidor, modosArranque, pestanasAjustes } from '../servidor'
import { idioma, t } from '../i18n'
import type { Clave } from '../idiomas/es'
import SelectorEsfuerzo from '../componentes/SelectorEsfuerzo.vue'
import SelectorModelo from '../componentes/SelectorModelo.vue'
import { cargarConversor, conversor, prepararConversor, preparando, textoConversor } from '../conversor'

const $q = useQuasar()
const entradas = ref<EntradaConfiguracion[]>([])
// servidor: Ajustes → Seguridad (el administrador de la Web UI) solo existe en el modo servidor. Los términos ya se
// consultaron: App.vue no monta las páginas antes.
const servidor = computed(() => !!estadoTerminos.value?.servidor)
// pestana separa lo de este espacio de lo de todo Tramtai (y, en el servidor, la seguridad); se recuerda entre visitas.
const recordada = leer<PestanaAjustes>('tramtai.pestanaAjustes', 'espacio')
const pestana = ref<PestanaAjustes>(pestanasAjustes(servidor.value).includes(recordada) ? recordada : 'espacio')
// filas de Servidor, con las fechas y los nombres de los agentes de la app.
const filas = computed(() => (estadoServidor.value ? filasServidor(estadoServidor.value, fecha, nombreAgente) : []))
watch(pestana, (p) => guardar('tramtai.pestanaAjustes', p))
// El recorrido guiado avisa de cada ancla antes de buscarla: si está en la pestaña oculta, se abre esa.
function alPasoDelRecorrido(e: Event) {
  const destino = pestanaDeAncla((e as CustomEvent<{ ancla?: string }>).detail?.ancla)
  if (destino) pestana.value = destino
}
onMounted(() => window.addEventListener(EVENTO_PASO, alPasoDelRecorrido))
onUnmounted(() => window.removeEventListener(EVENTO_PASO, alPasoDelRecorrido))

// Herencia: en un proyecto cada ajuste dice si es propio o de Global, y los propios pueden volver al de Global.
const clavesPredeterminado = ['predeterminados.agente', 'predeterminados.modelo', 'predeterminados.perfil_modelo', 'predeterminados.esfuerzo']
const clavesRetencion = ['retencion.horas_resultado', 'retencion.horas_metadatos', 'retencion.max_trabajos']
const ambito = computed(() => (sesion.value?.ambito === 'proyecto' ? 'proyecto' : 'global'))
// procedenciaDe un grupo de claves: propio si alguna es propia, de Global si alguna viene de ahí.
function procedenciaDe(claves: string[]): Procedencia | undefined {
  const lista = claves.map((c) => procedencia(entradas.value, c, ambito.value))
  return lista.includes('propio') ? 'propio' : lista.includes('global') ? 'global' : undefined
}
const heredando = ref(false)
async function volverAGlobal(claves: string[]) {
  heredando.value = true
  try {
    for (const clave of claves) if (heredable(clave) && procedencia(entradas.value, clave, ambito.value) === 'propio') await api.heredar(clave)
    entradas.value = await api.configuracion()
    catalogo.value = await api.catalogo()
    await leerRetencion()
    avisar(t('herencia.vuelto'))
  } catch (e) {
    avisarError(e)
  } finally {
    heredando.value = false
  }
}

// Historial: plazos y tope de la retención; el registro se edita en días y se guarda en horas.
const retencion = reactive({ horas_resultado: 24, max_trabajos: 10000 })
const diasRegistro = ref(7)
const usoHistorial = ref<{ n: number; peticiones: string; mb: string }>()
const guardandoRetencion = ref(false)
async function leerRetencion() {
  const r = await api.retencion()
  Object.assign(retencion, { horas_resultado: r.horas_resultado, max_trabajos: r.max_trabajos })
  diasRegistro.value = Math.round((r.horas_metadatos / 24) * 100) / 100 // 30 h son 1,25 días; al guardar vuelven a 30 h
  usoHistorial.value = { n: r.uso.trabajos, peticiones: r.uso.trabajos.toLocaleString(idioma.value), mb: (r.uso.bytes / 1048576).toLocaleString(idioma.value, { maximumFractionDigits: 1 }) }
}
async function guardarRetencion() {
  guardandoRetencion.value = true
  try {
    await api.fijarRetencion({ horas_resultado: retencion.horas_resultado, horas_metadatos: Math.round(diasRegistro.value * 24), max_trabajos: retencion.max_trabajos })
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
    // Un esfuerzo que el modelo nuevo no admite (máximo con Codex, o cualquiera con un agente sin niveles) vuelve al
    // del agente: si no, se aplicaría sin verse y lo heredarían los respaldos.
    const actual = esfuerzoEspacio(catalogo.value)
    await api.fijarPredeterminado(id, actual && !esfuerzosDe(catalogo.value, id).includes(actual) ? '' : undefined)
    catalogo.value = await api.catalogo()
    const o = opcionesModelo.value.find((x) => x.value === id)
    avisar(t('predeterminado.guardado', { modelo: o?.label ?? id }))
  } catch (e) {
    avisarError(e)
  } finally {
    guardandoModelo.value = false
  }
}

// cambiarEsfuerzo guarda el esfuerzo que se usa con el modelo predeterminado ('' = el de su agente).
async function cambiarEsfuerzo(esfuerzo: string) {
  guardandoModelo.value = true
  try {
    await api.fijarPredeterminado('', esfuerzo)
    catalogo.value = await api.catalogo()
    avisar(t('predeterminado.esfuerzoGuardado', { nivel: esfuerzo ? nombreEsfuerzo(esfuerzo).toLowerCase() : t('esfuerzo.agente').toLowerCase() }))
  } catch (e) {
    avisarError(e)
  } finally {
    guardandoModelo.value = false
  }
}

// origenes: de dónde sale cada valor, con la etiqueta de la app (neutro, info o aviso) y su texto; el entorno va en aviso
// porque no se cambia desde aquí.
const origenes = computed(() => [
  { id: 'interno', nombre: t('origen.interno'), clase: 'neutro', detalle: t('origen.internoDetalle') },
  { id: 'global', nombre: t('origen.global'), clase: 'info', detalle: rutaConfig.value },
  { id: 'proyecto', nombre: t('origen.proyecto'), clase: 'info', detalle: t('origen.proyectoDetalle') },
  { id: 'local', nombre: t('origen.local'), clase: 'info', detalle: t('origen.localDetalle') },
  { id: 'entorno', nombre: t('origen.entorno'), clase: 'aviso', detalle: t('origen.entornoDetalle') },
])
const origen = (id: string) => origenes.value.find((o) => o.id === id) ?? { nombre: id, clase: 'neutro' }

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
  return c ? new Date(c).toLocaleTimeString(idioma.value, { hour: '2-digit', minute: '2-digit' }) : ''
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

async function cambiarArranque(modo: '' | 'app' | 'servicio') {
  cambiandoArranque.value = true
  try {
    await api.autoarranque(modo)
    avisar(modo === '' ? t('arranque.desactivado') : modo === 'app' ? t('arranque.activado', { sistema }) : t('arranque.activadoServicio', { sistema }))
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
  $q.dialog({ title: t('datos.titulo'), message: t('datos.confirmar', { espacio: sesion.value?.nombre || sesion.value?.instancia || '' }),
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

// cambiarAlIniciar guarda conversion.preparar_al_iniciar.
const cambiandoConversor = ref(false)
async function cambiarAlIniciar(preparar: boolean) {
  cambiandoConversor.value = true
  try {
    await api.ajustesConversor(preparar)
    if (conversor.value) conversor.value = { ...conversor.value, preparar_al_iniciar: preparar }
  } catch (e) {
    avisarError(e)
  } finally {
    cambiandoConversor.value = false
  }
}

onMounted(async () => {
  cargarConversor()
  cargarRutaConfig()
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
