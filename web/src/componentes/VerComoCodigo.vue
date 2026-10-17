<template>
  <q-dialog v-model="abierto" :maximized="$q.screen.lt.sm" aria-labelledby="titulo-codigo">
    <q-card style="width: 760px; max-width: 96vw">
      <q-card-section class="row items-center no-wrap">
        <div class="col">
          <div id="titulo-codigo" class="titulo-seccion">{{ t('codigo.titulo') }}</div>
          <div class="texto-suave text-body2">{{ t('codigo.texto') }}</div>
        </div>
        <q-btn v-close-popup flat round dense icon="close" :aria-label="t('comun.cerrar')" class="self-start" />
      </q-card-section>
      <q-card-section class="q-pt-none">
        <q-banner v-if="aviso" rounded dense class="info-caja q-mb-sm">
          <template #avatar><q-icon name="o_info" color="primary" /></template>{{ aviso }}
        </q-banner>
        <div class="desplazable-x"><q-btn-toggle v-model="elegido" no-caps flat dense toggle-color="primary" class="segmentado inline-flex"
          :options="ejemplos.map((e) => ({ label: e.titulo(), value: e.id }))" /></div>
        <q-banner v-if="actual?.nota" rounded dense class="aviso-caja q-mt-sm">
          <template #avatar><q-icon name="o_warning" color="warning" /></template>{{ actual.nota }}
        </q-banner>
        <div v-if="actual" class="bloque-codigo sin-ajuste q-mt-sm">{{ actual.codigo(marcador) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar"
          :aria-label="t('conectar.copiarEjemplo')" @click="copiar(actual.codigo(marcador))" /></div>
      </q-card-section>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
// VerComoCodigo muestra la prueba de Probar como código para copiar; la clave va como marcador: Probar nunca la muestra.
import { computed, ref } from 'vue'
import { copiar } from '../api'
import type { Ejemplo } from '../ejemplos'
import { t } from '../i18n'

const abierto = defineModel<boolean>({ required: true })
const props = defineProps<{ ejemplos: Ejemplo[]; aviso?: string }>()

const marcador = computed(() => t('codigo.tuClave'))
const elegido = ref('python-nativa')
const actual = computed(() => props.ejemplos.find((e) => e.id === elegido.value) ?? props.ejemplos[0])
</script>
