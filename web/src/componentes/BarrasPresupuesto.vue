<template>
  <div class="q-mt-xs" style="max-width: 420px" data-captura="presupuesto">
    <div v-for="b in barras" :key="b.clave" class="q-mb-xs">
      <div class="row text-caption"><span>{{ b.titulo }}</span><q-space /><span>{{ b.texto }}</span></div>
      <q-linear-progress :value="Math.min(b.fraccion, 1)" rounded size="6px" :aria-label="b.titulo + ': ' + b.texto"
        :color="b.fraccion >= 1 ? 'negative' : b.fraccion >= 0.8 ? 'warning' : 'positive'" />
    </div>
    <div class="row items-center text-caption texto-suave">
      <span>{{ t('presupuesto.reinicios', { dia: dia(estado.diario.reinicio), mes: dia(estado.mensual.reinicio) }) }}</span>
      <q-btn v-if="!sinBoton" flat dense no-caps size="sm" color="primary" icon="o_restart_alt" class="q-ml-sm" :label="t('presupuesto.reiniciar')" @click="emit('reiniciar')" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { EstadoPresupuesto } from '../api/tipos'
import { idioma, t } from '../i18n'
import { decimal, numero } from '../formato'

// BarrasPresupuesto muestra una barra por límite definido: verde, amarilla desde el 80 % y roja al 100 %.
// sinBoton quita «Reiniciar consumo» cuando ya está en otro sitio (el menú de la API).
const props = defineProps<{ estado: EstadoPresupuesto; sinBoton?: boolean }>()
const emit = defineEmits<{ reiniciar: [] }>()

// Los periodos reinician a medianoche: basta la fecha.
const dia = (texto: string) => new Date(texto).toLocaleDateString(idioma.value)
const cifra = (n: number, usd: boolean) => (usd ? '$' + decimal(n, 2) : numero(n))
const barras = computed(() => {
  const lista = []
  for (const periodo of ['diario', 'mensual'] as const) {
    const { limites, consumo } = props.estado[periodo]
    for (const unidad of ['peticiones', 'tokens', 'usd'] as const) {
      const limite = limites[unidad] ?? 0
      if (limite <= 0) continue
      const usado = consumo[unidad]
      lista.push({
        clave: periodo + unidad,
        titulo: (periodo === 'diario' ? t('presupuesto.diario') : t('presupuesto.mensual')) + ' · ' +
          ({ peticiones: t('presupuesto.peticiones'), tokens: t('presupuesto.tokens'), usd: t('presupuesto.usd') })[unidad],
        texto: cifra(usado, unidad === 'usd') + ' / ' + cifra(limite, unidad === 'usd'),
        fraccion: usado / limite,
      })
    }
  }
  return lista
})
</script>
