<template>
  <div class="q-gutter-y-md">
    <q-input :model-value="modelValue.instrucciones" type="textarea" autogrow outlined dense :label="t('receta.instrucciones')"
      :placeholder="t('receta.instruccionesEjemplo')" :hint="t('receta.instruccionesHint')"
      @update:model-value="(v) => cambiar({ instrucciones: String(v ?? '') })" />
    <SelectorModelo :model-value="modelValue.modelo" :opciones="opciones" clearable :label="t('receta.modelo')"
      @update:model-value="(v) => cambiar({ modelo: v, forzar: v ? modelValue.forzar : false })" />
    <q-toggle :model-value="modelValue.forzar" :disable="!modelValue.modelo" :label="t('receta.forzar')"
      @update:model-value="(v: boolean) => cambiar({ forzar: v })" />
    <SelectorEsfuerzo :model-value="modelValue.esfuerzo" :niveles="niveles" :hint="paraApi ? t('esfuerzo.api') : undefined"
      @update:model-value="(v: string) => cambiar({ esfuerzo: v })" />
    <q-select :model-value="modelValue.respaldo" :options="opcionesRespaldo" multiple use-chips emit-value map-options outlined dense
      :max-values="3" :label="t('receta.respaldo')" :hint="t('receta.respaldoHint')"
      @update:model-value="(v: string[] | null) => cambiar({ respaldo: v ?? [] })">
      <template #prepend><q-icon name="o_alt_route" /></template>
    </q-select>
    <div>
      <q-toggle :model-value="estructurada" :label="t('receta.salidaEstructurada')" @update:model-value="alternarEsquema" />
      <div class="text-caption texto-suave q-pl-sm">{{ t('receta.salidaEstructuradaTexto') }}</div>
    </div>
    <q-input v-if="estructurada" :model-value="modelValue.esquema" type="textarea" autogrow outlined dense input-class="codigo"
      :label="t('receta.esquema')" :error="!!errorEsquema" :error-message="errorEsquema" :hint="t('receta.esquemaHint')"
      @update:model-value="(v) => cambiar({ esquema: String(v ?? '') })" />
  </div>
</template>

<script setup lang="ts">
// EditorReceta edita lo que define una API (instrucciones, modelo, esfuerzo, respaldo y esquema); lo comparten
// Probar, donde se prueba, y Mis APIs, donde se guarda.
import { computed, watch } from 'vue'
import SelectorEsfuerzo from './SelectorEsfuerzo.vue'
import SelectorModelo from './SelectorModelo.vue'
import { esfuerzosDe, esquemaDeReceta, opcionModelo, type Modelo, type Receta } from '../api'
import { t } from '../i18n'

// paraApi: en una API sin modelo fijo la aplicación puede elegirlo, así que se ofrecen todos los niveles de esfuerzo.
const props = defineProps<{ modelValue: Receta; catalogo: Modelo[]; paraApi?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [valor: Receta] }>()

const cambiar = (parcial: Partial<Receta>) => emit('update:modelValue', { ...props.modelValue, ...parcial })

const opciones = computed(() => props.catalogo.map(opcionModelo))
// El respaldo es agente o agente/modelo: un perfil no sirve porque da un modelo distinto por agente.
const opcionesRespaldo = computed(() => props.catalogo.filter((m) => !m.perfil).map(opcionModelo))

const todosLosNiveles = computed(() => props.catalogo.reduce<string[]>((a, m) => ((m.esfuerzos?.length ?? 0) > a.length ? m.esfuerzos! : a), []))
const niveles = computed(() => (!props.modelValue.modelo && props.paraApi ? todosLosNiveles.value : esfuerzosDe(props.catalogo, props.modelValue.modelo)))
// Un nivel que el modelo nuevo no admite vuelve al del agente.
watch(niveles, (n) => {
  if (props.modelValue.esfuerzo && !n.includes(props.modelValue.esfuerzo)) cambiar({ esfuerzo: '' })
})

const ejemploEsquema = `{
  "type": "object",
  "properties": {
    "resumen": { "type": "string" }
  },
  "required": ["resumen"]
}`
const estructurada = computed(() => props.modelValue.esquema !== '')
const alternarEsquema = (activa: boolean) => cambiar({ esquema: activa ? ejemploEsquema : '' })
const errorEsquema = computed(() => {
  try {
    esquemaDeReceta(props.modelValue)
    return ''
  } catch (e) {
    return (e as Error).message
  }
})
</script>
