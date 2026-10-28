<template>
  <div class="pagina apilado">
    <CabeceraPagina tema="agentes" :titulo="t('menu.agentes')" :descripcion="t('agentes.subtitulo')">
      <template #acciones>
        <q-btn data-recorrido="agentes.recomprobar" flat no-caps color="primary" icon="o_refresh" :label="t('agentes.recomprobar')" :loading="cargando" @click="cargar" />
      </template>
    </CabeceraPagina>

    <template v-if="adaptadores">
      <div class="row items-center q-col-gutter-sm">
        <div class="col-12 col-sm">
          <q-input v-model="busqueda" dense outlined clearable debounce="150" :placeholder="t('agentes.buscar')" :aria-label="t('agentes.buscar')">
            <template #prepend><q-icon name="search" /></template>
          </q-input>
        </div>
        <div class="col-12 col-sm-auto desplazable-x" data-recorrido="agentes.conectar">
          <q-btn-toggle v-model="filtro" no-caps flat dense toggle-color="primary" class="segmentado inline-flex" :options="opcionesFiltro" />
        </div>
      </div>

      <q-card v-if="filtro !== 'instalar'" data-captura="agente" data-recorrido="agentes.tarjeta">
        <q-list separator>
          <q-expansion-item v-for="a in visibles" :key="a.agente" group="agentes" header-class="q-py-sm">
            <template #header>
              <q-item-section avatar>
                <span class="linea__chapa linea__chapa--grande" :class="['linea--' + a.agente, { 'linea__chapa--apagada': !listo(a) }]" aria-hidden="true">{{ codigoLinea(a.agente) }}</span>
              </q-item-section>
              <q-item-section>
                <q-item-label class="text-weight-medium">{{ nombreAgente(a.agente) }}
                  <span class="text-caption texto-suave q-ml-xs">{{ a.version_detectada || t('agentes.sinVersion') }}</span></q-item-label>
                <q-item-label caption lines="1">{{ resumen(a) }}</q-item-label>
              </q-item-section>
              <q-item-section side>
                <span class="etiqueta" :class="listo(a) ? 'etiqueta--ok' : 'etiqueta--aviso'">
                  {{ t(etiquetas[a.estado]) }}
                </span>
              </q-item-section>
            </template>
            <div class="q-px-md q-pb-md">
        <!-- Desconectado: no se usa ni como respaldo hasta que se conecte -->
        <template v-if="a.estado === 'desconectado'">
          <div class="text-body2">{{ t('agentes.desconectadoTexto') }}</div>
          <q-btn unelevated dense no-caps color="primary" icon="o_link" class="q-mt-md q-px-md" :label="t('agentes.conectar')"
            :loading="cambiando === a.agente + ':conexion'" @click="conectar(a, true)" />
        </template>

        <!-- Listo: cómo está conectado y, si su ficha admite las dos, la otra forma -->
        <template v-else-if="listo(a)">
          <div v-if="a.problema" class="aviso text-body2 q-mb-sm">{{ a.problema }}</div>
          <div class="metadatos">
            <span><q-icon :name="suscripcion(a) ? 'o_person' : 'o_key'" />{{ suscripcion(a) ? t('agentes.listoSuscripcion') : t('agentes.listoClave') }}</span>
            <span v-if="suscripcion(a) && a.cuenta"><q-icon name="o_account_circle" />{{ t('agentes.cuenta', { cuenta: a.cuenta }) }}</span>
            <span v-if="esfuerzos[a.agente]?.length"><q-icon name="o_speed" />{{ t('agentes.esfuerzo') }}: {{ esfuerzos[a.agente].map(nombreEsfuerzo).join(' · ') }}</span>
            <span v-if="contextos[a.agente]?.length"><q-icon name="o_memory" />{{ t('agentes.contexto') }}: {{ textoContextos(a.agente) }}</span>
          </div>
          <q-btn unelevated dense no-caps color="primary" icon="o_chat" class="q-mt-md q-mr-sm q-px-md" :label="t('agentes.probar')" @click="probar(a)" />
          <q-btn v-if="puedeCambiar(a)" flat dense no-caps color="primary" class="q-mt-md q-px-sm" :loading="cambiando === a.agente"
            :label="suscripcion(a) ? t('agentes.cambiarAClave') : t('agentes.cambiarASuscripcion')"
            @click="suscripcion(a) ? cambiar(a, 'clave_api') : confirmarSuscripcion(a)" />
        </template>

        <!-- Falta conectarlo: las dos opciones, lado a lado -->
        <template v-else-if="a.estado === 'falta_clave'">
          <template v-if="puedeCambiar(a)">
            <div class="text-weight-medium q-mb-sm">{{ t('agentes.elegir') }}</div>
            <div class="row q-col-gutter-sm">
              <div class="col-12 col-sm-6">
                <div class="opcion full-height column">
                  <div class="text-weight-medium"><q-icon name="o_person" class="q-mr-xs" />{{ t('agentes.opcionSuscripcion') }}</div>
                  <div class="text-caption texto-suave q-mt-xs col">{{ t('agentes.opcionSuscripcionTexto', { cli: cli(a) }) }}</div>
                  <q-btn unelevated no-caps color="primary" class="q-mt-sm" :label="t('agentes.usarSuscripcion')" :loading="cambiando === a.agente"
                    @click="confirmarSuscripcion(a)" />
                </div>
              </div>
              <div class="col-12 col-sm-6">
                <div class="opcion full-height">
                  <div class="text-weight-medium"><q-icon name="o_key" class="q-mr-xs" />{{ t('agentes.opcionClave') }}</div>
                  <div class="text-caption texto-suave q-mt-xs">{{ t('agentes.opcionClaveTexto') }}</div>
                  <div class="bloque-codigo q-mt-sm">{{ comando(a) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('agentes.copiarComando')" @click="copiar(comando(a))" /></div>
                  <div class="text-caption texto-suave q-mt-xs">{{ textoAlmacen(a) }}</div>
                </div>
              </div>
            </div>
          </template>
          <!-- En el servidor, en lugar de «Usar mi suscripción»: la orden del inicio de sesión del CLI (en local) o de la
               suscripción personal con sus condiciones (abierto a la red), y la clave con tramtai-servidor. -->
          <template v-else-if="forma !== 'elegir' && admite(a)">
            <div class="text-body2">{{ t(forma === 'personal' ? 'agentes.servidorPersonal' : 'agentes.servidorEntrar') }}</div>
            <div class="bloque-codigo q-mt-sm">{{ ordenSuscripcion(a.agente, forma) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('agentes.copiarComando')" @click="copiar(ordenSuscripcion(a.agente, forma))" /></div>
            <div class="text-body2 q-mt-md">{{ t('agentes.servidorClave') }}</div>
            <div class="bloque-codigo q-mt-sm">{{ comando(a) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('agentes.copiarComando')" @click="copiar(comando(a))" /></div>
          </template>
          <template v-else>
            <div class="text-body2">{{ t('agentes.soloClave') }}</div>
            <div class="bloque-codigo q-mt-sm">{{ comando(a) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('agentes.copiarComando')" @click="copiar(comando(a))" /></div>
            <div class="text-caption texto-suave q-mt-xs">{{ textoAlmacen(a) }}</div>
          </template>
        </template>

        <!-- Con suscripción y sin sesión: cómo iniciarla o, si se prefiere, la clave -->
        <template v-else-if="a.estado === 'sin_sesion'">
          <div class="aviso text-body2">{{ a.problema }}</div>
          <div v-if="forma !== 'elegir'" class="bloque-codigo q-mt-sm">{{ ordenSuscripcion(a.agente, forma) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('agentes.copiarComando')" @click="copiar(ordenSuscripcion(a.agente, forma))" /></div>
          <q-btn v-if="puedeCambiar(a)" flat dense no-caps color="primary" class="q-mt-sm q-px-sm" :loading="cambiando === a.agente"
            :label="t('agentes.cambiarAClave')" @click="cambiar(a, 'clave_api')" />
        </template>

        <!-- No instalado: qué hacer, y si el CLI está en otra ruta, en qué archivo se escribe y cómo abrirlo. -->
        <div v-else-if="a.estado === 'no_instalado'" class="aviso text-body2">
          {{ t('agentes.instalarAntes') }} {{ t('agentes.otraRuta') }} <span class="codigo">comandos_agentes.{{ a.agente }}</span> {{ t('agentes.otraRutaArchivo') }}
          <div class="bloque-codigo q-mt-sm">{{ rutaConfig }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('agentes.copiarRuta')" @click="copiar(rutaConfig)" /></div>
        </div>
        <!-- El CLI pasó a una clave de API, la sesión no responde u otro problema: qué pasa y, con suscripción, la salida -->
        <template v-else>
          <div class="aviso text-body2">{{ a.estado === 'usa_clave' ? t('agentes.usaClave', { cli: cli(a) })
            : a.estado === 'sesion_no_responde' ? t('agentes.sesionNoResponde', { cli: cli(a) }) : a.problema }}</div>
          <q-btn v-if="ofrecerDejarSuscripcion(a, puedeCambiar(a))" flat dense no-caps color="primary" class="q-mt-sm q-px-sm" :loading="cambiando === a.agente"
            :label="t('agentes.dejarSuscripcion')" @click="cambiar(a, 'clave_api')" />
        </template>
        <q-btn v-if="!a.desconectado && sesion?.local" flat dense no-caps color="neutro" icon="o_link_off" class="q-mt-sm q-px-sm"
          :label="t('agentes.desconectar')" :loading="cambiando === a.agente + ':conexion'" @click="conectar(a, false)" />
      <q-expansion-item dense dense-toggle :label="t('agentes.detalles')" header-class="text-caption texto-suave q-px-none" class="q-mt-sm">
        <div class="q-pb-sm">
          <div class="ficha text-caption">
            <div>{{ t('agentes.version') }}</div><div>{{ a.version_detectada || t('agentes.sinVersion') }}</div>
            <div>{{ t('agentes.autenticacion') }}</div><div>{{ suscripcion(a) ? t('agentes.suscripcion') : t('agentes.claveApi') }}</div>
            <div>{{ t('agentes.enVivo') }}</div><div>{{ a.capacidades.streaming === 'tokens' ? t('agentes.fragmentos') : t('agentes.alTerminar') }}</div>
            <div>{{ t('agentes.json') }}</div><div>{{ a.capacidades.salida_estructurada ? t('comun.si') : t('comun.no') }}</div>
            <div>{{ t('agentes.informaModelo') }}</div><div>{{ a.capacidades.modelo_efectivo_informado ? t('comun.si') : t('comun.no') }}</div>
            <div>{{ t('agentes.esfuerzo') }}</div><div>{{ esfuerzos[a.agente]?.length ? esfuerzos[a.agente].map(nombreEsfuerzo).join(' · ') : t('comun.no') }}</div>
            <div>{{ t('agentes.contexto') }}</div><div>{{ contextos[a.agente]?.length ? textoContextos(a.agente) : t('agentes.contextoDesconocido') }}</div>
          </div>
          <div v-if="a.terminos" class="text-caption texto-suave q-mt-sm">{{ a.terminos }}</div>
        </div>
      </q-expansion-item>
            </div>
          </q-expansion-item>
        </q-list>
        <EstadoVacio v-if="!visibles.length" :titulo="t('agentes.sinResultados')" :accion="hayFiltro ? t('historial.quitarFiltros') : undefined" @accion="quitarFiltros" />
      </q-card>

      <!-- Los que no están instalados van aparte, plegados: con muchos agentes no tapan a los que se usan. -->
      <q-card v-if="filtro === 'instalar' && !otros.length">
        <EstadoVacio :titulo="t('agentes.sinResultados')" :accion="t('historial.quitarFiltros')" @accion="quitarFiltros" />
      </q-card>
      <q-card v-if="otros.length && filtro !== 'listos' && filtro !== 'conectar'">
        <q-expansion-item :model-value="otrosAbierto" icon="o_download" header-class="text-weight-medium"
          :label="t('agentes.otros', { n: otros.length })" :caption="t('agentes.otrosTexto')" @update:model-value="(v: boolean) => (otrosAbierto = v)">
          <q-list separator>
            <q-item v-for="a in otros" :key="a.agente">
              <q-item-section avatar>
                <span class="linea__chapa linea__chapa--grande linea__chapa--apagada" :class="'linea--' + a.agente" aria-hidden="true">{{ codigoLinea(a.agente) }}</span>
              </q-item-section>
              <q-item-section>
                <q-item-label class="text-weight-medium">{{ nombreAgente(a.agente) }}</q-item-label>
                <q-item-label caption>{{ t('agentes.instalarAntes') }} {{ t('agentes.otraRuta') }} <span class="codigo">comandos_agentes.{{ a.agente }}</span> {{ t('agentes.otraRutaArchivo') }}</q-item-label>
              </q-item-section>
              <q-item-section v-if="fichaDe(a.agente)?.url_instalacion" side>
                <q-btn flat dense no-caps color="primary" icon-right="o_open_in_new" :label="t('agentes.comoInstalar')"
                  :href="fichaDe(a.agente)!.url_instalacion" target="_blank" rel="noopener noreferrer" />
              </q-item-section>
            </q-item>
          </q-list>
          <div class="q-px-md q-pb-md">
            <div class="text-caption texto-suave q-mb-xs">{{ t('agentes.archivoConfig') }}</div>
            <div class="bloque-codigo">{{ rutaConfig }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('agentes.copiarRuta')" @click="copiar(rutaConfig)" /></div>
          </div>
        </q-expansion-item>
      </q-card>
    </template>
    <!-- La primera comprobación falló: qué pasó y Reintentar, en lugar de un esqueleto que no termina. -->
    <q-card v-else-if="error">
      <EstadoVacio role="alert" :titulo="t('comun.errorCarga')" :texto="error" :accion="t('comun.reintentar')" :cargando="cargando" @accion="cargar" />
    </q-card>
    <div v-else class="row q-col-gutter-md" role="status" :aria-label="t('comun.cargando')">
      <div v-for="n in 4" :key="n" class="col-12 col-lg-6">
        <q-card class="q-pa-md"><q-skeleton type="text" width="40%" /><q-skeleton v-for="m in 3" :key="m" type="text" /></q-card>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import EstadoVacio from '../componentes/EstadoVacio.vue'
import { useQuasar } from 'quasar'
import { api, avisar, avisarError, copiar, mensajeError } from '../api/cliente'
import type { Adaptador } from '../api/tipos'
import { sesion } from '../estado'
import type { Destino } from '../navegacion'
import { agenteListo, asegurarFichas, cargarFichas, codigoLinea, fichaDe, fijarDisponibilidad, nombreAgente, ofrecerDejarSuscripcion, ordenarAgentes } from '../agentes'
import { nombreEsfuerzo, numero } from '../formato'
import { cargarRutaConfig, rutaConfig } from '../rutaConfig'
import { estadoServidor, formaSuscripcion, ordenClave, ordenSuscripcion, puedeElegirSuscripcion } from '../servidor'
import { t } from '../i18n'
import type { Clave } from '../idiomas/es'

const $q = useQuasar()
const emit = defineEmits<{ ir: [destino: Destino] }>()

function probar(a: Adaptador) {
  emit('ir', { pagina: 'ejecutar', modelo: a.agente })
}
const adaptadores = ref<Adaptador[]>()
// esfuerzos: los niveles que admite cada agente, según el catálogo de modelos.
const esfuerzos = ref<Record<string, string[]>>({})
// contextos: el contexto conocido de los modelos de cada agente (configurado o informado), para ver qué cabe.
const contextos = ref<Record<string, { nombre: string; tokens: number; origen: string }[]>>({})
const textoContextos = (agente: string) => (contextos.value[agente] ?? [])
  .map((c) => `${c.nombre} ${numero(c.tokens)}${c.origen === 'configurado' ? ' (' + t('agentes.configurado') + ')' : ''}`).join(' · ')
const cargando = ref(false)
// error es el de la primera comprobación (sin agentes que enseñar); los de después solo avisan.
const error = ref('')
const cambiando = ref('')

const listo = agenteListo
const instalado = (a: Adaptador) => a.estado !== 'no_instalado'
// Lista para crecer: buscador, filtro por estado con su número y los no instalados aparte. Primero los listos, luego los
// que falta conectar, en el orden de sus fichas.
const busqueda = ref<string | null>('')
const filtro = ref<'todos' | 'listos' | 'conectar' | 'instalar'>('todos')
const otrosAbierto = ref(false)
const hayFiltro = computed(() => !!busqueda.value || filtro.value !== 'todos')
function quitarFiltros() {
  busqueda.value = ''
  filtro.value = 'todos'
}
watch(filtro, (f) => f === 'instalar' && (otrosAbierto.value = true))
const coincide = (a: Adaptador) => {
  const texto = (busqueda.value ?? '').trim().toLowerCase()
  return !texto || [a.agente, nombreAgente(a.agente), cli(a)].some((x) => x.toLowerCase().includes(texto))
}
// El orden se calcula al mostrar: las fichas pueden llegar después que la comprobación de los agentes.
const ordenados = computed(() => ordenarAgentes(adaptadores.value ?? []))
const instalados = computed(() => ordenados.value.filter(instalado))
const visibles = computed(() => instalados.value.filter(coincide)
  .filter((a) => filtro.value === 'todos' || (filtro.value === 'listos') === listo(a))
  .sort((a, b) => Number(listo(b)) - Number(listo(a))))
const otros = computed(() => ordenados.value.filter((a) => !instalado(a) && coincide(a)))
const opcionesFiltro = computed(() => {
  const n = (f: (a: Adaptador) => boolean) => (adaptadores.value ?? []).filter(f).length
  return [
    { label: t('agentes.filtroTodos', { n: n(() => true) }), value: 'todos' },
    { label: t('agentes.filtroListos', { n: n((a) => listo(a)) }), value: 'listos' },
    { label: t('agentes.filtroConectar', { n: n((a) => instalado(a) && !listo(a)) }), value: 'conectar' },
    { label: t('agentes.filtroInstalar', { n: n((a) => !instalado(a)) }), value: 'instalar' },
  ]
})
// resumen es la línea bajo el nombre: cómo está conectado, su esfuerzo y su contexto.
function resumen(a: Adaptador) {
  return [listo(a) ? (suscripcion(a) ? t('agentes.suscripcion') : t('agentes.claveApi')) : '',
    esfuerzos.value[a.agente]?.length ? t('agentes.esfuerzo') + ': ' + esfuerzos.value[a.agente].map(nombreEsfuerzo).join(' · ') : '',
    contextos.value[a.agente]?.length ? t('agentes.contexto') + ': ' + textoContextos(a.agente) : ''].filter(Boolean).join(' · ')
}

// etiquetas son las del estado que manda el servicio: «Sin conectar» vale para el que falta la clave y el que no tiene
// sesión, y «Revisar» para el resto de problemas.
const etiquetas: Record<Adaptador['estado'], Clave> = { listo: 'agente.listo', desconectado: 'agente.desconectado', no_instalado: 'agente.noInstalado',
  falta_clave: 'agente.faltaClave', sin_sesion: 'agente.faltaClave', usa_clave: 'agente.revisar', sesion_no_responde: 'agente.revisar', revisar: 'agente.revisar' }
const suscripcion = (a: Adaptador) => a.autenticacion === 'suscripcion'
// La suscripción la admiten los agentes cuya ficha la incluye, y solo se cambia desde una sesión de este mismo equipo y
// fuera del servidor: en él, forma dice qué orden la pone.
const admite = (a: Adaptador) => !!fichaDe(a.agente)?.autenticaciones.includes('suscripcion')
const forma = computed(() => formaSuscripcion(sesion.value, estadoServidor.value))
const puedeCambiar = (a: Adaptador) => puedeElegirSuscripcion(admite(a), sesion.value, estadoServidor.value)
const cli = (a: Adaptador) => fichaDe(a.agente)?.programa ?? a.agente
const variable = (a: Adaptador) => fichaDe(a.agente)?.variables_clave[0] ?? a.problema?.match(/[A-Z_]+_API_KEY/)?.[0] ?? ''
const comando = (a: Adaptador) => ordenClave(sesion.value, variable(a))
// textoAlmacen dice dónde queda la clave en este sistema: cifrada en su almacén, o qué hacer si no lo hay.
function textoAlmacen(a: Adaptador) {
  const almacen = sesion.value?.almacen
  if (almacen?.disponible) return t('agentes.almacenDisponible', { almacen: almacen.nombre })
  if (almacen?.nombre) return t('agentes.almacenFalta', { variable: variable(a) })
  return t('agentes.sinAlmacen', { variable: variable(a) })
}

async function cargar() {
  cargando.value = true
  try {
    adaptadores.value = (await api.capacidades()).adaptadores
    fijarDisponibilidad(adaptadores.value)
    error.value = ''
  } catch (e) {
    if (adaptadores.value) avisarError(e)
    else error.value = mensajeError(e)
  } finally {
    cargando.value = false
  }
}

// La suscripción solo se activa si el usuario acepta el riesgo con los términos del proveedor (TERMINOS.md, punto 4).
function confirmarSuscripcion(a: Adaptador) {
  $q.dialog({ title: t('agentes.confirmarTitulo', { agente: nombreAgente(a.agente) }),
    message: t('agentes.confirmarSuscripcion', { cli: cli(a) }) + '\n\n' + t('agentes.riesgoSuscripcion'),
    style: 'white-space: pre-line',
    options: { type: 'checkbox', model: [], items: [{ label: t('agentes.aceptoRiesgo'), value: 'acepto' }] },
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'primary', label: t('agentes.usarSuscripcion'), unelevated: true, noCaps: true } })
    .onOk((elegidas: string[]) => {
      if (elegidas.includes('acepto')) cambiar(a, 'suscripcion')
      else avisarError(new Error(t('agentes.faltaAceptar')))
    })
}

// conectar conecta o desconecta el agente en todos los espacios (al desconectar, pide confirmación).
async function conectar(a: Adaptador, conectado: boolean) {
  if (!conectado) {
    const seguir = await new Promise<boolean>((resolver) => $q.dialog({ title: t('agentes.desconectarTitulo', { agente: nombreAgente(a.agente) }),
      message: t('agentes.desconectarTexto'), cancel: { flat: true, noCaps: true, label: t('comun.cancelar') },
      ok: { unelevated: true, noCaps: true, label: t('agentes.desconectar') } }).onOk(() => resolver(true)).onCancel(() => resolver(false)).onDismiss(() => resolver(false)))
    if (!seguir) return
  }
  cambiando.value = a.agente + ':conexion'
  try {
    await api.conectarAgente(a.agente, conectado)
    avisar(t(conectado ? 'agentes.conectado' : 'agentes.desconectado', { agente: nombreAgente(a.agente) }))
    await cargar()
    await cargarFichas()
  } catch (e) {
    avisarError(e)
  } finally {
    cambiando.value = ''
  }
}

async function cambiar(a: Adaptador, modo: 'suscripcion' | 'clave_api') {
  cambiando.value = a.agente
  try {
    await api.cambiarAutenticacion(a.agente, modo)
    avisar(t(modo === 'suscripcion' ? 'agentes.suscripcionActivada' : 'agentes.claveActivada', { agente: nombreAgente(a.agente) }))
    await cargar()
  } catch (e) {
    avisarError(e)
  } finally {
    cambiando.value = ''
  }
}

// Mientras la página está a la vista se vuelve a comprobar cada 5 minutos: si el CLI cerró sesión o cambió de cuenta,
// se ve sin tocar nada.
let comprobacion = 0
onUnmounted(() => clearInterval(comprobacion))
onMounted(() => {
  comprobacion = window.setInterval(() => !document.hidden && cargar(), 5 * 60_000)
  asegurarFichas()
  cargarRutaConfig()
  cargar()
  api.catalogo().then((lista) => {
    for (const m of lista) if (m.agente && !m.modelo && !m.perfil) esfuerzos.value[m.agente] = m.esfuerzos ?? []
    for (const m of lista) {
      if (m.agente && !m.perfil && m.contexto_tokens) {
        const nombre = m.version || m.modelo || t('agentes.predeterminadoCli')
        if (!contextos.value[m.agente]?.some((c) => c.nombre === nombre)) {
          ;(contextos.value[m.agente] ??= []).push({ nombre, tokens: m.contexto_tokens, origen: m.contexto_origen ?? '' })
        }
      }
    }
  }).catch(() => {})
})
</script>
