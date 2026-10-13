<template>
  <div class="pagina apilado">
    <div>
      <div class="titulo-pagina">{{ t('menu.ayuda') }}</div>
      <div class="texto-suave">{{ t('ayuda.subtitulo') }}</div>
    </div>

    <q-card>
      <q-card-section class="row items-center">
        <div class="titulo-seccion col">{{ t('ayuda.empezar') }}</div>
        <q-btn v-if="guiaOculta" flat dense no-caps color="primary" icon="o_visibility" :label="t('ayuda.mostrarGuia')" @click="ocultarGuia(false); $emit('ir', 'inicio')" />
      </q-card-section>
      <q-list>
        <q-item v-for="(p, i) in pasos" :key="p.titulo" class="q-py-md">
          <q-item-section avatar top>
            <div class="icono-suave icono-suave--primary icono-suave--chico"><q-icon :name="p.icono" size="20px" /></div>
          </q-item-section>
          <q-item-section>
            <q-item-label class="text-weight-medium">{{ i + 1 }}. {{ t(p.titulo) }}</q-item-label>
            <q-item-label caption class="q-mt-xs" style="line-height: 1.5">{{ t(p.texto) }}</q-item-label>
          </q-item-section>
          <q-item-section side>
            <q-btn outline color="primary" no-caps :label="t('ayuda.ir', { pagina: t(p.menu) })" class="gt-xs" @click="$emit('ir', p.pagina)" />
            <q-btn flat round color="primary" icon="arrow_forward" class="lt-sm" :aria-label="t('ayuda.ir', { pagina: t(p.menu) })" @click="$emit('ir', p.pagina)" />
          </q-item-section>
        </q-item>
      </q-list>
    </q-card>

    <q-card>
      <q-card-section class="titulo-seccion">{{ t('ayuda.preguntas') }}</q-card-section>
      <q-list separator>
        <q-expansion-item v-for="f in preguntas" :key="f.pregunta" :label="t(f.pregunta)" header-class="text-weight-medium" expand-icon-class="texto-suave">
          <div class="q-px-md q-pb-md texto-suave" style="line-height: 1.6">{{ t(f.respuesta) }}</div>
        </q-expansion-item>
      </q-list>
    </q-card>

    <q-card>
      <q-item clickable tag="a" href="https://github.com/MscottB/Tramtai/blob/main/docs/inicio.md" target="_blank" rel="noopener" class="q-py-md">
        <q-item-section avatar><div class="icono-suave icono-suave--primary"><q-icon name="o_menu_book" size="22px" /></div></q-item-section>
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
import { guiaOculta, ocultarGuia } from '../api'
import { t } from '../i18n'

defineEmits<{ ir: [pagina: string] }>()

const pasos = [
  { titulo: 'ayuda.paso1', texto: 'ayuda.paso1Texto', icono: 'o_smart_toy', pagina: 'agentes', menu: 'menu.agentes' },
  { titulo: 'ayuda.paso2', texto: 'ayuda.paso2Texto', icono: 'o_play_circle', pagina: 'ejecutar', menu: 'menu.ejecutar' },
  { titulo: 'ayuda.paso3', texto: 'ayuda.paso3Texto', icono: 'o_link', pagina: 'conectar', menu: 'menu.conectar' },
  { titulo: 'ayuda.paso4', texto: 'ayuda.paso4Texto', icono: 'o_hub', pagina: 'conectar', menu: 'menu.conectar' },
  { titulo: 'ayuda.paso5', texto: 'ayuda.paso5Texto', icono: 'o_rocket_launch', pagina: 'configuracion', menu: 'menu.configuracion' },
] as const

const preguntas = [
  { pregunta: 'ayuda.p8', respuesta: 'ayuda.r8' },
  { pregunta: 'ayuda.p9', respuesta: 'ayuda.r9' },
  { pregunta: 'ayuda.p1', respuesta: 'ayuda.r1' },
  { pregunta: 'ayuda.p2', respuesta: 'ayuda.r2' },
  { pregunta: 'ayuda.p3', respuesta: 'ayuda.r3' },
  { pregunta: 'ayuda.p4', respuesta: 'ayuda.r4' },
  { pregunta: 'ayuda.p5', respuesta: 'ayuda.r5' },
  { pregunta: 'ayuda.p6', respuesta: 'ayuda.r6' },
  { pregunta: 'ayuda.p7', respuesta: 'ayuda.r7' },
  { pregunta: 'ayuda.p10', respuesta: 'ayuda.r10' },
] as const
</script>
