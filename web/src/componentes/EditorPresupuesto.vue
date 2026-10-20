<template>
  <div>
    <div class="text-caption texto-suave">{{ t('presupuesto.editorTexto') }}</div>
    <BotonAyuda tema="presupuestos" />
    <div v-for="periodo in periodos" :key="periodo" class="row q-col-gutter-sm q-mt-xs">
      <div class="col-12 text-body2">{{ nombrePeriodo[periodo]() }}</div>
      <div v-for="u in unidades" :key="u" class="col-12 col-sm-4">
        <q-input :model-value="modelValue[periodo]?.[u] || null" type="number" min="0" :step="u === 'usd' ? '0.01' : '1'" outlined dense
          :label="nombreUnidad[u]()" :hint="u === 'usd' ? t('presupuesto.usdHint') : undefined"
          @update:model-value="(v: string | number | null) => cambiar(periodo, u, v)" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import BotonAyuda from './ayuda/BotonAyuda.vue'
import type { Limites, Presupuesto } from '../api/tipos'
import { t } from '../i18n'

// EditorPresupuesto edita los límites diarios y mensuales de una API; un campo vacío o 0 es sin límite.
const props = defineProps<{ modelValue: Presupuesto }>()
const emit = defineEmits<{ 'update:modelValue': [valor: Presupuesto] }>()

const periodos = ['diario', 'mensual'] as const
const unidades = ['peticiones', 'tokens', 'usd'] as const
const nombrePeriodo = { diario: () => t('presupuesto.diario'), mensual: () => t('presupuesto.mensual') }
const nombreUnidad = { peticiones: () => t('presupuesto.peticiones'), tokens: () => t('presupuesto.tokens'), usd: () => t('presupuesto.usd') }

function cambiar(periodo: (typeof periodos)[number], unidad: keyof Limites, valor: string | number | null) {
  const numero = Number(valor)
  emit('update:modelValue', { ...props.modelValue, [periodo]: { ...props.modelValue[periodo], [unidad]: numero > 0 ? numero : undefined } })
}
</script>
