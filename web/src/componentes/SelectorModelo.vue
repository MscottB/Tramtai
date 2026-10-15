<template>
  <q-select :model-value="modelValue" :options="filtradas" emit-value map-options outlined dense options-dense use-input input-debounce="0"
    :hint="t('modelo.libreHint')" :error="!!aviso" :error-message="aviso" @filter="filtrar" @new-value="nuevo"
    @update:model-value="(v: string | null) => emit('update:modelValue', v)">
    <template #prepend><q-icon name="o_smart_toy" /></template>
    <template #selected-item>
      <div v-if="seleccionada" class="column no-wrap q-py-xs" style="min-width: 0; max-width: 100%">
        <span class="ellipsis">{{ seleccionada.label }}</span>
        <span class="text-caption texto-suave ellipsis">{{ seleccionada.caption }}</span>
      </div>
    </template>
    <template #option="scope">
      <q-item v-bind="scope.itemProps">
        <q-item-section>
          <q-item-label>{{ scope.opt.label }}</q-item-label>
          <q-item-label caption>{{ scope.opt.caption }}</q-item-label>
        </q-item-section>
        <q-item-section v-if="scope.opt.predeterminado" side><span class="etiqueta etiqueta--info">{{ t('modelo.esPredeterminado') }}</span></q-item-section>
      </q-item>
    </template>
  </q-select>
</template>

<script setup lang="ts">
// SelectorModelo elige un modelo del catálogo mostrando su versión, o acepta uno escrito como agente/modelo
// (por ejemplo una versión concreta que aún no se usó).
import { computed, ref } from 'vue'
import { etiquetaModelo, type OpcionModelo } from '../api'
import { t } from '../i18n'

const props = defineProps<{ modelValue: string | null; opciones: OpcionModelo[] }>()
const emit = defineEmits<{ 'update:modelValue': [valor: string | null] }>()

const agentes = computed(() => new Set(props.opciones.map((o) => o.value.split('/')[0]).filter((a) => a && a !== 'perfil')))
// todas añade el valor escrito a mano, que no está en el catálogo, para que se vea como una opción más.
const todas = computed(() => {
  const v = props.modelValue
  if (!v || props.opciones.some((o) => o.value === v)) return props.opciones
  return [...props.opciones, { label: etiquetaModelo(v), caption: t('modelo.escrito'), value: v, predeterminado: false, esfuerzos: [] }]
})
const seleccionada = computed(() => todas.value.find((o) => o.value === props.modelValue))
const filtro = ref('')
const filtradas = computed(() => todas.value.filter((o) => `${o.label} ${o.caption} ${o.value}`.toLowerCase().includes(filtro.value)))
const aviso = ref('')

function filtrar(texto: string, actualizar: (f: () => void) => void) {
  actualizar(() => {
    filtro.value = texto.trim().toLowerCase()
    aviso.value = ''
  })
}

function nuevo(texto: string, hecho: (valor?: string, modo?: 'add-unique') => void) {
  const valor = texto.trim()
  const [agente, ...resto] = valor.split('/')
  if (todas.value.some((o) => o.value === valor) || (agentes.value.has(agente) && resto.join('/'))) {
    hecho(valor, 'add-unique')
  } else {
    aviso.value = t('modelo.libreRegla', { agentes: [...agentes.value].join(', ') })
    hecho()
  }
}
</script>
