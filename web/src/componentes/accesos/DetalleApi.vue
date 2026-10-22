<template>
  <!-- Detalle de una API: cómo conectarla, sus acciones y la zona de peligro. Al cerrarse se olvida la clave
       mostrada: no queda en memoria de la página. -->
  <q-dialog v-model="abierto" :maximized="$q.screen.lt.sm" aria-labelledby="titulo-detalle" @hide="claveVista = ''">
    <q-card v-if="puntoDetalle" style="width: 760px; max-width: 96vw">
      <q-card-section class="row items-center no-wrap q-gutter-x-sm">
        <span class="linea__chapa linea__chapa--grande" :class="['linea--' + (puntoDetalle.agente ?? 'otro'), { 'linea__chapa--apagada': !activo(puntoDetalle) }]" aria-hidden="true">{{ puntoDetalle.agente ? codigoLinea(puntoDetalle.agente) : 'API' }}</span>
        <div class="col">
          <h2 id="titulo-detalle" class="titulo-seccion">{{ recienCreado ? t('puntos.listo') : puntoDetalle.nombre }}</h2>
          <div class="row items-center q-gutter-x-sm text-caption texto-suave">
            <span class="etiqueta" :class="!activo(puntoDetalle) ? 'etiqueta--neutro' : puntoDetalle.error ? 'etiqueta--error' : 'etiqueta--ok'">{{ activo(puntoDetalle) ? t('puntos.activo') : t('puntos.desactivado') }}</span>
            <span v-if="puntoDetalle.grupo"><q-icon name="o_folder" /> {{ puntoDetalle.grupo }}</span>
          </div>
        </div>
        <q-btn v-close-popup flat round dense icon="close" :aria-label="t('comun.cerrar')" />
      </q-card-section>
      <q-separator />
      <q-card-section>
        <div class="text-weight-medium">{{ t('puntos.conectar') }}</div>
        <div class="texto-suave text-body2 q-mb-sm">{{ t('puntos.conectarTexto') }}</div>
        <template v-if="puntoDetalle.url">
        <GuiaUrls :url="puntoDetalle.url" :contratos="puntoDetalle.contratos" :nombre="puntoDetalle.nombre" class="q-mb-md" @usar="usarApiPropia" />
        <div class="row q-col-gutter-md">
          <div class="col-12">
            <div class="text-caption texto-suave q-mb-xs">{{ t('conectar.clave') }}</div>
            <template v-if="claveVista">
              <div class="bloque-codigo">{{ claveVista }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('conectar.copiarClave')" @click="copiar(claveVista, t('conectar.claveCopiada'))" /></div>
              <q-banner rounded dense class="aviso-caja q-mt-sm">
                <template #avatar><q-icon name="o_lock" color="warning" /></template>{{ t('puntos.claveUnaVez') }}
              </q-banner>
            </template>
            <div v-else class="row items-center q-gutter-sm">
              <span class="text-body2 texto-suave col-12 col-sm">{{ t('puntos.claveOculta') }}</span>
              <q-btn outline dense no-caps color="primary" icon="o_key" class="q-px-sm" :label="t('puntos.regenerarClave')" @click="emit('regenerarClave', puntoDetalle)" />
            </div>
          </div>
        </div>
        <template v-if="ejemplosDetalle.length">
        <div ref="ejemplosRef" class="text-caption texto-suave q-mt-md q-mb-xs">{{ t('conectar.ejemplos') }}</div>
        <div class="desplazable-x"><q-btn-toggle v-model="lenguaje" no-caps flat dense toggle-color="primary" class="segmentado inline-flex"
          :options="ejemplosDetalle.map((e) => ({ label: e.titulo(), value: e.id }))" /></div>
        <div class="bloque-codigo sin-ajuste q-mt-sm">{{ ejemploDetalle.codigo(claveVista || t('puntos.claveMarcador', { nombre: puntoDetalle.nombre })) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('conectar.copiarEjemplo')" @click="copiar(ejemploDetalle.codigo(claveVista || t('puntos.claveMarcador', { nombre: puntoDetalle.nombre })))" /></div>
        </template>
        </template>
        <div v-else class="row items-center q-gutter-sm">
          <span class="text-body2 texto-suave col-12 col-sm">{{ t('puntos.activaParaConectar') }}</span>
          <q-btn outline dense no-caps color="primary" icon="o_play_arrow" class="q-px-sm" :label="t('puntos.accionActivar')" @click="emit('activar', puntoDetalle.nombre, true)" />
        </div>
      </q-card-section>
      <q-separator />
      <q-card-section>
        <div class="text-weight-medium q-mb-sm">{{ t('puntos.acciones') }}</div>
        <div class="row q-gutter-sm">
          <q-btn outline no-caps color="primary" icon="o_tune" :label="t('puntos.ajustes')" @click="emit('ajustes', puntoDetalle)" />
          <q-btn outline no-caps color="primary" icon="o_science" :label="t('puntos.abrirProbar')" @click="emit('probar', puntoDetalle.nombre)" />
          <q-btn outline no-caps color="primary" icon="o_history" :label="t('puntos.verPeticiones')" @click="emit('verPeticiones', puntoDetalle.nombre)" />
          <q-btn outline no-caps color="primary" :icon="activo(puntoDetalle) ? 'o_pause' : 'o_play_arrow'"
            :label="activo(puntoDetalle) ? t('puntos.accionDesactivar') : t('puntos.accionActivar')" @click="emit('activar', puntoDetalle.nombre, !activo(puntoDetalle))" />
          <q-btn v-if="presupuestos[puntoDetalle.nombre]" outline no-caps color="primary" icon="o_restart_alt" :label="t('presupuesto.reiniciar')" @click="emit('reiniciarPresupuesto', puntoDetalle.nombre)" />
        </div>
      </q-card-section>
      <q-separator />
      <q-card-section>
        <div class="text-weight-medium text-negative">{{ t('puntos.peligro') }}</div>
        <div class="row items-center q-gutter-sm q-mt-xs">
          <span class="text-body2 texto-suave col-12 col-sm">{{ t('puntos.peligroTexto') }}</span>
          <q-btn outline no-caps color="negative" icon="o_delete" :label="t('puntos.eliminarApi')" @click="emit('eliminar', puntoDetalle.nombre)" />
        </div>
      </q-card-section>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useQuasar } from 'quasar'
import GuiaUrls from '../GuiaUrls.vue'
import { copiar } from '../../api/cliente'
import type { EstadoPresupuesto, EstadoPunto } from '../../api/tipos'
import { codigoLinea } from '../../agentes'
import { idModeloDe as idModelo } from '../../recetas'
import { ejemplos as ejemplosDe } from '../../ejemplos'
import { t } from '../../i18n'
import { activo } from './comun'

// puntoDetalle es la API abierta; claveVista, su clave recién creada o regenerada (solo se ve esta vez).
const props = defineProps<{ puntoDetalle?: EstadoPunto; recienCreado: boolean; presupuestos: Record<string, EstadoPresupuesto>; modeloEjemplo: string }>()
const emit = defineEmits<{ activar: [nombre: string, valor: boolean]; ajustes: [p: EstadoPunto]; probar: [nombre: string]; verPeticiones: [nombre: string];
  reiniciarPresupuesto: [nombre: string]; eliminar: [nombre: string]; regenerarClave: [p: EstadoPunto] }>()
const abierto = defineModel<boolean>({ required: true })
const claveVista = defineModel<string>('claveVista', { required: true })
const lenguaje = defineModel<string>('lenguaje', { required: true })
// usarApiPropia muestra los pasos de la API propia: su ejemplo en curl, con la URL y la clave puestas.
const ejemplosRef = ref<HTMLElement>()
function usarApiPropia() {
  lenguaje.value = 'curl-nativa'
  ejemplosRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
const $q = useQuasar()

const ejemplosDetalle = computed(() => (props.puntoDetalle
  ? ejemplosDe({ url: props.puntoDetalle.url ?? '', modelo: idModelo(props.puntoDetalle) ?? props.modeloEjemplo, contratos: props.puntoDetalle.contratos,
    muestra: { archivos: ['informe.pdf'], solicitud: { mensajes: [{ rol: 'usuario', contenido: t('ejemplos.mensaje') }] } } }) : []))
const ejemploDetalle = computed(() => ejemplosDetalle.value.find((e) => e.id === lenguaje.value) ?? ejemplosDetalle.value[0])
</script>
