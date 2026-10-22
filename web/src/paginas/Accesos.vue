<template>
  <div class="pagina apilado">
    <CabeceraPagina tema="conectar" :titulo="t('menu.conectar')" :descripcion="t('conectar.subtitulo')">
      <template #acciones>
        <q-btn data-recorrido="conectar.nueva" unelevated color="primary" icon="add" :label="t('puntos.nuevo')" no-caps @click="abrirNuevo" />
      </template>
    </CabeceraPagina>


    <!-- Las dos clases de clave, en tres líneas: la del espacio y las de cada app. -->
    <q-card flat bordered>
      <q-card-section class="row q-col-gutter-md">
        <div class="col-12 col-md-6" data-recorrido="conectar.clave-espacio">
          <div class="text-weight-medium">{{ t('claves.espacioTitulo', { nombre: sesion?.nombre ?? '' }) }}</div>
          <div class="text-caption texto-suave">{{ t('claves.espacioTexto') }}</div>
        </div>
        <div class="col-12 col-md-6">
          <div class="text-weight-medium">{{ t('claves.appsTitulo') }}</div>
          <div class="text-caption texto-suave">{{ t('claves.appsTexto') }}</div>
        </div>
      </q-card-section>
    </q-card>
    <q-banner v-if="sugerirApi" rounded class="aviso">{{ t('claves.sugerencia', { n: appConClave }) }}</q-banner>

    <q-card data-captura="mis-apis" data-recorrido="conectar.apis">
      <q-tabs v-model="pestana" dense no-caps align="left" active-color="primary" indicator-color="primary" class="q-px-sm" :breakpoint="0">
        <q-tab name="apis" icon="o_hub" :label="t('puntos.pestana', { n: puntos.length })" />
        <q-tab name="principal" icon="o_cable" :label="t('conectar.directa', { nombre: sesion?.nombre ?? '' })" />
      </q-tabs>
      <q-separator />
      <q-tab-panels v-model="pestana" animated keep-alive>
        <q-tab-panel name="apis" class="q-pa-none">
          <ListaApis v-model:filtro-grupo="filtroGrupo" :puntos="puntos" :uso="uso" :presupuestos="presupuestos" :opciones="opciones" :ocupado="ocupado"
            :cargando="cargandoPuntos" :error="errorPuntos" @recargar="cargarPuntos" @ver-clave="pestana = 'principal'" @abrir="abrirDetalle" @nuevo="abrirNuevo" @activar-grupo="activarGrupo" @ver-peticiones="verPeticiones" @grupos="(g) => (gruposLista = g)" />
        </q-tab-panel>
        <q-tab-panel name="principal" class="q-pa-none">
      <q-card-section class="seccion">
        <div>
          <h2 class="titulo-seccion">{{ t('conectar.directa', { nombre: sesion?.nombre ?? '' }) }}</h2>
          <div class="texto-suave text-body2">{{ t('conectar.directaTexto') }}</div>
        </div>
      </q-card-section>
      <q-card-section class="q-pt-none">
        <GuiaUrls :url="base" :contratos="contratos" @usar="usarApiPropia" />
      </q-card-section>
      <q-card-section class="row q-col-gutter-md q-pt-none">
        <div class="col-12 col-md-6">
          <div class="text-caption texto-suave q-mb-xs">{{ t('conectar.clave') }}</div>
          <div v-if="errorClave && !clave" class="row items-center q-gutter-sm" role="alert">
            <span class="text-body2">{{ t('comun.errorCarga') }}: {{ errorClave }}</span>
            <q-btn flat dense no-caps color="primary" icon="o_refresh" :label="t('comun.reintentar')" @click="cargarClave" />
          </div>
          <div v-else class="bloque-codigo bloque-codigo--dos">{{ clave ? (verClave ? clave : ocultar(clave)) : t('comun.cargando') }}
            <div class="boton-copiar row no-wrap">
              <q-btn flat dense round size="sm" :icon="verClave ? 'visibility_off' : 'visibility'" :aria-label="verClave ? t('conectar.ocultarClave') : t('conectar.mostrarClave')" @click="verClave = !verClave" />
              <q-btn flat dense round size="sm" icon="content_copy" :aria-label="t('conectar.copiarClave')" :disable="!clave" @click="copiar(clave, t('conectar.claveCopiada'))" />
            </div>
          </div>
          <!-- Regenerar invalida la clave en todas las apps: va aparte de Copiar, con su nombre escrito y en color de peligro. -->
          <q-btn v-if="clave" flat dense no-caps color="negative" icon="o_autorenew" class="q-mt-sm" :label="t('conectar.regenerarClave')"
            @click="regenerarClaveInstancia" />
        </div>
      </q-card-section>
      <q-card-section class="q-pt-none">
        <div class="text-caption texto-suave q-mb-xs">{{ t('conectar.modeloAntes') }} <span class="codigo">model</span> {{ t('conectar.modeloDespues') }}</div>
        <div v-for="g in gruposModelo" :key="g.titulo" class="row items-center q-gutter-xs q-mb-xs">
          <span class="grupo-modelo">
            <span v-if="g.agente" class="linea__chapa" :class="'linea--' + g.agente" aria-hidden="true">{{ codigoLinea(g.agente) }}</span>{{ g.titulo }}
          </span>
          <q-chip v-for="m in g.ids" :key="m" dense clickable outline color="primary" class="codigo" @click="copiar(m)">{{ m }}</q-chip>
        </div>
      </q-card-section>
      <q-separator />
      <div ref="ejemplosRef" class="q-px-md q-pt-md text-weight-medium">{{ t('conectar.ejemplos') }}</div>
      <div class="q-px-md q-pt-sm">
        <div class="desplazable-x"><q-btn-toggle v-model="lenguaje" no-caps flat dense toggle-color="primary" class="segmentado inline-flex"
          :options="ejemplos.map((e) => ({ label: e.titulo(), value: e.id }))" /></div>
      </div>
      <q-card-section>
        <div class="bloque-codigo sin-ajuste">{{ ejemplo.codigo(verClave ? clave : ocultar(clave)) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('conectar.copiarEjemplo')" @click="copiar(ejemplo.codigo(clave), t('conectar.ejemploCopiado'))" /></div>
      </q-card-section>
      <template v-if="presupuestoEspacio">
        <q-separator />
        <q-card-section>
          <div class="text-weight-medium">{{ t('presupuesto.espacio') }}</div>
          <div class="text-caption texto-suave">{{ t('presupuesto.espacioTexto') }}</div>
          <BarrasPresupuesto :estado="presupuestoEspacio" @reiniciar="reiniciarPresupuesto()" />
        </q-card-section>
      </template>
        </q-tab-panel>
      </q-tab-panels>
    </q-card>

    <FormularioApi ref="formularioApi" :catalogo="catalogo" :opciones="opciones" :grupos="gruposSugeridos" :recargar="cargarPuntos" @creada="alCrear" />

    <DetalleApi v-model="dialogoDetalle" v-model:clave-vista="claveVista" v-model:lenguaje="lenguaje" :punto-detalle="puntoDetalle" :recien-creado="recienCreado"
      :presupuestos="presupuestos" :modelo-ejemplo="modeloEjemplo" @activar="activar" @ajustes="(p) => formularioApi?.abrirAjustes(p)" @probar="abrirEnProbar"
      @ver-peticiones="verPeticiones" @reiniciar-presupuesto="reiniciarPresupuesto" @eliminar="eliminar" @regenerar-clave="regenerarClave" />
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useQuasar } from 'quasar'
import BarrasPresupuesto from '../componentes/BarrasPresupuesto.vue'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import GuiaUrls from '../componentes/GuiaUrls.vue'
import DetalleApi from '../componentes/accesos/DetalleApi.vue'
import FormularioApi from '../componentes/accesos/FormularioApi.vue'
import ListaApis from '../componentes/accesos/ListaApis.vue'
import { activo, contratos, guardar, leer } from '../componentes/accesos/comun'
import { api, avisar, avisarError, copiar, cargarDiasHistorial, mensajeError } from '../api/cliente'
import type { ConfigPunto, EstadoPresupuesto, EstadoPunto, Modelo, UsoPunto } from '../api/tipos'
import { sesion } from '../estado'
import type { Destino, IntencionDe } from '../navegacion'
import { asegurarFichas, codigoLinea, nombreAgente, ordenAgentes, opcionModelo } from '../agentes'
import { ejemplos as ejemplosDe } from '../ejemplos'
import { t } from '../i18n'

const $q = useQuasar()
const base = ref(location.origin)
// appConClave son las peticiones de apps con la clave del espacio en 24 h; sugerirApi, si el servicio aconseja darles su
// propia API.
const appConClave = ref(0)
const sugerirApi = ref(false)
const clave = ref('')
const errorClave = ref('')
const verClave = ref(false)
const catalogo = ref<Modelo[]>([])
const ids = computed(() => catalogo.value.map((m) => m.id))
const puntos = ref<EstadoPunto[]>([])
const ocupado = ref('')
const dialogoDetalle = ref(false)
const recienCreado = ref(false)
// detalle es la API abierta; claveVista, su clave recién creada o regenerada (solo se ve esta vez).
const detalle = ref('')
const claveVista = ref('')
// respaldoDetalle es la API con que se abrió el detalle, por si la lista aún no la trae (recién creada y sin recargar).
const respaldoDetalle = ref<EstadoPunto>()
const puntoDetalle = computed(() => puntos.value.find((p) => p.nombre === detalle.value) ?? respaldoDetalle.value)
// Si la API desaparece (borrada desde otro sitio), el detalle se cierra en vez de quedar vacío.
watch(puntos, (lista) => {
  if (dialogoDetalle.value && !lista.some((p) => p.nombre === detalle.value)) {
    if (respaldoDetalle.value?.nombre !== detalle.value) dialogoDetalle.value = false
  } else respaldoDetalle.value = undefined
})
function abrirDetalle(p: EstadoPunto, clave = '', creada = false) {
  respaldoDetalle.value = p
  detalle.value = p.nombre
  claveVista.value = clave
  recienCreado.value = creada
  dialogoDetalle.value = true
}
const lenguaje = ref('python')
// usarApiPropia muestra los pasos de la API propia: su ejemplo en curl, con la URL y la clave puestas.
const ejemplosRef = ref<HTMLElement>()
function usarApiPropia() {
  lenguaje.value = 'curl-nativa'
  ejemplosRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
const filtroGrupo = ref<string | null>(null) // null: todos; '': sin grupo

// pestana es la pestaña abierta: las APIs por aplicación o la clave del espacio; se recuerda en este navegador.
const pestana = ref<'apis' | 'principal'>(leer('tramtai.pestanaApis', 'apis'))
watch(pestana, (v) => guardar('tramtai.pestanaApis', v))
// gruposLista son los grupos que muestra la lista con sus filtros; si aún no se abrió su pestaña, todos los grupos.
const gruposLista = ref<string[]>()
const gruposSugeridos = computed(() => gruposLista.value ??
  [...new Set(puntos.value.map((p) => p.grupo ?? ''))].filter(Boolean).sort((a, b) => a.localeCompare(b)))

// gruposModelo agrupa los valores de model: primero los perfiles y luego cada agente en el orden fijo de la red.
const gruposModelo = computed(() => [
  { agente: '', titulo: t('conectar.perfiles'), ids: ids.value.filter((id) => id.startsWith('perfil/')) },
  ...ordenAgentes.value.map((a) => ({ agente: a, titulo: nombreAgente(a), ids: ids.value.filter((id) => id.startsWith(a + '/')) })),
].filter((g) => g.ids.length))

const modeloEjemplo = computed(() => ids.value.find((id) => id === 'perfil/rapido') ?? ids.value.find((id) => id.includes('/')) ?? ids.value[0] ?? '')

const ocultar = (c: string) => (c ? c.slice(0, 6) + '•'.repeat(18) : '')
const ejemplos = computed(() => ejemplosDe({ url: base.value, modelo: modeloEjemplo.value, contratos }))
const ejemplo = computed(() => ejemplos.value.find((e) => e.id === lenguaje.value) ?? ejemplos.value[0])

const opciones = computed(() => catalogo.value.map(opcionModelo))
const formularioApi = ref<InstanceType<typeof FormularioApi>>()

// uso son las peticiones de cada API que conserva la retención, contadas por el servicio.
const uso = ref<Record<string, UsoPunto>>({})
// presupuestos es el consumo frente a los límites de cada API con presupuesto; presupuestoEspacio, el del espacio.
const presupuestos = ref<Record<string, EstadoPresupuesto>>({})
const presupuestoEspacio = ref<EstadoPresupuesto>()
const props = defineProps<{ intencion?: IntencionDe<'conectar'> }>()
const emit = defineEmits<{ ir: [destino: Destino] }>()

function verPeticiones(nombre: string) {
  emit('ir', { pagina: 'historial', origen: nombre })
}

// cargandoPuntos y errorPuntos: la lista no enseña «crea tu primera API» mientras carga ni si la carga falló.
const cargandoPuntos = ref(true)
const errorPuntos = ref('')
async function cargarPuntos() {
  cargandoPuntos.value = true
  try {
    const r = await api.puntosConUso()
    puntos.value = r.puntos_acceso
    uso.value = r.uso ?? {}
    presupuestos.value = r.presupuestos ?? {}
    errorPuntos.value = ''
    presupuestoEspacio.value = await api.presupuestoInstancia()
  } catch (e) {
    if (puntos.value.length) avisarError(e)
    else errorPuntos.value = mensajeError(e)
  } finally {
    cargandoPuntos.value = false
  }
}

function cargarClave() {
  errorClave.value = ''
  api.claveInstancia().then((r) => (clave.value = r.token)).catch((e) => (errorClave.value = mensajeError(e)))
}

function abrirNuevo() {
  formularioApi.value?.abrirNuevo()
}

// alCrear abre la API recién creada, ya con la lista recargada.
function alCrear(punto: ConfigPunto, r: { punto_acceso?: EstadoPunto; token: string }) {
  // Con la clave recién creada en los ejemplos: es la única vez que se ve, así que el detalle se abre siempre.
  const creado = puntos.value.find((p) => p.nombre === punto.nombre) ?? r.punto_acceso
  if (creado) abrirDetalle(creado, r.token, true)
  // Sin el detalle, la clave queda en un aviso que no se cierra solo: no se puede volver a ver.
  else $q.notify({ type: 'positive', message: t('puntos.claveCreada', { clave: r.token }), position: 'top', timeout: 0,
    actions: [{ label: t('comun.copiar'), color: 'white', noDismiss: true, handler: () => copiar(r.token) }, { label: t('comun.cerrar'), color: 'white' }] })
}

async function activar(nombre: string, valor: boolean) {
  try {
    await api.activarPunto(nombre, valor)
    await cargarPuntos()
  } catch (e) {
    avisarError(e)
  }
}

// activarGrupo cambia de uno en uno los puntos del grupo que no están ya en el estado pedido.
async function activarGrupo(grupo: string, valor: boolean) {
  ocupado.value = grupo + (valor ? ':1' : ':0')
  try {
    for (const p of puntos.value.filter((p) => (p.grupo ?? '') === grupo && activo(p) !== valor)) await api.activarPunto(p.nombre, valor)
  } catch (e) {
    avisarError(e)
  } finally {
    ocupado.value = ''
    await cargarPuntos()
  }
}

function eliminar(nombre: string) {
  $q.dialog({ title: t('puntos.eliminarTitulo', { nombre }), message: t('puntos.eliminarTexto'),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'negative', label: t('comun.eliminar'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        await api.eliminarPunto(nombre)
        if (detalle.value === nombre) dialogoDetalle.value = false
        await cargarPuntos()
      } catch (e) {
        avisarError(e)
      }
    })
}

// abrirEnProbar lleva la receta de la API a Probar para probarla y actualizarla.
function abrirEnProbar(nombre: string) {
  emit('ir', { pagina: 'ejecutar', receta: nombre })
}

// reiniciarPresupuesto pone a cero el consumo del día y del mes de una API, o del espacio sin nombre.
function reiniciarPresupuesto(nombre?: string) {
  $q.dialog({ title: nombre ? t('presupuesto.reiniciarTitulo', { nombre }) : t('presupuesto.reiniciarEspacio'), message: nombre ? t('presupuesto.reiniciarTexto') : t('presupuesto.reiniciarEspacioTexto'),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'primary', label: t('presupuesto.reiniciar'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        await api.reiniciarPresupuesto(nombre)
        avisar(t('presupuesto.reiniciado'))
        await cargarPuntos()
      } catch (e) {
        avisarError(e)
      }
    })
}

// regenerarClaveInstancia cambia la clave de «Conectar» por si se filtró; las aplicaciones que la usan dejan de
// conectar hasta que se les ponga la nueva.
function regenerarClaveInstancia() {
  $q.dialog({ title: t('conectar.regenerarTitulo'), message: t('conectar.regenerarTexto'),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'negative', label: t('conectar.regenerarClave'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        clave.value = (await api.regenerarClaveInstancia()).token
        verClave.value = true
        avisar(t('conectar.claveRegenerada'))
      } catch (e) {
        avisarError(e)
      }
    })
}

// regenerarClave crea una clave nueva y la muestra esta única vez: Tramtai solo guarda su huella.
function regenerarClave(p: EstadoPunto) {
  $q.dialog({ title: t('puntos.regenerarTitulo', { nombre: p.nombre }), message: t('puntos.regenerarTexto'),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'primary', label: t('puntos.regenerarClave'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        abrirDetalle(p, await api.regenerarClave(p.nombre))
      } catch (e) {
        avisarError(e)
      }
    })
}

onMounted(async () => {
  asegurarFichas()
  cargarDiasHistorial()
  if (props.intencion?.grupo) {
    pestana.value = 'apis'
    filtroGrupo.value = props.intencion.grupo // llegó desde el mapa de Inicio
  }
  cargarPuntos()
  api.catalogo().then((m) => (catalogo.value = m)).catch(avisarError)
  api.instancia().then((i) => {
    base.value = i.base
    appConClave.value = i.app_con_clave_24h
    sugerirApi.value = i.sugerir_api_propia
  }).catch(() => {})
  cargarClave()
})
</script>
