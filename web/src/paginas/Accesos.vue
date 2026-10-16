<template>
  <div class="pagina apilado">
    <CabeceraPagina :titulo="t('menu.conectar')" :descripcion="t('conectar.subtitulo')" />

    <q-card>
      <q-card-section class="seccion">
        <div>
          <div class="titulo-seccion">{{ t('conectar.directa') }}</div>
          <div class="texto-suave text-body2">{{ t('conectar.directaTexto') }}</div>
        </div>
      </q-card-section>
      <div class="q-px-md text-weight-medium">{{ t('conectar.conexion') }}</div>
      <q-card-section class="row q-col-gutter-md">
        <div v-for="c in campos" :key="c.titulo" class="col-12 col-md-6">
          <div class="text-caption texto-suave q-mb-xs">{{ c.titulo }}</div>
          <div class="bloque-codigo">{{ c.valor }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('comun.copiar') + ' ' + c.titulo" @click="copiar(c.valor)" /></div>
          <div class="text-caption texto-suave q-mt-xs">{{ c.texto }}</div>
        </div>
        <div class="col-12 col-md-6">
          <div class="text-caption texto-suave q-mb-xs">{{ t('conectar.clave') }}</div>
          <div class="bloque-codigo" style="padding-right: 112px">{{ clave ? (verClave ? clave : ocultar(clave)) : t('comun.cargando') }}
            <div class="boton-copiar row no-wrap">
              <q-btn flat dense round size="sm" :icon="verClave ? 'visibility_off' : 'visibility'" :aria-label="verClave ? t('conectar.ocultarClave') : t('conectar.mostrarClave')" @click="verClave = !verClave" />
              <q-btn flat dense round size="sm" icon="content_copy" :aria-label="t('conectar.copiarClave')" :disable="!clave" @click="copiar(clave, t('conectar.claveCopiada'))" />
              <q-btn flat dense round size="sm" icon="o_autorenew" :aria-label="t('conectar.regenerarClave')" :disable="!clave" @click="regenerarClaveInstancia">
                <q-tooltip>{{ t('conectar.regenerarClave') }}</q-tooltip>
              </q-btn>
            </div>
          </div>
        </div>
      </q-card-section>
      <q-card-section class="q-pt-none">
        <div class="text-caption texto-suave q-mb-xs">{{ t('conectar.modeloAntes') }} <span class="codigo">model</span> {{ t('conectar.modeloDespues') }}</div>
        <div v-for="g in gruposModelo" :key="g.titulo" class="row items-center q-gutter-xs q-mb-xs">
          <span class="grupo-modelo">
            <span v-if="g.agente" class="linea__chapa" :class="'linea--' + g.agente" aria-hidden="true">{{ codigoLinea(g.agente) }}</span>{{ g.titulo }}
          </span>
          <q-chip v-for="m in g.ids" :key="m" dense clickable outline color="primary" class="codigo" @click="copiar(m)">{{ m }}</q-chip>
        </div>
      </q-card-section>
      <q-separator />
      <div class="q-px-md q-pt-md text-weight-medium">{{ t('conectar.ejemplos') }}</div>
      <div class="q-px-md q-pt-sm">
        <q-btn-toggle v-model="lenguaje" no-caps flat dense toggle-color="primary" class="segmentado inline-flex"
          :options="ejemplos.map((e) => ({ label: e.titulo(), value: e.id }))" />
      </div>
      <q-card-section>
        <div class="bloque-codigo sin-ajuste">{{ ejemplo.codigo(verClave ? clave : ocultar(clave)) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('conectar.copiarEjemplo')" @click="copiar(ejemplo.codigo(clave), t('conectar.ejemploCopiado'))" /></div>
      </q-card-section>
    </q-card>

    <q-card>
      <q-card-section class="row items-center q-col-gutter-md">
        <div class="col-12 col-sm">
          <div>
            <div class="titulo-seccion">{{ t('puntos.titulo') }}</div>
            <div class="texto-suave text-body2">{{ t('puntos.texto') }}</div>
          </div>
        </div>
        <div class="col-12 col-sm-auto">
          <q-btn unelevated color="primary" icon="add" :label="t('puntos.nuevo')" no-caps @click="abrirNuevo" />
        </div>
      </q-card-section>
      <template v-if="presupuestoEspacio">
        <q-separator />
        <q-card-section>
          <div class="text-weight-medium">{{ t('presupuesto.espacio') }}</div>
          <div class="text-caption texto-suave">{{ t('presupuesto.espacioTexto') }}</div>
          <BarrasPresupuesto :estado="presupuestoEspacio" @reiniciar="reiniciarPresupuesto()" />
        </q-card-section>
      </template>
      <q-separator />
      <template v-if="puntos.length">
        <div v-for="g in grupos" :key="g.nombre">
          <div v-if="hayGrupos" class="row items-center q-px-md q-pt-md q-pb-xs cabecera-grupo">
            <q-icon :name="g.nombre ? 'o_folder' : 'o_folder_off'" class="q-mr-sm texto-suave" />
            <div class="text-weight-medium">{{ g.nombre || t('puntos.sinGrupo') }}</div>
            <div class="text-caption texto-suave q-ml-sm">{{ t('puntos.activos', { activos: g.activos, total: g.puntos.length }) }}</div>
            <q-space />
            <q-btn flat dense no-caps size="sm" color="primary" icon="o_play_arrow" :label="t('puntos.activarTodos')"
              :disable="g.activos === g.puntos.length" :loading="ocupado === g.nombre + ':1'" @click="activarGrupo(g.nombre, true)" />
            <q-btn flat dense no-caps size="sm" color="grey-8" icon="o_stop" :label="t('puntos.desactivarTodos')"
              :disable="g.activos === 0" :loading="ocupado === g.nombre + ':0'" @click="activarGrupo(g.nombre, false)" />
          </div>
          <div>
            <div v-for="p in g.puntos" :key="p.nombre" class="fila-api">
              <span class="linea__chapa linea__chapa--grande" :class="['linea--' + (p.agente ?? 'otro'), { 'linea__chapa--apagada': !activo(p) }]" aria-hidden="true">{{ p.agente ? codigoLinea(p.agente) : 'API' }}</span>
              <div class="fila-api__info">
                <div class="row items-center q-gutter-x-sm">
                  <span class="text-weight-medium">{{ p.nombre }}</span>
                  <span class="etiqueta" :class="!activo(p) ? 'etiqueta--neutro' : p.error ? 'etiqueta--error' : 'etiqueta--ok'">{{ activo(p) ? t('puntos.activo') : t('puntos.desactivado') }}</span>
                  <span v-if="p.esquema" class="etiqueta etiqueta--info">{{ t('puntos.json') }}</span>
                  <span v-if="p.respaldo?.length" class="etiqueta etiqueta--neutro">{{ t('puntos.respaldoDe', { agentes: p.respaldo.map(etiquetaDe).join(', ') }) }}</span>
                </div>
                <div class="row items-center no-wrap codigo texto-suave">
                  <span class="ellipsis">{{ p.url || t('puntos.sinDireccion') }}</span>
                  <q-btn v-if="p.url" flat dense round size="sm" icon="content_copy" :aria-label="t('puntos.copiarUrl')" @click="copiar(p.url)"><q-tooltip>{{ t('puntos.copiarUrl') }}</q-tooltip></q-btn>
                </div>
                <div class="text-caption texto-suave">
                  <template v-if="idModelo(p)">{{ etiquetaDe(idModelo(p)!) }} · </template>{{ p.contratos.map(nombreContrato).join(' · ') }}
                  <template v-if="p.esfuerzo"> · {{ t('esfuerzo.titulo') }} {{ nombreEsfuerzo(p.esfuerzo).toLowerCase() }}</template>
                </div>
                <div v-if="p.instrucciones" class="text-caption texto-suave row no-wrap items-start">
                  <q-icon name="o_notes" class="q-mr-xs q-mt-xs" /><span class="recortar-2">{{ p.instrucciones }}</span>
                </div>
                <div v-if="p.error && p.error !== 'desactivado'" class="text-caption text-negative">{{ p.error }}</div>
              </div>
              <div class="fila-api__uso text-caption">
                <a v-if="uso[p.nombre]" href="#historial" class="enlace" @click.prevent="verPeticiones(p.nombre)">{{ textoUso(p.nombre) }}</a>
                <span v-else class="texto-suave">{{ t('puntos.sinUso') }}</span>
                <BarrasPresupuesto v-if="presupuestos[p.nombre]" :estado="presupuestos[p.nombre]" sin-boton />
              </div>
              <div class="fila-api__acciones row items-center no-wrap q-gutter-x-xs">
                <q-btn outline dense no-caps color="primary" icon="o_tune" class="q-px-sm" :label="t('puntos.ajustes')" @click="abrirAjustes(p)" />
                <MenuAcciones :acciones="accionesDe(p)" @elegir="(i: number) => ejecutarAccion(p, i)" />
              </div>
            </div>
          </div>
        </div>
      </template>
      <EstadoVacio v-else :titulo="t('puntos.vacio')" :accion="t('puntos.nuevo')" @accion="abrirNuevo" />
    </q-card>

    <q-dialog v-model="nuevo">
      <q-card style="width: 600px; max-width: 95vw">
        <q-card-section>
          <div class="titulo-seccion">{{ t('puntos.nuevo') }}</div>
          <div class="texto-suave text-body2">{{ t('puntos.nuevoTexto') }}</div>
        </q-card-section>
        <q-form @submit="crear">
          <q-card-section class="q-gutter-y-md q-pt-none">
            <q-input v-model="formulario.nombre" outlined dense :label="t('puntos.nombre')" autofocus :hint="t('puntos.nombreHint')"
              :rules="[(v: string) => /^[a-z0-9][a-z0-9_-]{0,39}$/.test(v) || t('puntos.nombreRegla')]" />
            <q-select :model-value="formulario.grupo" :options="sugerencias" outlined dense use-input hide-selected fill-input input-debounce="0"
              :label="t('puntos.grupo')" :hint="t('puntos.grupoHint')" maxlength="40" hide-dropdown-icon
              @input-value="(v: string) => (formulario.grupo = v)" @filter="filtrarGrupos" @update:model-value="(v: string) => (formulario.grupo = v)" />
            <EditorReceta v-model="formulario.receta" :catalogo="catalogo" para-api />
            <q-expansion-item dense icon="o_tune" :label="t('comun.opcionesAvanzadas')" header-class="q-px-none text-body2">
              <div class="q-gutter-y-md q-pt-sm">
                <div class="text-caption texto-suave">{{ t('puntos.avanzadasTexto') }}</div>
                <div>
                  <div class="text-caption texto-suave">{{ t('puntos.direccion') }}</div>
                  <q-option-group v-model="formulario.direccion" inline
                    :options="[{ label: t('puntos.ruta'), value: 'ruta' }, { label: t('puntos.puertoPropio'), value: 'puerto' }]" />
                  <q-input v-if="formulario.direccion === 'puerto'" v-model="formulario.puerto" outlined dense inputmode="numeric" :label="t('puntos.puerto')"
                    class="q-mt-sm" style="max-width: 200px"
                    :rules="[(v: string) => (/^\d+$/.test(v) && +v >= 1024 && +v <= 65535) || t('puntos.puertoRegla')]" />
                </div>
                <div>
                  <div class="text-caption texto-suave">{{ t('puntos.compatibilidad') }}</div>
                  <q-option-group v-model="formulario.contratos" type="checkbox" inline
                    :options="contratos.map((value) => ({ label: nombreContrato(value), value }))" />
                </div>
                <q-select v-model="formulario.permitidos" :options="opciones" emit-value map-options outlined dense multiple use-chips
                  use-input new-value-mode="add-unique" :label="t('puntos.permitidos')" :hint="t('puntos.permitidosHint')" />
                <q-input v-model="formulario.alias" type="textarea" autogrow outlined dense :label="t('puntos.alias')" input-class="codigo"
                  :hint="t('puntos.aliasHint')" />
                <EditorPresupuesto v-model="formulario.presupuesto" />
              </div>
            </q-expansion-item>
          </q-card-section>
          <q-card-actions align="right" class="q-pa-md">
            <q-btn v-close-popup flat no-caps :label="t('comun.cancelar')" />
            <q-btn unelevated color="primary" no-caps type="submit" :label="t('comun.crear')" :loading="guardando" />
          </q-card-actions>
        </q-form>
      </q-card>
    </q-dialog>

    <q-dialog v-model="dialogoInstrucciones">
      <q-card style="width: 560px; max-width: 95vw">
        <q-card-section>
          <div class="titulo-seccion">{{ t('puntos.ajustesDe', { nombre: editando }) }}</div>
        </q-card-section>
        <q-card-section class="q-pt-none q-gutter-y-md">
          <EditorReceta v-model="ajustes.receta" :catalogo="catalogo" para-api />
          <q-select v-model="ajustes.permitidos" :options="opciones" emit-value map-options outlined dense multiple use-chips
            use-input new-value-mode="add-unique" :label="t('puntos.permitidos')" :hint="t('puntos.permitidosHint')" />
          <EditorPresupuesto v-model="ajustes.presupuesto" />
        </q-card-section>
        <q-card-actions align="right" class="q-pa-md">
          <q-btn v-close-popup flat no-caps :label="t('comun.cancelar')" />
          <q-btn unelevated color="primary" no-caps :label="t('comun.guardar')" :loading="guardando" @click="guardarAjustes" />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <!-- Al cerrarse se olvida la clave mostrada: no queda en memoria de la página. -->
    <q-dialog v-model="dialogoClave" @hide="elegido = undefined">
      <q-card style="width: 560px; max-width: 95vw">
        <q-card-section>
          <div class="titulo-seccion">{{ recienCreado ? t('puntos.listo') : t('puntos.claveNuevaDe', { nombre: elegido?.nombre ?? '' }) }}</div>
          <div class="texto-suave text-body2">{{ t('puntos.configura') }}</div>
          <q-banner rounded dense class="aviso-caja q-mt-sm">
            <template #avatar><q-icon name="o_lock" color="warning" /></template>{{ t('puntos.claveUnaVez') }}
          </q-banner>
        </q-card-section>
        <q-card-section class="q-gutter-y-md q-pt-none">
          <div v-for="c in camposPunto" :key="c.titulo">
            <div class="text-caption texto-suave q-mb-xs">{{ c.titulo }}</div>
            <div class="bloque-codigo">{{ c.valor }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('comun.copiar') + ' ' + c.titulo" @click="copiar(c.valor)" /></div>
          </div>
        </q-card-section>
        <q-card-actions align="right" class="q-pa-md"><q-btn v-close-popup unelevated color="primary" no-caps :label="t('comun.hecho')" /></q-card-actions>
      </q-card>
    </q-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useQuasar } from 'quasar'
import BarrasPresupuesto from '../componentes/BarrasPresupuesto.vue'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import EstadoVacio from '../componentes/EstadoVacio.vue'
import MenuAcciones, { type Accion } from '../componentes/MenuAcciones.vue'
import EditorPresupuesto from '../componentes/EditorPresupuesto.vue'
import EditorReceta from '../componentes/EditorReceta.vue'
import { ajustesDeReceta, api, avisar, avisarError, codigoLinea, nombreAgente, ordenAgentes, configDeReceta, copiar, etiquetaModelo, haceCuanto, idModeloDe as idModelo, nombreEsfuerzo, opcionModelo,
  origenHistorial, presupuestoLimpio, recetaDePunto, recetaInicial, recetaVacia, type ConfigPunto, type EstadoPresupuesto, type EstadoPunto, type Modelo,
  type Presupuesto, type UsoPunto } from '../api'
import { t } from '../i18n'

const $q = useQuasar()
const base = ref(location.origin)
const clave = ref('')
const verClave = ref(false)
const catalogo = ref<Modelo[]>([])
const ids = computed(() => catalogo.value.map((m) => m.id))
const puntos = ref<EstadoPunto[]>([])
const nuevo = ref(false)
const guardando = ref(false)
const ocupado = ref('')
const dialogoClave = ref(false)
const recienCreado = ref(false)
const elegido = ref<{ nombre: string; url: string; clave: string }>()
const lenguaje = ref('python')
const contratos = ['openai', 'anthropic', 'nativa']
const nombreContrato = (c: string) => ({ openai: 'OpenAI', anthropic: 'Anthropic', nativa: t('contrato.nativa') })[c] ?? c
const activo = (p: EstadoPunto) => p.error !== 'desactivado'

// grupos ordena los puntos por grupo (los que no tienen, al final).
const grupos = computed(() => {
  const mapa = new Map<string, EstadoPunto[]>()
  for (const p of puntos.value) mapa.set(p.grupo ?? '', [...(mapa.get(p.grupo ?? '') ?? []), p])
  return [...mapa.entries()]
    .sort(([a], [b]) => (!a ? 1 : !b ? -1 : a.localeCompare(b)))
    .map(([nombre, lista]) => ({ nombre, puntos: lista, activos: lista.filter(activo).length }))
})
const hayGrupos = computed(() => grupos.value.some((g) => g.nombre))
const sugerencias = ref<string[]>([])
function filtrarGrupos(texto: string, actualizar: (f: () => void) => void) {
  actualizar(() => {
    const todos = grupos.value.map((g) => g.nombre).filter(Boolean)
    sugerencias.value = todos.filter((g) => g.toLowerCase().includes(texto.toLowerCase()))
  })
}

const campos = computed(() => [
  { titulo: t('conectar.urlOpenai'), valor: base.value + '/openai/v1', texto: t('conectar.urlOpenaiTexto') },
  { titulo: t('conectar.urlAnthropic'), valor: base.value + '/anthropic', texto: t('conectar.urlAnthropicTexto') },
  { titulo: t('conectar.apiPropia'), valor: base.value + '/api/v1', texto: t('conectar.apiPropiaTexto') },
])
// gruposModelo agrupa los valores de model: primero los perfiles y luego cada agente en el orden fijo de la red.
const gruposModelo = computed(() => [
  { agente: '', titulo: t('conectar.perfiles'), ids: ids.value.filter((id) => id.startsWith('perfil/')) },
  ...ordenAgentes.map((a) => ({ agente: a, titulo: nombreAgente(a), ids: ids.value.filter((id) => id.startsWith(a + '/')) })),
].filter((g) => g.ids.length))

// accionesDe son las acciones del menú «⋯» de una API; ejecutarAccion las corre en el mismo orden.
function accionesDe(p: EstadoPunto): Accion[] {
  return [
    { etiqueta: activo(p) ? t('puntos.accionDesactivar') : t('puntos.accionActivar'), icono: activo(p) ? 'o_pause' : 'o_play_arrow' },
    { etiqueta: t('puntos.abrirProbar'), icono: 'o_science' },
    { etiqueta: t('puntos.regenerarClave'), icono: 'o_key' },
    { etiqueta: t('presupuesto.reiniciar'), icono: 'o_restart_alt', oculto: !presupuestos.value[p.nombre] },
    { etiqueta: t('comun.eliminar'), icono: 'o_delete', peligro: true },
  ]
}
function ejecutarAccion(p: EstadoPunto, i: number) {
  const acciones = [() => activar(p.nombre, !activo(p)), () => abrirEnProbar(p.nombre), () => regenerarClave(p), () => reiniciarPresupuesto(p.nombre), () => eliminar(p.nombre)]
  acciones[i]?.()
}
const modeloEjemplo = computed(() => ids.value.find((id) => id === 'perfil/rapido') ?? ids.value.find((id) => id.includes('/')) ?? 'claude')

const ocultar = (c: string) => (c ? c.slice(0, 6) + '•'.repeat(18) : '')
const ejemplos = [
  { id: 'python', titulo: () => 'Python · OpenAI', codigo: (k: string) => `from openai import OpenAI

cliente = OpenAI(base_url="${base.value}/openai/v1", api_key="${k}")
r = cliente.chat.completions.create(
    model="${modeloEjemplo.value}",
    messages=[{"role": "user", "content": "${t('conectar.hola')}"}],
)
print(r.choices[0].message.content)` },
  { id: 'node', titulo: () => 'Node · OpenAI', codigo: (k: string) => `import OpenAI from 'openai'

const cliente = new OpenAI({ baseURL: '${base.value}/openai/v1', apiKey: '${k}' })
const r = await cliente.chat.completions.create({
  model: '${modeloEjemplo.value}',
  messages: [{ role: 'user', content: '${t('conectar.hola')}' }],
})
console.log(r.choices[0].message.content)` },
  { id: 'anthropic', titulo: () => 'Python · Anthropic', codigo: (k: string) => `import anthropic

cliente = anthropic.Anthropic(base_url="${base.value}/anthropic", api_key="${k}")
m = cliente.messages.create(
    model="${modeloEjemplo.value}", max_tokens=1024,
    messages=[{"role": "user", "content": "${t('conectar.hola')}"}],
)
print(m.content[0].text)` },
  { id: 'curl', titulo: () => 'curl', codigo: (k: string) => `curl ${base.value}/openai/v1/chat/completions \\
  -H "Authorization: Bearer ${k}" \\
  -H "Content-Type: application/json" \\
  -d '{"model": "${modeloEjemplo.value}", "messages": [{"role": "user", "content": "${t('conectar.hola')}"}]}'` },
  { id: 'entorno', titulo: () => t('conectar.entorno'), codigo: (k: string) => `${t('conectar.entornoComentario')}
OPENAI_BASE_URL=${base.value}/openai/v1
OPENAI_API_KEY=${k}
ANTHROPIC_BASE_URL=${base.value}/anthropic
ANTHROPIC_API_KEY=${k}` },
]
const ejemplo = computed(() => ejemplos.find((e) => e.id === lenguaje.value) ?? ejemplos[0])

const opciones = computed(() => catalogo.value.map(opcionModelo))
const etiquetaDe = (id: string) => opciones.value.find((o) => o.value === id)?.label ?? etiquetaModelo(id)
const vacio = () => ({ nombre: '', grupo: '', direccion: 'ruta', puerto: '', contratos: [...contratos], permitidos: [] as string[], alias: '',
  receta: recetaVacia(), presupuesto: {} as Presupuesto })
const formulario = reactive(vacio())

const camposPunto = computed(() => {
  const p = elegido.value
  if (!p) return []
  return [
    { titulo: t('conectar.urlOpenai'), valor: p.url + '/openai/v1' },
    { titulo: t('conectar.urlAnthropic'), valor: p.url + '/anthropic' },
    { titulo: t('conectar.clave'), valor: p.clave },
  ]
})

// uso son las peticiones de cada API que conserva la retención, contadas por el servicio.
const uso = ref<Record<string, UsoPunto>>({})
// presupuestos es el consumo frente a los límites de cada API con presupuesto; presupuestoEspacio, el del espacio.
const presupuestos = ref<Record<string, EstadoPresupuesto>>({})
const presupuestoEspacio = ref<EstadoPresupuesto>()
const emit = defineEmits<{ ir: [pagina: string] }>()

function textoUso(nombre: string) {
  const u = uso.value[nombre]
  return [t('puntos.uso', { n: u.peticiones }), u.errores ? t('puntos.usoErrores', { n: u.errores }) : '',
    u.ultima ? t('puntos.usoUltima', { cuando: haceCuanto(u.ultima) }) : ''].filter(Boolean).join(' · ')
}

function verPeticiones(nombre: string) {
  origenHistorial.value = nombre
  emit('ir', 'historial')
}

async function cargarPuntos() {
  try {
    const r = await api.puntosConUso()
    puntos.value = r.puntos_acceso
    uso.value = r.uso ?? {}
    presupuestos.value = r.presupuestos ?? {}
    presupuestoEspacio.value = await api.presupuestoInstancia()
  } catch (e) {
    avisarError(e)
  }
}

function abrirNuevo() {
  Object.assign(formulario, vacio())
  nuevo.value = true
}

async function crear() {
  const alias: Record<string, string> = {}
  for (const linea of formulario.alias.split('\n').map((l) => l.trim()).filter(Boolean)) {
    const [origen, destino] = linea.split('=').map((p) => p.trim())
    if (!origen || !destino) {
      avisarError(new Error(t('puntos.aliasInvalido', { linea })))
      return
    }
    alias[origen] = destino
  }
  guardando.value = true
  try {
    const punto: ConfigPunto = {
      nombre: formulario.nombre.trim(),
      grupo: formulario.grupo.trim() || undefined,
      puerto: formulario.direccion === 'puerto' ? Number(formulario.puerto) : undefined,
      contratos: formulario.contratos,
      modelos_permitidos: formulario.permitidos.length ? formulario.permitidos : undefined,
      alias_modelos: Object.keys(alias).length ? alias : undefined,
      presupuesto: presupuestoLimpio(formulario.presupuesto),
      ...configDeReceta(formulario.receta), // lanza un error legible si el esquema no es válido
    }
    const r = await api.crearPunto(punto)
    nuevo.value = false
    await cargarPuntos()
    const url = r.punto_acceso?.url ?? puntos.value.find((p) => p.nombre === punto.nombre)?.url ?? ''
    elegido.value = { nombre: punto.nombre, url, clave: r.token }
    recienCreado.value = true
    dialogoClave.value = true
  } catch (e) {
    avisarError(e)
  } finally {
    guardando.value = false
  }
}

async function activar(nombre: string, valor: boolean) {
  try {
    await api.activarPunto(nombre, valor)
    await cargarPuntos()
  } catch (e) {
    avisarError(e)
  }
}

// activarGrupo cambia de uno en uno los puntos del grupo que no están ya en el estado pedido.
async function activarGrupo(grupo: string, valor: boolean) {
  ocupado.value = grupo + (valor ? ':1' : ':0')
  try {
    for (const p of puntos.value.filter((p) => (p.grupo ?? '') === grupo && activo(p) !== valor)) await api.activarPunto(p.nombre, valor)
  } catch (e) {
    avisarError(e)
  } finally {
    ocupado.value = ''
    await cargarPuntos()
  }
}

function eliminar(nombre: string) {
  $q.dialog({ title: t('puntos.eliminarTitulo', { nombre }), message: t('puntos.eliminarTexto'),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'negative', label: t('comun.eliminar'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        await api.eliminarPunto(nombre)
        await cargarPuntos()
      } catch (e) {
        avisarError(e)
      }
    })
}

const dialogoInstrucciones = ref(false)
const editando = ref('')
const ajustes = reactive({ receta: recetaVacia(), permitidos: [] as string[], presupuesto: {} as Presupuesto })

function abrirAjustes(p: EstadoPunto) {
  editando.value = p.nombre
  Object.assign(ajustes, { receta: recetaDePunto(p), permitidos: [...(p.modelos_permitidos ?? [])],
    presupuesto: JSON.parse(JSON.stringify(p.presupuesto ?? {})) as Presupuesto })
  dialogoInstrucciones.value = true
}

// abrirEnProbar lleva la receta de la API a Probar para probarla y actualizarla.
function abrirEnProbar(nombre: string) {
  recetaInicial.value = nombre
  emit('ir', 'ejecutar')
}

async function guardarAjustes() {
  guardando.value = true
  try {
    await api.ajustesPunto(editando.value, { ...ajustesDeReceta(ajustes.receta), modelos_permitidos: ajustes.permitidos,
      presupuesto: presupuestoLimpio(ajustes.presupuesto) ?? null })
    dialogoInstrucciones.value = false
    avisar(t('puntos.ajustesGuardados'))
    await cargarPuntos()
  } catch (e) {
    avisarError(e)
  } finally {
    guardando.value = false
  }
}

// reiniciarPresupuesto pone a cero el consumo del día y del mes de una API, o del espacio sin nombre.
function reiniciarPresupuesto(nombre?: string) {
  $q.dialog({ title: nombre ? t('presupuesto.reiniciarTitulo', { nombre }) : t('presupuesto.reiniciarEspacio'), message: nombre ? t('presupuesto.reiniciarTexto') : t('presupuesto.reiniciarEspacioTexto'),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'primary', label: t('presupuesto.reiniciar'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        await api.reiniciarPresupuesto(nombre)
        avisar(t('presupuesto.reiniciado'))
        await cargarPuntos()
      } catch (e) {
        avisarError(e)
      }
    })
}

// regenerarClaveInstancia cambia la clave de «Conectar» por si se filtró; las aplicaciones que la usan dejan de
// conectar hasta que se les ponga la nueva.
function regenerarClaveInstancia() {
  $q.dialog({ title: t('conectar.regenerarTitulo'), message: t('conectar.regenerarTexto'),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'negative', label: t('conectar.regenerarClave'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        clave.value = (await api.regenerarClaveInstancia()).token
        verClave.value = true
        avisar(t('conectar.claveRegenerada'))
      } catch (e) {
        avisarError(e)
      }
    })
}

// regenerarClave crea una clave nueva y la muestra esta única vez: Tramtai solo guarda su huella.
function regenerarClave(p: EstadoPunto) {
  $q.dialog({ title: t('puntos.regenerarTitulo', { nombre: p.nombre }), message: t('puntos.regenerarTexto'),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'primary', label: t('puntos.regenerarClave'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        elegido.value = { nombre: p.nombre, url: p.url ?? '', clave: await api.regenerarClave(p.nombre) }
        recienCreado.value = false
        dialogoClave.value = true
      } catch (e) {
        avisarError(e)
      }
    })
}

onMounted(async () => {
  cargarPuntos()
  api.catalogo().then((m) => (catalogo.value = m)).catch(avisarError)
  api.instancia().then((i) => (base.value = i.base)).catch(() => {})
  api.claveInstancia().then((r) => (clave.value = r.token)).catch(avisarError)
})
</script>
