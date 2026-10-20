<template>
  <div>
    <ol class="diagrama" :style="{ '--paradas': bloque.paradas.length }" :aria-label="alt">
      <li v-for="(p, i) in bloque.paradas" :key="p.titulo" class="diagrama__parada">
        <q-icon :name="p.icono" size="22px" color="primary" aria-hidden="true" />
        <div class="text-weight-medium q-mt-xs">{{ i + 1 }}. {{ t(p.titulo) }}</div>
        <div v-if="p.nota" class="text-caption texto-suave">{{ t(p.nota) }}</div>
      </li>
    </ol>
    <div v-for="r in bloque.ramas ?? []" :key="r.etiqueta" class="diagrama__rama text-caption q-mt-sm">
      <q-icon name="subdirectory_arrow_right" size="16px" color="primary" aria-hidden="true" />
      {{ r.desde + 1 }} → {{ r.hacia + 1 }}: {{ t(r.etiqueta) }}
    </div>
  </div>
</template>

<script setup lang="ts">
// BloqueDiagrama dibuja un recorrido como paradas de una línea: en fila en pantallas anchas, en columna en el móvil.
import { computed } from 'vue'
import { t } from '../../i18n'
import { altDiagrama } from '../../ayuda/vista'
import type { Bloque } from '../../ayuda/tipos'

const props = defineProps<{ bloque: Extract<Bloque, { tipo: 'diagrama' }> }>()
const alt = computed(() => altDiagrama(props.bloque.paradas.map((p) => t(p.titulo))))
</script>
