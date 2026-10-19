<template>
  <div class="pagina apilado">
    <CabeceraPagina :titulo="t('menu.agentes')" :descripcion="t('agentes.subtitulo')">
      <template #acciones>
        <q-btn flat no-caps color="primary" icon="o_refresh" :label="t('agentes.recomprobar')" :loading="cargando" @click="cargar" />
      </template>
    </CabeceraPagina>

    <template v-if="adaptadores">
      <div class="row items-center q-col-gutter-sm">
        <div class="col-12 col-sm">
          <q-input v-model="busqueda" dense outlined clearable debounce="150" :placeholder="t('agentes.buscar')" :aria-label="t('agentes.buscar')">
            <template #prepend><q-icon name="search" /></template>
          </q-input>
        </div>
        <div class="col-12 col-sm-auto desplazable-x">
          <q-btn-toggle v-model="filtro" no-caps flat dense toggle-color="primary" class="segmentado inline-flex" :options="opcionesFiltro" />
        </div>
      </div>

      <q-card v-if="filtro !== 'instalar'">
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
                  <q-icon :name="listo(a) ? 'check' : 'priority_high'" size="14px" />{{ listo(a) ? t('agente.listo') : t(tipo(a)) }}
                </span>
              </q-item-section>
            </template>
            <div class="q-px-md q-pb-md">
        <!-- Listo: cómo está conectado y, si su ficha admite las dos, la otra forma -->
        <template v-if="listo(a)">
          <div v-if="a.problema" class="aviso text-body2 q-mb-sm">{{ a.problema }}</div>
          <div class="metadatos">
            <span><q-icon :name="suscripcion(a) ? 'o_person' : 'o_key'" />{{ suscripcion(a) ? t('agentes.listoSuscripcion') : t('agentes.listoClave') }}</span>
            <span v-if="esfuerzos[a.agente]?.length"><q-icon name="o_psychology" />{{ t('agentes.esfuerzo') }}: {{ esfuerzos[a.agente].map(nombreEsfuerzo).join(' · ') }}</span>
            <span v-if="contextos[a.agente]?.length"><q-icon name="o_memory" />{{ t('agentes.contexto') }}: {{ textoContextos(a.agente) }}</span>
          </div>
          <q-btn unelevated dense no-caps color="primary" icon="o_chat" class="q-mt-md q-mr-sm q-px-md" :label="t('agentes.probar')" @click="probar(a)" />
          <q-btn v-if="puedeCambiar(a)" flat dense no-caps color="primary" class="q-mt-md q-px-sm" :loading="cambiando === a.agente"
            :label="suscripcion(a) ? t('agentes.cambiarAClave') : t('agentes.cambiarASuscripcion')"
            @click="suscripcion(a) ? cambiar(a, 'clave_api') : confirmarSuscripcion(a)" />
        </template>

        <!-- Falta conectarlo: las dos opciones, lado a lado -->
        <template v-else-if="tipo(a) === 'agente.faltaClave' && !suscripcion(a)">
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
          <template v-else>
            <div class="text-body2">{{ t('agentes.soloClave') }}</div>
            <div class="bloque-codigo q-mt-sm">{{ comando(a) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('agentes.copiarComando')" @click="copiar(comando(a))" /></div>
            <div class="text-caption texto-suave q-mt-xs">{{ textoAlmacen(a) }}</div>
          </template>
        </template>

        <!-- Con suscripción y sin sesión: cómo iniciarla o, si se prefiere, la clave -->
        <template v-else-if="tipo(a) === 'agente.faltaClave'">
          <div class="aviso text-body2">{{ a.problema }}</div>
          <q-btn v-if="puedeCambiar(a)" flat dense no-caps color="primary" class="q-mt-sm q-px-sm" :loading="cambiando === a.agente"
            :label="t('agentes.cambiarAClave')" @click="cambiar(a, 'clave_api')" />
        </template>

        <div v-else-if="tipo(a) === 'agente.noInstalado'" class="aviso text-body2">
          {{ t('agentes.instalarAntes') }} <span class="codigo">comandos_agentes.{{ a.agente }}</span>.
        </div>
        <div v-else class="aviso text-body2">{{ a.problema }}</div>
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
        <div v-if="!visibles.length" class="q-pa-md texto-suave">{{ t('agentes.sinResultados') }}</div>
      </q-card>

      <!-- Los que no están instalados van aparte, plegados: con muchos agentes no tapan a los que se usan. -->
      <q-card v-if="filtro === 'instalar' && !otros.length">
        <div class="q-pa-md texto-suave">{{ t('agentes.sinResultados') }}</div>
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
                <q-item-label caption>{{ t('agentes.instalarAntes') }} <span class="codigo">comandos_agentes.{{ a.agente }}</span>.</q-item-label>
              </q-item-section>
              <q-item-section v-if="fichaDe(a.agente)?.url_instalacion" side>
                <q-btn flat dense no-caps color="primary" icon-right="o_open_in_new" :label="t('agentes.comoInstalar')"
                  :href="fichaDe(a.agente)!.url_instalacion" target="_blank" rel="noopener noreferrer" />
              </q-item-section>
            </q-item>
          </q-list>
        </q-expansion-item>
      </q-card>
    </template>
    <div v-else class="row q-col-gutter-md">
      <div v-for="n in 4" :key="n" class="col-12 col-lg-6">
        <q-card class="q-pa-md"><q-skeleton type="text" width="40%" /><q-skeleton v-for="m in 3" :key="m" type="text" /></q-card>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import { useQuasar } from 'quasar'
import { api, avisar, avisarError, copiar } from '../api/cliente'
import type { Adaptador } from '../api/tipos'
import { modeloInicial, sesion } from '../estado'
import { agenteListo, asegurarFichas, codigoLinea, fichaDe, nombreAgente, fijarDisponibilidad, ordenarAgentes } from '../agentes'
import { nombreEsfuerzo, numero } from '../formato'
import { t } from '../i18n'

const $q = useQuasar()
const emit = defineEmits<{ ir: [pagina: string] }>()

function probar(a: Adaptador) {
  modeloInicial.value = a.agente
  emit('ir', 'ejecutar')
}
const adaptadores = ref<Adaptador[]>()
// esfuerzos: los niveles que admite cada agente, según el catálogo de modelos.
const esfuerzos = ref<Record<string, string[]>>({})
// contextos: el contexto conocido de los modelos de cada agente (configurado o informado), para ver qué cabe.
const contextos = ref<Record<string, { nombre: string; tokens: number; origen: string }[]>>({})
const textoContextos = (agente: string) => (contextos.value[agente] ?? [])
  .map((c) => `${c.nombre} ${numero(c.tokens)}${c.origen === 'configurado' ? ' (' + t('agentes.configurado') + ')' : ''}`).join(' · ')
const cargando = ref(false)
const cambiando = ref('')

const listo = agenteListo
const instalado = (a: Adaptador) => a.problema_codigo !== 'no_instalado'
// Lista para crecer: buscador, filtro por estado con su número y los no instalados aparte. Primero los listos, luego los
// que falta conectar, en el orden de sus fichas.
const busqueda = ref<string | null>('')
const filtro = ref<'todos' | 'listos' | 'conectar' | 'instalar'>('todos')
const otrosAbierto = ref(false)
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

// tipo clasifica el problema por su código (el texto llega en el idioma del servicio) para mostrar la ayuda adecuada.
function tipo(a: Adaptador) {
  if (a.problema_codigo === 'no_instalado') return 'agente.noInstalado' as const
  if (a.problema_codigo === 'falta_clave') return 'agente.faltaClave' as const
  return 'agente.revisar' as const
}
const suscripcion = (a: Adaptador) => a.autenticacion === 'suscripcion'
// La suscripción la admiten los agentes cuya ficha la incluye, y solo se cambia desde una sesión de este mismo equipo.
const puedeCambiar = (a: Adaptador) => !!fichaDe(a.agente)?.autenticaciones.includes('suscripcion') && !!sesion.value?.local
const cli = (a: Adaptador) => fichaDe(a.agente)?.programa ?? a.agente
const variable = (a: Adaptador) => fichaDe(a.agente)?.variables_clave[0] ?? a.problema?.match(/[A-Z_]+_API_KEY/)?.[0] ?? ''
const comando = (a: Adaptador) => 'tramtai claves guardar ' + variable(a)
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
  } catch (e) {
    avisarError(e)
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

onMounted(() => {
  asegurarFichas()
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
