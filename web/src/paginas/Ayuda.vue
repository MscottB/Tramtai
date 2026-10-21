<template>
  <div class="pagina apilado">
    <CabeceraPagina :titulo="actual ? t(actual.titulo) : t('menu.ayuda')" :descripcion="actual ? t(actual.resumen) : t('ayuda.subtitulo')">
      <template v-if="!actual && guiaOculta" #acciones>
        <q-btn flat dense no-caps color="primary" icon="o_visibility" :label="t('ayuda.mostrarGuia')" @click="ocultarGuia(false); $emit('ir', 'inicio')" />
      </template>
    </CabeceraPagina>

    <q-input v-model="busqueda" outlined dense clearable debounce="150" :placeholder="t('ayuda.buscar')" :aria-label="t('ayuda.buscar')">
      <template #prepend><q-icon name="search" /></template>
    </q-input>

    <div class="solo-lectores" aria-live="polite">{{ texto ? t('ayuda.ui.resultados', { n: resultados.length }) : '' }}</div>
    <div>
      <template v-if="texto">
        <q-card v-if="resultados.length">
          <q-list separator>
            <q-item v-for="r in resultados" :key="r.tema + r.ancla" clickable @click="abrir(r.tema, r.ancla)">
              <q-item-section>
                <q-item-label class="text-weight-medium">{{ r.donde }}</q-item-label>
                <q-item-label v-if="r.donde !== r.titulo" caption>{{ r.titulo }}</q-item-label>
              </q-item-section>
              <q-item-section side><q-icon name="arrow_forward" /></q-item-section>
            </q-item>
          </q-list>
        </q-card>
        <q-card v-else><q-card-section class="texto-suave">{{ t('ayuda.sinResultados', { texto }) }}</q-card-section></q-card>
      </template>
    </div>

    <template v-if="!texto">
      <q-card v-if="actual">
        <q-card-section>
          <VistaTema :tema="actual" contexto="app" sin-cabecera :ancla="ancla" @ir="(d) => $emit('ir', d)" @recorrido="(p) => $emit('mostrar', p)" />
        </q-card-section>
      </q-card>
      <TarjetasTemas v-else :temas="temas" @elegir="(id) => abrir(id)" />
    </template>

    <q-card v-if="!actual">
      <q-item clickable tag="a" href="https://mscottb.github.io/Tramtai/desarrolladores.html" target="_blank" rel="noopener" class="q-py-md">
        <q-item-section avatar><q-icon name="o_code" color="primary" size="24px" /></q-item-section>
        <q-item-section>
          <q-item-label class="text-weight-medium">{{ t('ayuda.desarrolladores') }}</q-item-label>
          <q-item-label caption>{{ t('ayuda.desarrolladoresTexto') }}</q-item-label>
        </q-item-section>
        <q-item-section side><q-icon name="open_in_new" /></q-item-section>
      </q-item>
      <q-separator />
      <q-item clickable tag="a" href="https://github.com/MscottB/Tramtai/blob/main/docs/inicio.md" target="_blank" rel="noopener" class="q-py-md">
        <q-item-section avatar><q-icon name="o_menu_book" color="primary" size="24px" /></q-item-section>
        <q-item-section>
          <q-item-label class="text-weight-medium">{{ t('ayuda.documentacion') }}</q-item-label>
          <q-item-label caption>{{ t('ayuda.documentacionTexto') }}</q-item-label>
        </q-item-section>
        <q-item-section side><q-icon name="open_in_new" /></q-item-section>
      </q-item>
    </q-card>
  </div>
</template>

<script setup lang="ts">
// Ayuda es el inicio por temas (tarjetas y buscador) o un tema (#ayuda/<tema>); los temas son los datos de
// web/src/ayuda, los mismos que genera el sitio.
import { computed, ref, watch } from 'vue'
import { tituloError } from '../api/cliente'
import { guiaOculta, ocultarGuia } from '../estado'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import TarjetasTemas from '../componentes/ayuda/TarjetasTemas.vue'
import VistaTema from '../componentes/ayuda/VistaTema.vue'
import { idioma, idiomas, t } from '../i18n'
import { temas, temaPorId } from '../ayuda/indice'
import { buscar } from '../ayuda/buscar'
import type { Pagina } from '../ayuda/tipos'

const props = defineProps<{ tema?: string }>()
// mostrar (y no recorrido, que en App.vue es la bienvenida de Primeros pasos) lleva a una página y empieza su recorrido.
const emit = defineEmits<{ ir: [pagina: string]; mostrar: [pagina: Pagina] }>()

const busqueda = ref<string | null>('')
const texto = computed(() => (busqueda.value ?? '').trim())
const resultados = computed(() => buscar(texto.value, temas, idiomas[idioma.value].textos, tituloError))
const actual = computed(() => (props.tema ? temaPorId(props.tema) : undefined))
const ancla = ref('')
// Al llegar a otro tema por Volver o por los relacionados, la pregunta buscada antes ya no cuenta.
watch(() => props.tema, () => { if (!abriendo) ancla.value = ''; abriendo = false })
let abriendo = false

// abrir lleva a un tema (y a una pregunta o error dentro de él) y deja de buscar.
function abrir(tema: string, a = '') {
  ancla.value = a
  abriendo = tema !== props.tema
  busqueda.value = ''
  emit('ir', 'ayuda/' + tema)
}
</script>
