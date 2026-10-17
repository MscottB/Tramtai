<template>
  <q-select :model-value="modelValue" :options="filtradas" emit-value map-options outlined dense options-dense use-input input-debounce="0"
    :multiple="multiple" :max-values="maximo" :hint="hint ?? t('modelo.libreHint')" :error="!!aviso" :error-message="aviso" @filter="filtrar" @new-value="nuevo"
    @update:model-value="(v: string | string[] | null) => emit('update:modelValue', v)">
    <template #prepend><q-icon :name="icono ?? 'o_smart_toy'" /></template>
    <template #selected-item="scope">
      <q-chip v-if="multiple" removable dense :tabindex="scope.tabindex" class="q-ml-none" @remove="scope.removeAtIndex(scope.index)">
        {{ scope.opt.label }}<span v-if="scope.opt.version" class="texto-suave q-ml-xs">{{ scope.opt.version }}</span>
      </q-chip>
      <div v-else class="column no-wrap q-py-xs" style="min-width: 0; max-width: 100%">
        <span class="ellipsis">{{ scope.opt.label }}</span>
        <span class="text-caption texto-suave ellipsis">{{ scope.opt.caption }}</span>
      </div>
    </template>
    <template #option="scope">
      <q-item-label v-if="scope.opt.cabecera" header class="row items-center no-wrap q-py-sm cabecera-modelo">
        <span v-if="scope.opt.grupo !== 'perfil'" class="linea__chapa q-mr-sm" :class="'linea--' + scope.opt.grupo" aria-hidden="true">{{ codigoLinea(scope.opt.grupo) }}</span>
        <q-icon v-else name="o_tune" size="18px" class="q-mr-sm" />{{ scope.opt.label }}
      </q-item-label>
      <q-item v-else v-bind="scope.itemProps" class="q-pl-lg">
        <q-item-section>
          <q-item-label>{{ scope.opt.corto }}</q-item-label>
          <q-item-label caption>{{ scope.opt.caption }}</q-item-label>
        </q-item-section>
        <q-item-section side class="row no-wrap items-center q-gutter-x-xs">
          <span v-if="scope.opt.predeterminado" class="etiqueta etiqueta--info">{{ t('modelo.esPredeterminado') }}</span>
          <q-icon v-if="scope.opt.esfuerzos.length" name="o_psychology" size="16px" :aria-label="t('esfuerzo.elegible')"><q-tooltip>{{ t('esfuerzo.elegible') }}</q-tooltip></q-icon>
        </q-item-section>
      </q-item>
    </template>
  </q-select>
</template>

<script setup lang="ts">
// SelectorModelo elige un modelo del catálogo (o varios, para el respaldo), agrupado por agente y con la versión
// exacta de cada uno; también acepta uno escrito como agente/modelo (por ejemplo una versión que aún no se usó).
import { computed, ref } from 'vue'
import { agruparOpciones, codigoLinea, opcionEscrita, type OpcionModelo } from '../api'
import { t } from '../i18n'

const props = defineProps<{ modelValue: string | string[] | null; opciones: OpcionModelo[]; multiple?: boolean; maximo?: number; hint?: string; icono?: string }>()
const emit = defineEmits<{ 'update:modelValue': [valor: string | string[] | null] }>()

const agentes = computed(() => new Set(props.opciones.map((o) => o.grupo).filter((g) => g !== 'perfil')))
const elegidos = computed(() => (Array.isArray(props.modelValue) ? props.modelValue : props.modelValue ? [props.modelValue] : []))
// todas añade los valores escritos a mano, que no están en el catálogo, para que se vean como una opción más.
const todas = computed(() => [...props.opciones, ...elegidos.value.filter((v) => !props.opciones.some((o) => o.value === v)).map(opcionEscrita)])
const filtro = ref('')
const filtradas = computed(() => agruparOpciones(todas.value.filter((o) => `${o.label} ${o.caption} ${o.value}`.toLowerCase().includes(filtro.value))))
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

<style scoped>
.cabecera-modelo {
  font-weight: 600;
  color: var(--texto);
}
</style>
