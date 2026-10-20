<template>
  <div class="pagina apilado">
    <CabeceraPagina :titulo="t('menu.ayuda')" :descripcion="t('ayuda.subtitulo')" />

    <div>
      <q-input v-model="busqueda" outlined dense clearable debounce="150" :placeholder="t('ayuda.buscar')" :aria-label="t('ayuda.buscar')">
        <template #prepend><q-icon name="search" /></template>
      </q-input>
      <div v-if="!texto" class="row q-gutter-xs q-mt-sm">
        <q-chip v-for="s in secciones" :key="s.id" clickable outline dense color="primary" :icon="s.icono" @click="irA(s.id)">{{ t(s.titulo) }}</q-chip>
      </div>
    </div>

    <template v-if="!texto">
      <q-card id="ayuda-funciona">
        <q-card-section>
          <div class="titulo-seccion">{{ t('ayuda.secFunciona') }}</div>
          <div class="texto-suave text-body2 q-mt-xs">{{ t('ayuda.funcionaTexto') }}</div>
        </q-card-section>
        <q-card-section class="q-pt-none">
          <ol class="recorrido">
            <li v-for="(e, i) in recorrido" :key="e.titulo" class="recorrido__parada">
              <q-icon :name="e.icono" size="22px" color="primary" />
              <div class="text-weight-medium q-mt-xs">{{ i + 1 }}. {{ t(e.titulo) }}</div>
              <div class="text-caption texto-suave">{{ t(e.texto) }}</div>
            </li>
          </ol>
          <q-banner rounded dense class="info-caja q-mt-md">
            <template #avatar><q-icon name="o_shield" color="primary" /></template>{{ t('ayuda.funcionaNota') }}
          </q-banner>
        </q-card-section>
      </q-card>

      <q-card id="ayuda-archivos">
        <q-card-section>
          <div class="titulo-seccion">{{ t('ayuda.secArchivos') }}</div>
          <div class="texto-suave text-body2 q-mt-xs">{{ t('ayuda.archivosTexto') }}</div>
        </q-card-section>
        <q-card-section class="q-pt-none">
          <ol class="recorrido">
            <li v-for="(e, i) in recorridoArchivo" :key="e.titulo" class="recorrido__parada">
              <q-icon :name="e.icono" size="22px" color="primary" />
              <div class="text-weight-medium q-mt-xs">{{ i + 1 }}. {{ t(e.titulo) }}</div>
              <div class="text-caption texto-suave">{{ t(e.texto) }}</div>
            </li>
          </ol>
          <ul class="text-body2 q-mt-md q-mb-none q-pl-lg">
            <li v-for="n in notasArchivo" :key="n" class="q-mb-xs">{{ t(n) }}</li>
          </ul>
          <div class="text-weight-medium q-mt-md">{{ t('ayuda.archivosEjemplos') }}</div>
          <div class="desplazable-x q-mt-xs"><q-btn-toggle v-model="ejemploArchivo" no-caps flat dense toggle-color="primary" class="segmentado inline-flex"
            :options="ejemplosArchivo.map((e) => ({ label: e.titulo(), value: e.id }))" /></div>
          <div class="bloque-codigo sin-ajuste q-mt-sm">{{ codigoArchivo }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar"
            :aria-label="t('conectar.copiarEjemplo')" @click="copiar(codigoArchivo, t('conectar.ejemploCopiado'))" /></div>
        </q-card-section>
      </q-card>

      <q-card id="ayuda-apis">
        <q-card-section>
          <div class="titulo-seccion">{{ t('ayuda.secApis') }}</div>
          <div class="texto-suave text-body2 q-mt-xs">{{ t('ayuda.apisTexto') }}</div>
        </q-card-section>
        <q-card-section class="q-pt-none"><TablaApis /></q-card-section>
      </q-card>

      <q-card id="ayuda-empezar">
        <q-card-section class="row items-center">
          <div class="titulo-seccion col">{{ t('ayuda.empezar') }}</div>
          <q-btn v-if="guiaOculta" flat dense no-caps color="primary" icon="o_visibility" :label="t('ayuda.mostrarGuia')" @click="ocultarGuia(false); $emit('ir', 'inicio')" />
        </q-card-section>
        <q-list>
          <q-item v-for="(p, i) in pasos" :key="p.titulo" class="q-py-md">
            <q-item-section avatar top>
              <span class="numero-estacion">{{ i + 1 }}</span>
            </q-item-section>
            <q-item-section>
              <q-item-label class="text-weight-medium">{{ t(p.titulo) }}</q-item-label>
              <q-item-label caption class="q-mt-xs" style="line-height: 1.5">{{ t(p.texto) }}</q-item-label>
            </q-item-section>
            <q-item-section side>
              <q-btn outline color="primary" no-caps :label="t('ayuda.ir', { pagina: t(p.menu) })" class="gt-xs" @click="$emit('ir', p.pagina)" />
              <q-btn flat round color="primary" icon="arrow_forward" class="lt-sm" :aria-label="t('ayuda.ir', { pagina: t(p.menu) })" @click="$emit('ir', p.pagina)" />
            </q-item-section>
          </q-item>
        </q-list>
      </q-card>
    </template>

    <q-card v-if="guiasVistas.length" id="ayuda-guias">
      <q-card-section>
        <div class="titulo-seccion">{{ t('ayuda.secGuias') }}</div>
        <div class="texto-suave text-body2 q-mt-xs">{{ t('ayuda.guiasTexto') }}</div>
      </q-card-section>
      <q-list separator>
        <q-expansion-item v-for="g in guiasVistas" :key="g.titulo + (texto ? '?' : '')" :icon="g.icono" :label="t(g.titulo)" header-class="text-weight-medium"
          :default-opened="!!texto" expand-icon-class="texto-suave">
          <div class="q-px-md q-pb-md">
            <ol class="pasos-guia">
              <li v-for="(paso, i) in t(g.pasos).split('\n')" :key="i"><span class="numero-estacion">{{ i + 1 }}</span><span>{{ paso }}</span></li>
            </ol>
            <q-btn outline color="primary" no-caps icon-right="arrow_forward" :label="t('ayuda.ir', { pagina: t(g.menu) })" @click="$emit('ir', g.pagina)" />
          </div>
        </q-expansion-item>
      </q-list>
    </q-card>

    <q-card v-if="conceptosVistos.length" id="ayuda-conceptos">
      <q-card-section class="titulo-seccion">{{ t('ayuda.secConceptos') }}</q-card-section>
      <q-card-section class="row q-col-gutter-md q-pt-none">
        <div v-for="c in conceptosVistos" :key="c.termino" class="col-12 col-sm-6 col-lg-4">
          <div class="concepto full-height">
            <div class="row items-center no-wrap text-weight-medium"><q-icon :name="c.icono" color="primary" size="20px" class="q-mr-sm" />{{ t(c.termino) }}</div>
            <div class="text-body2 texto-suave q-mt-xs">{{ t(c.definicion) }}</div>
          </div>
        </div>
      </q-card-section>
    </q-card>

    <q-card v-if="problemasVistos.length" id="ayuda-problemas">
      <q-card-section>
        <div class="titulo-seccion">{{ t('ayuda.secProblemas') }}</div>
        <div class="texto-suave text-body2 q-mt-xs">{{ t('ayuda.problemasTexto') }}</div>
      </q-card-section>
      <q-list separator>
        <q-item v-for="p in problemasVistos" :key="p.codigo" class="q-py-sm">
          <q-item-section avatar top><q-icon name="o_report" color="negative" /></q-item-section>
          <q-item-section>
            <q-item-label class="text-weight-medium">{{ p.titulo }}<span v-if="p.codigo !== 'sin_conexion'" class="codigo texto-suave text-caption q-ml-sm">{{ p.codigo }}</span></q-item-label>
            <q-item-label caption style="line-height: 1.5">{{ t(p.solucion) }}</q-item-label>
          </q-item-section>
        </q-item>
      </q-list>
    </q-card>

    <q-card v-if="categoriasVistas.length" id="ayuda-preguntas">
      <q-card-section class="titulo-seccion">{{ t('ayuda.preguntas') }}</q-card-section>
      <template v-for="c in categoriasVistas" :key="c.titulo">
        <div class="q-px-md q-pt-sm text-caption text-weight-medium texto-suave text-uppercase">{{ t(c.titulo) }}</div>
        <q-list separator>
          <q-expansion-item v-for="f in c.preguntas" :key="f.pregunta + (texto ? '?' : '')" :label="t(f.pregunta)" :default-opened="!!texto" header-class="text-weight-medium" expand-icon-class="texto-suave">
            <div class="q-px-md q-pb-md texto-suave" style="line-height: 1.6">{{ t(f.respuesta) }}</div>
          </q-expansion-item>
        </q-list>
      </template>
    </q-card>

    <q-card v-if="texto && !hayResultados">
      <q-card-section class="texto-suave">{{ t('ayuda.sinResultados', { texto }) }}</q-card-section>
    </q-card>

    <q-card>
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
// Ayuda explica qué hace Tramtai (el recorrido de una petición), cómo empezar, guías por tarea, los conceptos, qué
// hacer ante cada error y las preguntas frecuentes; el buscador filtra todo menos el recorrido y los primeros pasos.
import { computed, ref } from 'vue'
import { copiar, tituloError } from '../api/cliente'
import { ejemplos } from '../ejemplos'
import { guiaOculta, ocultarGuia } from '../estado'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import TablaApis from '../componentes/TablaApis.vue'
import { t } from '../i18n'

defineEmits<{ ir: [pagina: string] }>()

const busqueda = ref<string | null>('')
const texto = computed(() => (busqueda.value ?? '').trim())
const coincide = (...textos: string[]) => !texto.value || textos.some((x) => x.toLowerCase().includes(texto.value.toLowerCase()))

const secciones = [
  { id: 'funciona', titulo: 'ayuda.secFunciona', icono: 'o_route' },
  { id: 'archivos', titulo: 'ayuda.secArchivos', icono: 'o_description' },
  { id: 'apis', titulo: 'ayuda.secApis', icono: 'o_compare_arrows' },
  { id: 'empezar', titulo: 'ayuda.empezar', icono: 'o_flag' },
  { id: 'guias', titulo: 'ayuda.secGuias', icono: 'o_checklist' },
  { id: 'conceptos', titulo: 'ayuda.secConceptos', icono: 'o_lightbulb' },
  { id: 'problemas', titulo: 'ayuda.secProblemas', icono: 'o_build' },
  { id: 'preguntas', titulo: 'ayuda.preguntas', icono: 'o_quiz' },
] as const
const irA = (id: string) => document.getElementById('ayuda-' + id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })

const recorrido = [
  { titulo: 'ayuda.f1', texto: 'ayuda.f1Texto', icono: 'o_apps' },
  { titulo: 'ayuda.f2', texto: 'ayuda.f2Texto', icono: 'o_vpn_key' },
  { titulo: 'ayuda.f3', texto: 'ayuda.f3Texto', icono: 'o_hourglass_top' },
  { titulo: 'ayuda.f4', texto: 'ayuda.f4Texto', icono: 'o_smart_toy' },
  { titulo: 'ayuda.f5', texto: 'ayuda.f5Texto', icono: 'o_task_alt' },
] as const

// recorridoArchivo es el camino de un archivo, de la app al agente, igual por las tres APIs.
const recorridoArchivo = [
  { titulo: 'ayuda.a1', texto: 'ayuda.a1Texto', icono: 'o_upload_file' },
  { titulo: 'ayuda.a2', texto: 'ayuda.a2Texto', icono: 'o_inventory_2' },
  { titulo: 'ayuda.a3', texto: 'ayuda.a3Texto', icono: 'o_transform' },
  { titulo: 'ayuda.a4', texto: 'ayuda.a4Texto', icono: 'o_smart_toy' },
  { titulo: 'ayuda.a5', texto: 'ayuda.a5Texto', icono: 'o_task_alt' },
] as const
const notasArchivo = ['ayuda.archivosNota0', 'ayuda.archivosNota1', 'ayuda.archivosNota2', 'ayuda.archivosNota3'] as const
// ejemplosArchivo envían un PDF por cada API, con la dirección predeterminada y la clave por poner.
const ejemplosArchivo = computed(() => ejemplos({ url: 'http://127.0.0.1:17373', modelo: 'perfil/rapido', contratos: ['nativa', 'openai', 'anthropic'],
  prueba: { archivos: ['informe.pdf'], solicitud: { mensajes: [{ rol: 'usuario', contenido: t('ejemplos.mensaje') }] } } }))
const ejemploArchivo = ref('python')
const codigoArchivo = computed(() => (ejemplosArchivo.value.find((e) => e.id === ejemploArchivo.value) ?? ejemplosArchivo.value[0]).codigo(t('codigo.tuClave')))

const pasos = [
  { titulo: 'ayuda.paso1', texto: 'ayuda.paso1Texto', pagina: 'agentes', menu: 'menu.agentes' },
  { titulo: 'ayuda.paso2', texto: 'ayuda.paso2Texto', pagina: 'ejecutar', menu: 'menu.ejecutar' },
  { titulo: 'ayuda.paso3', texto: 'ayuda.paso3Texto', pagina: 'conectar', menu: 'menu.conectar' },
  { titulo: 'ayuda.paso4', texto: 'ayuda.paso4Texto', pagina: 'conectar', menu: 'menu.conectar' },
  { titulo: 'ayuda.paso5', texto: 'ayuda.paso5Texto', pagina: 'configuracion', menu: 'menu.configuracion' },
] as const

const guias = [
  { titulo: 'ayuda.g1', pasos: 'ayuda.g1Pasos', icono: 'o_link', pagina: 'conectar', menu: 'menu.conectar' },
  { titulo: 'ayuda.g2', pasos: 'ayuda.g2Pasos', icono: 'o_description', pagina: 'conectar', menu: 'menu.conectar' },
  { titulo: 'ayuda.g3', pasos: 'ayuda.g3Pasos', icono: 'o_data_object', pagina: 'ejecutar', menu: 'menu.ejecutar' },
  { titulo: 'ayuda.g4', pasos: 'ayuda.g4Pasos', icono: 'o_alt_route', pagina: 'ejecutar', menu: 'menu.ejecutar' },
  { titulo: 'ayuda.g5', pasos: 'ayuda.g5Pasos', icono: 'o_savings', pagina: 'conectar', menu: 'menu.conectar' },
  { titulo: 'ayuda.g6', pasos: 'ayuda.g6Pasos', icono: 'o_troubleshoot', pagina: 'historial', menu: 'menu.historial' },
] as const
const guiasVistas = computed(() => guias.filter((g) => coincide(t(g.titulo), t(g.pasos))))

const conceptos = [
  { termino: 'ayuda.cAgente', definicion: 'ayuda.cAgenteTexto', icono: 'o_smart_toy' },
  { termino: 'ayuda.cModelo', definicion: 'ayuda.cModeloTexto', icono: 'o_memory' },
  { termino: 'ayuda.cPerfil', definicion: 'ayuda.cPerfilTexto', icono: 'o_tune' },
  { termino: 'ayuda.cEsfuerzo', definicion: 'ayuda.cEsfuerzoTexto', icono: 'o_psychology' },
  { termino: 'ayuda.cReceta', definicion: 'ayuda.cRecetaTexto', icono: 'o_receipt_long' },
  { termino: 'ayuda.cRespaldo', definicion: 'ayuda.cRespaldoTexto', icono: 'o_alt_route' },
  { termino: 'ayuda.cApis', definicion: 'ayuda.cApisTexto', icono: 'o_hub' },
  { termino: 'ayuda.cPeticion', definicion: 'ayuda.cPeticionTexto', icono: 'o_history' },
  { termino: 'ayuda.cEsquema', definicion: 'ayuda.cEsquemaTexto', icono: 'o_data_object' },
  { termino: 'ayuda.cContexto', definicion: 'ayuda.cContextoTexto', icono: 'o_straighten' },
  { termino: 'ayuda.cEspacio', definicion: 'ayuda.cEspacioTexto', icono: 'o_folder' },
  { termino: 'ayuda.cPresupuesto', definicion: 'ayuda.cPresupuestoTexto', icono: 'o_savings' },
] as const
const conceptosVistos = computed(() => conceptos.filter((c) => coincide(t(c.termino), t(c.definicion))))

// problemas son los errores más comunes con el mismo título que muestra la app y qué hacer.
const problemas = computed(() => [
  { codigo: 'sin_conexion', titulo: t('app.sinConexion'), solucion: 'ayuda.sSinConexion' },
  { codigo: 'no_autorizado', titulo: tituloError('no_autorizado'), solucion: 'ayuda.sNoAutorizado' },
  { codigo: 'autenticacion_agente', titulo: tituloError('autenticacion_agente'), solucion: 'ayuda.sAutenticacion' },
  { codigo: 'agente_no_disponible', titulo: tituloError('agente_no_disponible'), solucion: 'ayuda.sNoDisponible' },
  { codigo: 'limite_uso_proveedor', titulo: tituloError('limite_uso_proveedor'), solucion: 'ayuda.sLimite' },
  { codigo: 'modelo_no_disponible', titulo: tituloError('modelo_no_disponible'), solucion: 'ayuda.sModelo' },
  { codigo: 'entrada_demasiado_grande', titulo: tituloError('entrada_demasiado_grande'), solucion: 'ayuda.sGrande' },
  { codigo: 'presupuesto_agotado', titulo: t('ayuda.presupuestoAgotado'), solucion: 'ayuda.sPresupuesto' },
  { codigo: 'salida_no_cumple_esquema', titulo: tituloError('salida_no_cumple_esquema'), solucion: 'ayuda.sEsquema' },
  { codigo: 'tiempo_ejecucion_agotado', titulo: tituloError('tiempo_ejecucion_agotado'), solucion: 'ayuda.sTiempo' },
  { codigo: 'cola_llena', titulo: tituloError('cola_llena'), solucion: 'ayuda.sCola' },
  { codigo: 'conversion_fallida', titulo: tituloError('conversion_fallida'), solucion: 'ayuda.sConversion' },
  { codigo: 'conversor_no_disponible', titulo: tituloError('conversor_no_disponible'), solucion: 'ayuda.sConversor' },
] as const)
const problemasVistos = computed(() => problemas.value.filter((p) => coincide(p.titulo, p.codigo, t(p.solucion))))

const categorias = [
  { titulo: 'ayuda.catEmpezar', preguntas: [['ayuda.p22', 'ayuda.r22'], ['ayuda.p23', 'ayuda.r23'], ['ayuda.p20', 'ayuda.r20'], ['ayuda.p5', 'ayuda.r5'], ['ayuda.p1', 'ayuda.r1'], ['ayuda.p6', 'ayuda.r6'], ['ayuda.p9', 'ayuda.r9']] },
  { titulo: 'ayuda.catApis', preguntas: [['ayuda.p11', 'ayuda.r11'], ['ayuda.p25', 'ayuda.r25'], ['ayuda.p21', 'ayuda.r21'], ['ayuda.p4', 'ayuda.r4'], ['ayuda.p12', 'ayuda.r12'],
    ['ayuda.p14', 'ayuda.r14'], ['ayuda.p15', 'ayuda.r15'], ['ayuda.p18', 'ayuda.r18'], ['ayuda.p19', 'ayuda.r19']] },
  { titulo: 'ayuda.catDatos', preguntas: [['ayuda.p2', 'ayuda.r2'], ['ayuda.p3', 'ayuda.r3'], ['ayuda.p13', 'ayuda.r13'], ['ayuda.p7', 'ayuda.r7']] },
  { titulo: 'ayuda.catApp', preguntas: [['ayuda.p8', 'ayuda.r8'], ['ayuda.p16', 'ayuda.r16'], ['ayuda.p17', 'ayuda.r17'], ['ayuda.p24', 'ayuda.r24'], ['ayuda.p10', 'ayuda.r10']] },
] as const
const categoriasVistas = computed(() => categorias
  .map((c) => ({ titulo: c.titulo, preguntas: c.preguntas.filter(([p, r]) => coincide(t(p), t(r))).map(([pregunta, respuesta]) => ({ pregunta, respuesta })) }))
  .filter((c) => c.preguntas.length))

const hayResultados = computed(() => guiasVistas.value.length + conceptosVistos.value.length + problemasVistos.value.length + categoriasVistas.value.length > 0)
</script>

<style scoped>
/* El recorrido de una petición, como las paradas de una línea: en fila en pantallas anchas y en columna en el móvil. */
.recorrido {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
}
.recorrido__parada {
  position: relative;
  border: 1px solid var(--borde);
  border-radius: var(--radio-control);
  padding: 12px;
}
.recorrido__parada:not(:last-child)::after {
  content: '→';
  position: absolute;
  right: -11px;
  top: 14px;
  color: var(--acento);
  font-weight: 700;
}
@media (max-width: 899px) {
  .recorrido {
    grid-template-columns: minmax(0, 1fr);
  }
  .recorrido__parada:not(:last-child)::after {
    content: '↓';
    right: auto;
    left: 18px;
    top: auto;
    bottom: -15px;
  }
}
.pasos-guia {
  list-style: none;
  margin: 0 0 12px;
  padding: 0;
  display: grid;
  gap: 10px;
}
.pasos-guia li {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  line-height: 1.5;
}
.pasos-guia .numero-estacion {
  flex: none;
  width: 24px;
  height: 24px;
}
.concepto {
  border: 1px solid var(--borde);
  border-radius: var(--radio-control);
  padding: 12px;
}
</style>
