<template>
  <q-card-section>
    <div class="encabezado-herramientas">
      <div class="encabezado-herramientas__texto">
        <h2 class="titulo-seccion">{{ t('herramientas.titulo') }}</h2>
        <div class="texto-suave text-body2">{{ t('herramientas.texto') }}</div>
      </div>
      <div v-if="!noDisponible && lista.length" class="row items-center q-gutter-xs">
        <q-btn flat dense no-caps size="sm" color="primary" icon="o_play_arrow" :label="t('herramientas.activarTodas')" :disable="!puedeActivarTodas(lista) || !!ocupado"
          :loading="ocupado === 'todas:1'" @click="pedirModelo(permitidas)" />
        <q-btn flat dense no-caps size="sm" color="neutro" icon="o_stop" :label="t('herramientas.desactivarTodas')" :disable="!lista.some(estaPuesta) || !!ocupado"
          :loading="ocupado === 'todas:0'" @click="desactivarTodas" />
        <q-btn flat round dense size="sm" icon="o_refresh" :aria-label="t('herramientas.refrescar')" :loading="cargando" @click="cargar"><q-tooltip>{{ t('herramientas.refrescar') }}</q-tooltip></q-btn>
      </div>
    </div>
  </q-card-section>
  <q-separator />

  <!-- En el modo servidor la cuenta del servicio no es la de nadie: las herramientas se apuntan desde cada equipo. -->
  <EstadoVacio v-if="noDisponible" :titulo="t('herramientas.servidor')" :texto="t('herramientas.servidorTexto')" />
  <div v-else-if="cargando && !lista.length" class="q-pa-md" role="status" :aria-label="t('comun.cargando')"><q-skeleton v-for="n in 3" :key="n" type="text" /></div>
  <EstadoVacio v-else-if="error" role="alert" :titulo="t('comun.errorCarga')" :texto="error" :accion="t('comun.reintentar')" @accion="cargar" />
  <EstadoVacio v-else-if="!lista.some((x) => x.estado !== 'no_detectada')" :titulo="t('herramientas.vacio')" :texto="t('herramientas.vacioTexto')">
    <BotonAyuda :tema="TEMA_AYUDA" />
  </EstadoVacio>
  <template v-else>
    <div v-for="herramienta in lista" :key="herramienta.id" class="fila-herramienta" :data-herramienta="herramienta.id">
      <div class="fila-herramienta__info">
        <div class="row items-center q-gutter-x-sm">
          <span class="text-weight-medium">{{ herramienta.nombre }}</span>
          <span class="etiqueta" :class="'etiqueta--' + claseEstado(herramienta.estado)">{{ textoEstado(herramienta.estado) }}</span>
        </div>
        <div v-if="herramienta.modelo" class="text-caption texto-suave">{{ t('herramientas.modelo', { modelo: etiquetaModelo(herramienta.modelo) }) }}</div>
        <div v-if="herramienta.ruta" class="codigo texto-suave romper">{{ herramienta.ruta }}</div>
        <div v-if="herramienta.estado === 'solo_global'" class="text-caption">{{ t('herramientas.soloGlobal') }}</div>
        <div v-else-if="herramienta.estado === 'no_detectada'" class="text-caption texto-suave">{{ t('herramientas.noDetectadaFila') }}</div>
        <!-- El motivo lo da el servicio: por qué está desactualizada, qué impide activarla, si la gestiona una política de la organización. -->
        <div v-else-if="textoMotivo(herramienta)" class="text-caption text-warning row no-wrap items-start">
          <q-icon name="o_info" class="q-mr-xs q-mt-xs" aria-hidden="true" /><span class="romper">{{ textoMotivo(herramienta) }}</span>
        </div>
      </div>
      <div class="fila-herramienta__acciones">
        <q-btn v-if="herramienta.estado === 'desactualizada'" flat dense no-caps color="primary" icon="o_play_arrow" :label="t('herramientas.volverAActivar')"
          :disable="!!ocupado" @click="volverAActivar(herramienta)" />
        <q-spinner v-if="ocupado === herramienta.id" size="18px" color="primary" aria-hidden="true" />
        <q-toggle :model-value="estaPuesta(herramienta)" :label="t('herramientas.interruptor')" :aria-label="t('herramientas.interruptorDe', { nombre: herramienta.nombre })"
          :disable="!puedeCambiar(herramienta) || !!ocupado" @update:model-value="(v: boolean) => cambiar(herramienta, v)" />
        <q-btn v-if="accionesDe(herramienta).length" flat round dense icon="more_horiz" :aria-label="t('herramientas.acciones', { nombre: herramienta.nombre })" :disable="!!ocupado">
          <q-menu>
            <q-list style="min-width: 220px">
              <q-item v-if="accionesDe(herramienta).includes('modelo')" v-close-popup clickable @click="pedirModelo(permitidas, [herramienta.id], herramienta.modelo)">
                <q-item-section avatar><q-icon name="o_tune" /></q-item-section>
                <q-item-section>{{ t('herramientas.cambiarModelo') }}</q-item-section>
              </q-item>
              <q-item v-if="accionesDe(herramienta).includes('olvidar')" v-close-popup clickable @click="olvidar(herramienta)">
                <q-item-section avatar><q-icon name="o_link_off" /></q-item-section>
                <q-item-section>{{ t('herramientas.olvidar') }}</q-item-section>
              </q-item>
              <!-- La destructiva, aparte y al final, en color de error. -->
              <template v-if="herramienta.hay_copia">
                <q-separator v-if="accionesDe(herramienta).length > 1" />
                <q-item v-close-popup clickable class="text-negative" @click="restaurar(herramienta)">
                  <q-item-section avatar><q-icon name="o_restore" color="negative" /></q-item-section>
                  <q-item-section>{{ t('herramientas.restaurar') }}</q-item-section>
                </q-item>
              </template>
            </q-list>
          </q-menu>
        </q-btn>
      </div>
      <div v-if="herramienta.clave_en_archivo && puedeCambiar(herramienta)" class="fila-herramienta__nota text-caption texto-suave row no-wrap items-start">
        <q-icon name="o_lock" class="q-mr-xs q-mt-xs" aria-hidden="true" /><span>{{ t('herramientas.claveEnArchivo') }}</span>
      </div>
    </div>
  </template>

  <!-- Activar pide el agente y el modelo, con el primero conectado elegido; «Activar todas» lo pide una vez para todas. -->
  <q-dialog v-model="dialogoModelo" :maximized="$q.screen.lt.sm" aria-labelledby="titulo-herramientas-modelo">
    <q-card style="width: 520px; max-width: 96vw">
      <q-card-section class="row items-center no-wrap q-gutter-x-sm">
        <h2 id="titulo-herramientas-modelo" class="titulo-seccion col">{{ tituloModelo }}</h2>
        <q-btn v-close-popup flat round dense icon="close" :aria-label="t('comun.cerrar')" />
      </q-card-section>
      <q-separator />
      <q-card-section>
        <div class="text-body2 texto-suave q-mb-md">{{ t('herramientas.modeloTexto') }}</div>
        <q-banner v-if="!permitidas.length" rounded dense class="aviso-caja">{{ t('herramientas.sinAgentes') }}</q-banner>
        <SelectorModelo v-else :model-value="modelo" :opciones="permitidas" :label="t('herramientas.modeloEtiqueta')" @update:model-value="(v) => (modelo = (v as string | null) ?? '')" />
      </q-card-section>
      <q-card-actions align="right">
        <q-btn v-close-popup flat no-caps :label="t('comun.cancelar')" />
        <q-btn unelevated no-caps color="primary" :label="t(objetivo.todas ? 'herramientas.activarTodas' : objetivo.cambiando ? 'herramientas.cambiarModelo' : 'herramientas.activar')"
          :disable="!modelo" @click="confirmarModelo" />
      </q-card-actions>
    </q-card>
  </q-dialog>

  <!-- Restaurar pierde lo hecho en el archivo desde entonces: persistent, y Esc cierra sin más (nada que perder al cerrar). -->
  <q-dialog v-model="dialogoRestaurar" persistent aria-labelledby="titulo-herramientas-restaurar" @keyup.esc="escaparConAviso($event, false, () => (dialogoRestaurar = false))">
    <q-card style="width: 480px; max-width: 96vw">
      <q-card-section>
        <h2 id="titulo-herramientas-restaurar" class="titulo-seccion">{{ t('herramientas.restaurarTitulo', { nombre: aRestaurar?.nombre ?? '' }) }}</h2>
      </q-card-section>
      <q-card-section class="q-pt-none text-body2">{{ aRestaurar ? textoRestaurar(aRestaurar) : '' }}</q-card-section>
      <q-card-actions align="right">
        <q-btn flat no-caps :label="t('comun.cancelar')" @click="dialogoRestaurar = false" />
        <q-btn unelevated no-caps color="negative" :label="t('herramientas.restaurar')" @click="confirmarRestaurar" />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
// Herramientas es la tercera pestaña de Mis APIs: Claude Code, Codex, Continue, opencode y Kilo apuntadas a Tramtai desde
// el espacio actual. El estado y las llamadas al servicio están en herramientas.ts.
import { computed, onMounted, ref } from 'vue'
import { useQuasar } from 'quasar'
import BotonAyuda from '../ayuda/BotonAyuda.vue'
import EstadoVacio from '../EstadoVacio.vue'
import SelectorModelo from '../SelectorModelo.vue'
import { avisar } from '../../api/cliente'
import type { EstadoHerramienta } from '../../api/tipos'
import { etiquetaModelo, type OpcionModelo } from '../../agentes'
import { escaparConAviso } from '../../cierre'
import { accionesDe, claseEstado, crearHerramientas, estaPuesta, modeloInicial, opcionesPermitidas, puedeActivarTodas, puedeCambiar, TEMA_AYUDA, textoEstado, textoMotivo, textoRestaurar } from '../../herramientas'
import { t } from '../../i18n'

// opciones son los modelos del catálogo (los de Probar); solo se ofrecen los de agentes que una API admite.
const props = defineProps<{ opciones: OpcionModelo[] }>()
const $q = useQuasar()
// cambio avisa a Mis APIs de que algo cambió: la API de la herramienta se crea, se detiene o se reanuda con ella.
const emit = defineEmits<{ cambio: [] }>()

const { lista, cargando, error, noDisponible, ocupado, cargar, activar, desactivar, olvidar: olvidarId, restaurar: restaurarId, dialogoModelo, modelo, objetivo,
  pedirModelo, confirmarModelo: confirmar } = crearHerramientas(() => emit('cambio'))

const permitidas = computed(() => opcionesPermitidas(props.opciones))
const tituloModelo = computed(() => objetivo.value.todas ? t('herramientas.activarTodasTitulo')
  : t(objetivo.value.cambiando ? 'herramientas.cambiarTitulo' : 'herramientas.activarTitulo', { nombre: objetivo.value.nombre }))

async function confirmarModelo() {
  if (await confirmar()) avisar(t('herramientas.avisoActivada'))
}

// Encender el interruptor pide el modelo; apagarlo desactiva sin más (la clave y la API se conservan).
async function cambiar(h: EstadoHerramienta, valor: boolean) {
  if (valor) return pedirModelo(permitidas.value, [h.id], h.modelo)
  if (await desactivar([h.id], false)) avisar(t('herramientas.avisoDesactivada'))
}

async function desactivarTodas() {
  if (await desactivar([], true)) avisar(t('herramientas.avisoDesactivada'))
}

// Una desactualizada vuelve a activarse con el mismo modelo: lo que cambió fue el puerto, la API o el archivo.
async function volverAActivar(h: EstadoHerramienta) {
  if (await activar([h.id], h.modelo || modeloInicial(permitidas.value))) avisar(t('herramientas.avisoActivada'))
}

function olvidar(h: EstadoHerramienta) {
  $q.dialog({ title: t('herramientas.olvidarTitulo', { nombre: h.nombre }), message: t('herramientas.olvidarTexto'),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'negative', label: t('herramientas.olvidar'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      if (await olvidarId(h.id)) avisar(t('herramientas.avisoOlvidada'))
    })
}

const dialogoRestaurar = ref(false)
const aRestaurar = ref<EstadoHerramienta>()
function restaurar(h: EstadoHerramienta) {
  aRestaurar.value = h
  dialogoRestaurar.value = true
}
async function confirmarRestaurar() {
  dialogoRestaurar.value = false
  if (aRestaurar.value && (await restaurarId(aRestaurar.value.id))) avisar(t('herramientas.avisoRestaurada'))
}

onMounted(cargar)
</script>

<style scoped>
/* El texto y los botones de arriba: en pantallas estrechas, los botones pasan debajo. */
.encabezado-herramientas {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 8px 16px;
}
.encabezado-herramientas__texto {
  flex: 1 1 280px;
  min-width: 0;
}
/* Una fila: nombre, estado, modelo y archivo a la izquierda; el interruptor y el menú a la derecha. */
.fila-herramienta {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px 16px;
  align-items: start;
  padding: 16px;
}
.fila-herramienta + .fila-herramienta {
  border-top: 1px solid var(--borde);
}
.fila-herramienta__info {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.fila-herramienta__acciones {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 4px;
}
.fila-herramienta__nota {
  grid-column: 1 / -1;
}
@media (max-width: 599px) {
  .fila-herramienta {
    grid-template-columns: minmax(0, 1fr);
  }
  .fila-herramienta__acciones {
    justify-content: space-between;
  }
}
/* Con el dedo, el interruptor ocupa al menos 44 px de alto (los botones ya lo hacen en estilos.css). */
@media (pointer: coarse) {
  .fila-herramienta :deep(.q-toggle) {
    min-height: 44px;
  }
}
</style>
