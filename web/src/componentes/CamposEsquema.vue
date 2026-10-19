<template>
  <div ref="raiz" class="campos-esquema" :class="{ 'campos-esquema--anidado': nivel > 1 }">
    <div v-for="(c, i) in modelValue" :key="clave(c)" :data-clave="clave(c)" class="campos-esquema__campo">
      <!-- Apilado: la columna de la receta es estrecha también en escritorio. -->
      <div class="row no-wrap items-start q-gutter-x-xs">
        <q-input :model-value="c.nombre" outlined dense class="col" :label="t('campos.nombre')" input-class="codigo"
          @update:model-value="(v) => cambiar(i, { nombre: String(v ?? '').replace(/\s+/g, '_') })" />
        <q-btn v-if="ordenable" flat dense round size="sm" icon="o_arrow_upward" data-accion="subir" class="q-mt-xs" :disable="i === 0" :aria-label="t('campos.subir', { campo: c.nombre || i + 1 })" @click="mover(i, -1)" />
        <q-btn v-if="ordenable" flat dense round size="sm" icon="o_arrow_downward" data-accion="bajar" class="q-mt-xs" :disable="i === modelValue.length - 1" :aria-label="t('campos.bajar', { campo: c.nombre || i + 1 })" @click="mover(i, 1)" />
        <q-btn flat dense round size="sm" icon="o_delete" class="q-mt-xs" :aria-label="t('campos.quitar', { campo: c.nombre || i + 1 })" @click="quitar(i)" />
      </div>
      <div class="row no-wrap items-center q-gutter-x-sm q-mt-sm">
        <q-select :model-value="c.tipo" :options="tipos" emit-value map-options outlined dense class="col" :label="t('campos.tipo')"
          @update:model-value="(v: TipoCampo) => cambiarTipo(i, v)" />
        <q-checkbox :model-value="c.obligatorio" dense :label="t('campos.obligatorio')" @update:model-value="(v: boolean) => cambiar(i, { obligatorio: v })" />
      </div>
      <q-input :model-value="c.descripcion" outlined dense class="q-mt-sm" :label="t('campos.descripcion')" :placeholder="t('campos.descripcionEjemplo')"
        @update:model-value="(v) => cambiar(i, { descripcion: String(v ?? '') })" />
      <q-select v-if="c.tipo === 'opcion'" :model-value="c.opciones ?? []" multiple use-chips use-input hide-dropdown-icon new-value-mode="add-unique"
        input-debounce="0" outlined dense class="q-mt-sm" :label="t('campos.opciones')" :hint="t('campos.opcionesHint')"
        @update:model-value="(v: string[] | null) => cambiar(i, { opciones: (v ?? []).map((o) => o.trim()).filter(Boolean) })" />
      <div v-if="c.tipo === 'lista'" class="row q-col-gutter-sm q-mt-none">
        <div class="col-12">
          <q-select :model-value="c.elemento ?? 'texto'" :options="elementos" emit-value map-options outlined dense :label="t('campos.elemento')"
            @update:model-value="(v: Elemento) => cambiar(i, { elemento: v, campos: v === 'objeto' ? (c.campos?.length ? c.campos : [campoVacio()]) : undefined })" />
        </div>
        <div class="col-6"><q-input :model-value="c.minimo ?? ''" outlined dense inputmode="numeric" :label="t('campos.minimoElementos')"
          @change="(e: Event) => cambiar(i, { minimo: entero(valorDe(e)) })" /></div>
        <div class="col-6"><q-input :model-value="c.maximo ?? ''" outlined dense inputmode="numeric" :label="t('campos.maximoElementos')"
          @change="(e: Event) => cambiar(i, { maximo: entero(valorDe(e)) })" /></div>
      </div>
      <div v-if="c.tipo === 'numero' || c.tipo === 'entero'" class="row q-col-gutter-sm q-mt-none">
        <div class="col-6"><q-input :model-value="c.minimo ?? ''" outlined dense inputmode="decimal" :label="t('campos.minimo')"
          @change="(e: Event) => cambiar(i, { minimo: decimal(valorDe(e)) })" /></div>
        <div class="col-6"><q-input :model-value="c.maximo ?? ''" outlined dense inputmode="decimal" :label="t('campos.maximo')"
          @change="(e: Event) => cambiar(i, { maximo: decimal(valorDe(e)) })" /></div>
      </div>
      <div v-if="c.tipo === 'objeto' || (c.tipo === 'lista' && c.elemento === 'objeto')" class="q-mt-sm">
        <div class="text-caption texto-suave q-mb-xs">{{ t('campos.subcampos', { campo: c.nombre || '…' }) }}</div>
        <CamposEsquema :model-value="c.campos ?? []" :nivel="nivel + 1" :ordenable="ordenable" @update:model-value="(v: Campo[]) => cambiar(i, { campos: v })" />
      </div>
    </div>
    <q-btn flat dense no-caps color="primary" icon="o_add" :label="nivel > 1 ? t('campos.anadirSubcampo') : t('campos.anadir')" @click="anadir" />
  </div>
</template>

<script setup lang="ts">
// CamposEsquema edita una lista de campos de «Respuesta con forma»; los objetos y las listas de objetos se editan con
// este mismo componente, hasta NIVELES niveles.
import { computed, nextTick, ref, toRaw } from 'vue'
import { campoVacio, NIVELES, type Campo, type Elemento, type TipoCampo } from '../esquemaCampos'
import { t } from '../i18n'

// ordenable: en una API guardada la configuración conserva los campos por orden alfabético, así que no se ofrece
// reordenarlos (en Probar el esquema viaja tal cual y el orden sí se mantiene).
const props = defineProps<{ modelValue: Campo[]; nivel: number; ordenable?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [valor: Campo[]] }>()

// En el último nivel no se ofrecen objetos: no cabrían sus subcampos.
const tipos = computed(() => (['texto', 'numero', 'entero', 'si_no', 'fecha', 'opcion', 'lista', 'objeto'] as const)
  .filter((v) => v !== 'objeto' || props.nivel < NIVELES).map((value) => ({ value, label: t(`campos.tipo.${value}`) })))
const elementos = computed(() => (['texto', 'numero', 'objeto'] as const)
  .filter((v) => v !== 'objeto' || props.nivel < NIVELES).map((value) => ({ value, label: t(`campos.elemento.${value}`) })))

const emitir = (lista: Campo[]) => emit('update:modelValue', lista)
const cambiar = (i: number, parcial: Partial<Campo>) => emitir(props.modelValue.map((c, j) => (j === i ? sigue(c, limpio({ ...c, ...parcial })) : c)))
const anadir = () => emitir([...props.modelValue, campoVacio()])
const quitar = (i: number) => emitir(props.modelValue.filter((_, j) => j !== i))
// mover devuelve el foco al botón con el que se movió el campo (el navegador lo pierde al mover la fila); si en la
// nueva posición ese botón queda desactivado, al otro.
const raiz = ref<HTMLElement>()
function mover(i: number, paso: number) {
  const lista = [...props.modelValue]
  ;[lista[i], lista[i + paso]] = [lista[i + paso], lista[i]]
  const fila = clave(props.modelValue[i])
  emitir(lista)
  nextTick(() => {
    const botones = [...(raiz.value?.querySelectorAll<HTMLButtonElement>(`:scope > [data-clave="${fila}"] [data-accion]`) ?? [])]
    const accion = paso < 0 ? 'subir' : 'bajar'
    const boton = botones.find((b) => b.dataset.accion === accion && !b.disabled) ?? botones.find((b) => !b.disabled)
    boton?.focus()
  })
}
// Al cambiar de tipo se quita lo que el tipo nuevo no usa y se prepara lo que necesita.
function cambiarTipo(i: number, tipo: TipoCampo) {
  const c = props.modelValue[i]
  const nuevo: Campo = { nombre: c.nombre, tipo, obligatorio: c.obligatorio, descripcion: c.descripcion }
  if (tipo === 'opcion') nuevo.opciones = c.opciones ?? []
  if (tipo === 'lista') nuevo.elemento = 'texto'
  if (tipo === 'objeto') nuevo.campos = c.campos?.length ? c.campos : [campoVacio()]
  emitir(props.modelValue.map((x, j) => (j === i ? sigue(c, nuevo) : x)))
}
// Cada campo tiene una clave estable (no su posición): al subirlo o bajarlo, su fila y el botón con el foco se mueven
// con él. Un campo cambiado es un objeto nuevo que sigue con la clave del anterior; la clave no llega al esquema.
const claves = new WeakMap<Campo, number>()
let ultima = 0
function clave(c: Campo): number {
  // toRaw: el campo llega como proxy reactivo, y la clave se guarda con el objeto de verdad.
  let n = claves.get(toRaw(c))
  if (n === undefined) claves.set(toRaw(c), (n = ++ultima))
  return n
}
function sigue(antes: Campo, despues: Campo): Campo {
  claves.set(toRaw(despues), clave(antes))
  return despues
}
// limpio quita las propiedades sin valor: el campo queda igual que el que rehace desdeEsquema.
function limpio(c: Campo): Campo {
  return Object.fromEntries(Object.entries(c).filter(([, v]) => v !== undefined)) as unknown as Campo
}
// Los límites se leen al salir del cuadro (change): mientras se escribe, «0.» o «-» aún no son un número.
const valorDe = (e: Event) => (e.target as HTMLInputElement).value
const entero = (v: string | number | null) => (v === null || !/^\d+$/.test(String(v)) ? undefined : Number(v))
const decimal = (v: string | number | null) => (v === null || String(v).trim() === '' || Number.isNaN(Number(v)) ? undefined : Number(v))
</script>

<style scoped>
.campos-esquema__campo {
  border: 1px solid var(--borde);
  border-radius: var(--radio);
  padding: 10px;
  margin-bottom: 8px;
}
.campos-esquema--anidado {
  border-left: 2px solid var(--borde);
  padding-left: 10px;
}
</style>
