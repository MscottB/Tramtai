<template>
  <q-list separator class="borde-redondeado">
    <q-item v-for="e in errores" :id="'ayuda-e-' + e.codigo" :key="e.codigo" class="q-py-sm" :class="{ 'ayuda-resaltado': 'e-' + e.codigo === ancla }">
      <q-item-section avatar top><q-icon name="o_report" color="negative" /></q-item-section>
      <q-item-section>
        <q-item-label class="text-weight-medium">{{ e.titulo }}<span v-if="e.codigo !== 'sin_conexion'" class="codigo texto-suave text-caption q-ml-sm">{{ e.codigo }}</span></q-item-label>
        <q-item-label caption style="line-height: 1.5">{{ e.solucion }}</q-item-label>
      </q-item-section>
    </q-item>
  </q-list>
</template>

<script setup lang="ts">
// BloqueErrores lista errores con el mismo título que muestra la app y qué hacer con cada uno.
import { computed } from 'vue'
import { tituloError } from '../../api/cliente'
import { t } from '../../i18n'
import { ERRORES } from '../../ayuda/temas/problemas'
import type { Bloque } from '../../ayuda/tipos'

const props = defineProps<{ bloque: Extract<Bloque, { tipo: 'errores' }>; ancla?: string }>()
const errores = computed(() => props.bloque.codigos.map((codigo) => {
  const e = ERRORES[codigo]
  return { codigo, titulo: e?.titulo ? t(e.titulo) : tituloError(codigo), solucion: e ? t(e.solucion) : '' }
}))
</script>
