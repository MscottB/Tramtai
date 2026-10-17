<template>
  <div class="q-gutter-y-md">
    <q-input :model-value="modelValue.instrucciones" type="textarea" autogrow outlined dense :label="t('receta.instrucciones')"
      :placeholder="t('receta.instruccionesEjemplo')" :hint="t('receta.instruccionesHint')"
      @update:model-value="(v) => cambiar({ instrucciones: String(v ?? '') })" />
    <!-- El esfuerzo va justo bajo el modelo, fuera de las opciones: se ve siempre cuál se usará, también el del agente.
         En una fila no cabe: la columna de la receta es estrecha también en escritorio. -->
    <SelectorModelo :model-value="modelValue.modelo" :opciones="opciones" clearable :label="t('receta.modelo')"
      @update:model-value="(v) => cambiar({ modelo: v as string | null, forzar: v ? modelValue.forzar : false })" />
    <div v-if="sinAntigravity" class="text-caption texto-suave q-mt-xs">{{ t('receta.sinAntigravity') }}</div>
    <SelectorEsfuerzo :model-value="modelValue.esfuerzo" :niveles="niveles" :predeterminado="esfuerzoPredeterminadoDe(catalogo, modelValue.modelo)" :hint="paraApi ? t('esfuerzo.api') : undefined"
      @update:model-value="(v: string) => cambiar({ esfuerzo: v })" />
    <component :is="plegarOpciones ? QExpansionItem : 'div'" v-bind="plegarOpciones ? plegable : {}">
    <div class="q-gutter-y-md" :class="{ 'q-pt-sm': plegarOpciones }">
    <q-toggle :model-value="modelValue.forzar" :disable="!modelValue.modelo" :label="t('receta.forzar')"
      @update:model-value="(v: boolean) => cambiar({ forzar: v })" />
    <SelectorModelo :model-value="basesRespaldo" :opciones="opcionesRespaldo" multiple :maximo="3" icono="o_alt_route"
      :label="t('receta.respaldo')" :hint="t('receta.respaldoHint')" @update:model-value="(v) => elegirRespaldo((v as string[] | null) ?? [])" />
    <!-- Cada agente de respaldo puede llevar su propio esfuerzo; sin elegir, el del principal o el de su CLI. -->
    <div v-for="(r, i) in respaldoDividido" :key="r.base" class="q-pl-md">
      <SelectorEsfuerzo v-if="r.niveles.length" :model-value="r.esfuerzo" :niveles="modelValue.esfuerzo ? ['agente', ...r.niveles] : r.niveles" :etiqueta="t('receta.esfuerzoDe', { agente: r.nombre })"
        :vacio="vacioRespaldo(r.base, r.niveles)" hint="" @update:model-value="(v: string) => esfuerzoRespaldo(i, v)" />
      <div v-else class="text-caption texto-suave row items-center no-wrap">
        <q-icon name="o_psychology" size="18px" class="q-mr-xs" />{{ t('receta.respaldoSinEsfuerzo', { agente: r.nombre }) }}
      </div>
    </div>
    <div>
      <q-toggle :model-value="estructurada" :label="t('receta.salidaEstructurada')" @update:model-value="alternarEsquema" />
      <div class="text-caption texto-suave q-pl-sm">{{ t('receta.salidaEstructuradaTexto') }}</div>
    </div>
    <q-input v-if="estructurada" :model-value="modelValue.esquema" type="textarea" autogrow outlined dense input-class="codigo"
      :label="t('receta.esquema')" :error="!!errorEsquema" :error-message="errorEsquema" :hint="t('receta.esquemaHint')"
      @update:model-value="(v) => escribirEsquema(String(v ?? ''))" />
    </div>
    </component>
  </div>
</template>

<script setup lang="ts">
// EditorReceta edita lo que define una API (instrucciones, modelo, esfuerzo, respaldo y esquema); lo comparten
// Probar, donde se prueba, y Mis APIs, donde se guarda.
import { computed, ref, watch } from 'vue'
import { QExpansionItem } from 'quasar'
import SelectorEsfuerzo from './SelectorEsfuerzo.vue'
import SelectorModelo from './SelectorModelo.vue'
import { dividirRespaldo, esfuerzoPredeterminadoDe, esfuerzosDe, esquemaDeReceta, nombreEsfuerzo, opcionModelo, unirRespaldo, type Modelo,
  type Receta } from '../api'
import { t } from '../i18n'

// paraApi: en una API sin modelo fijo la aplicación puede elegirlo, así que se ofrecen todos los niveles de esfuerzo.
// plegarOpciones (Probar): forzar modelo, esfuerzo, respaldo y salida JSON van en «Opciones de la receta», abierta si
// alguna tiene valor, para dejar arriba lo esencial.
const props = defineProps<{ modelValue: Receta; catalogo: Modelo[]; paraApi?: boolean; plegarOpciones?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [valor: Receta] }>()

const cambiar = (parcial: Partial<Receta>) => emit('update:modelValue', { ...props.modelValue, ...parcial })
const conOpciones = computed(() => props.modelValue.forzar || props.modelValue.respaldo.length > 0 || !!props.modelValue.esquema)
// La receta puede llegar después de montar (Abrir en Probar): se abre en cuanto alguna opción tiene valor.
const abierta = ref(conOpciones.value)
watch(conOpciones, (v) => v && (abierta.value = true))
const plegable = computed(() => ({ label: t('receta.opciones'), icon: 'o_tune', dense: true, modelValue: abierta.value,
  'onUpdate:modelValue': (v: boolean) => (abierta.value = v), headerClass: 'q-px-none text-body2' }))

// Una API no ofrece Antigravity (ni como agente ni de respaldo): hasta comprobarlo con el CLI real, solo se usa desde
// Probar y la terminal. El servicio lo rechaza igual al guardar.
const sinAntigravity = computed(() => !!props.paraApi && props.catalogo.some((m) => m.agente === 'antigravity'))
const usables = computed(() => (props.paraApi ? props.catalogo.filter((m) => m.agente !== 'antigravity') : props.catalogo))
const opciones = computed(() => usables.value.map(opcionModelo))
// El respaldo es agente o agente/modelo: un perfil no sirve porque da un modelo distinto por agente.
const opcionesRespaldo = computed(() => usables.value.filter((m) => !m.perfil).map(opcionModelo))
// Las entradas de respaldo son agente[/modelo][+esfuerzo]: el selector elige las bases y cada una conserva su esfuerzo.
// «agente» (el esfuerzo de su propio CLI) solo se distingue de «igual que el principal» si el principal tiene uno; si no,
// son lo mismo y se muestra como el vacío.
const respaldoDividido = computed(() => props.modelValue.respaldo.map((e) => {
  const r = dividirRespaldo(e)
  if (r.esfuerzo === 'agente' && !props.modelValue.esfuerzo) r.esfuerzo = ''
  return { ...r, niveles: esfuerzosDe(props.catalogo, r.base), nombre: opcionesRespaldo.value.find((o) => o.value === r.base)?.label ?? r.base }
}))
const basesRespaldo = computed(() => respaldoDividido.value.map((r) => r.base))
function elegirRespaldo(bases: string[]) {
  cambiar({ respaldo: bases.map((b) => props.modelValue.respaldo.find((e) => dividirRespaldo(e).base === b) ?? b) })
}
function esfuerzoRespaldo(i: number, esfuerzo: string) {
  cambiar({ respaldo: props.modelValue.respaldo.map((e, j) => (j === i ? unirRespaldo(dividirRespaldo(e).base, esfuerzo) : e)) })
}
// vacioRespaldo dice qué esfuerzo usa un respaldo sin uno propio: el del principal (ajustado al máximo de ese agente,
// como hace el servicio) o, si no hay, el de su CLI.
function vacioRespaldo(base: string, niveles: string[]): string {
  const principal = props.modelValue.esfuerzo
  if (principal) return t('receta.respaldoIgual', { nivel: nombreEsfuerzo(niveles.includes(principal) ? principal : niveles.at(-1)).toLowerCase() })
  const p = esfuerzoPredeterminadoDe(props.catalogo, base)
  return p ? t('esfuerzo.agenteCon', { nivel: nombreEsfuerzo(p).toLowerCase() }) : t('esfuerzo.agente')
}

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
// estructurada no depende del texto: borrarlo mientras se escribe no debe apagar el interruptor ni ocultar el campo.
// Solo se sincroniza con un esquema que llega de fuera (una receta cargada); el que se escribe aquí no lo toca.
const estructurada = ref(props.modelValue.esquema !== '')
let escrito = props.modelValue.esquema
watch(() => props.modelValue.esquema, (e) => {
  if (e !== escrito) estructurada.value = e !== ''
  escrito = e
})
function escribirEsquema(texto: string) {
  escrito = texto
  cambiar({ esquema: texto })
}
function alternarEsquema(activa: boolean) {
  estructurada.value = activa
  escribirEsquema(activa ? props.modelValue.esquema || ejemploEsquema : '')
}
const errorEsquema = computed(() => {
  if (!props.modelValue.esquema.trim()) return t('receta.esquemaVacio')
  try {
    esquemaDeReceta(props.modelValue)
    return ''
  } catch (e) {
    return (e as Error).message
  }
})
</script>
