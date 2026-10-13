<template>
  <q-select v-if="niveles.length" :model-value="modelValue" :options="opciones" emit-value map-options outlined dense
    :label="t('esfuerzo.titulo')" :hint="hint ?? t('esfuerzo.hint')" @update:model-value="(v: string) => emit('update:modelValue', v)">
    <template #prepend><q-icon name="o_psychology" /></template>
    <template #option="scope">
      <q-item v-bind="scope.itemProps">
        <q-item-section>
          <q-item-label>{{ scope.opt.label }}</q-item-label>
          <q-item-label caption>{{ scope.opt.caption }}</q-item-label>
        </q-item-section>
      </q-item>
    </template>
  </q-select>
  <div v-else class="text-caption texto-suave row items-center no-wrap">
    <q-icon name="o_psychology" size="18px" class="q-mr-xs" />{{ t('esfuerzo.noAdmite') }}
  </div>
</template>

<script setup lang="ts">
// SelectorEsfuerzo elige el nivel de esfuerzo entre los que admite el modelo; '' deja el del agente.
import { computed } from 'vue'
import { t } from '../i18n'
import type { Clave } from '../idiomas/es'

const props = defineProps<{ modelValue: string; niveles: string[]; hint?: string }>()
const emit = defineEmits<{ 'update:modelValue': [valor: string] }>()

const opciones = computed(() => [
  { label: t('esfuerzo.agente'), caption: t('esfuerzo.agenteDetalle'), value: '' },
  ...props.niveles.map((n) => ({ label: t(`esfuerzo.${n}` as Clave), caption: t(`esfuerzo.${n}Detalle` as Clave), value: n })),
])
</script>
