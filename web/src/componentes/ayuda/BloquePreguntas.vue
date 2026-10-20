<template>
  <q-list separator class="borde-redondeado">
    <q-expansion-item v-for="p in bloque.preguntas" :id="'ayuda-' + p.id" :key="p.id" :label="t(p.pregunta)" :default-opened="p.id === ancla"
      header-class="text-weight-medium" expand-icon-class="texto-suave">
      <div class="q-px-md q-pb-md texto-suave" style="line-height: 1.6">
        {{ t(p.respuesta) }}
        <q-btn v-if="p.tema && temaPorId(p.tema)" flat dense no-caps color="primary" icon-right="arrow_forward" class="q-ml-xs"
          :label="t(temaPorId(p.tema)!.titulo)" @click="$emit('tema', p.tema)" />
      </div>
    </q-expansion-item>
  </q-list>
</template>

<script setup lang="ts">
// BloquePreguntas son preguntas plegadas; ancla abre la que se buscó. Un concepto enlaza con el tema que lo explica.
import { t } from '../../i18n'
import { temaPorId } from '../../ayuda/indice'
import type { Bloque } from '../../ayuda/tipos'

defineProps<{ bloque: Extract<Bloque, { tipo: 'preguntas' }>; ancla?: string }>()
defineEmits<{ tema: [id: string] }>()
</script>
