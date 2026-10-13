<template>
  <div class="pagina apilado">
    <div>
      <div class="titulo-pagina">{{ t('menu.conectar') }}</div>
      <div class="texto-suave">{{ t('conectar.subtitulo') }}</div>
    </div>

    <q-card>
      <q-card-section>
        <div class="titulo-seccion">{{ t('conectar.directa') }}</div>
        <div class="texto-suave text-body2">{{ t('conectar.directaTexto') }}</div>
      </q-card-section>
      <q-card-section class="row q-col-gutter-md q-pt-none">
        <div v-for="c in campos" :key="c.titulo" class="col-12 col-md-6">
          <div class="text-caption texto-suave q-mb-xs">{{ c.titulo }}</div>
          <div class="bloque-codigo">{{ c.valor }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('comun.copiar') + ' ' + c.titulo" @click="copiar(c.valor)" /></div>
        </div>
        <div class="col-12 col-md-6">
          <div class="text-caption texto-suave q-mb-xs">{{ t('conectar.clave') }}</div>
          <div class="bloque-codigo" style="padding-right: 80px">{{ clave ? (verClave ? clave : ocultar(clave)) : t('comun.cargando') }}
            <div class="boton-copiar row no-wrap">
              <q-btn flat dense round size="sm" :icon="verClave ? 'visibility_off' : 'visibility'" :aria-label="verClave ? t('conectar.ocultarClave') : t('conectar.mostrarClave')" @click="verClave = !verClave" />
              <q-btn flat dense round size="sm" icon="content_copy" :aria-label="t('conectar.copiarClave')" :disable="!clave" @click="copiar(clave, t('conectar.claveCopiada'))" />
            </div>
          </div>
        </div>
      </q-card-section>
      <q-card-section class="q-pt-none">
        <div class="text-caption texto-suave q-mb-xs">{{ t('conectar.modeloAntes') }} <span class="codigo">model</span> {{ t('conectar.modeloDespues') }}</div>
        <div class="row q-gutter-xs">
          <q-chip v-for="m in ejemplosModelo" :key="m" dense clickable outline color="primary" class="codigo" @click="copiar(m)">{{ m }}</q-chip>
        </div>
      </q-card-section>
      <q-separator />
      <div class="q-px-md q-pt-md text-weight-medium">{{ t('conectar.ejemplos') }}</div>
      <q-tabs v-model="lenguaje" dense no-caps align="left" active-color="primary" indicator-color="primary" class="q-px-sm">
        <q-tab v-for="e in ejemplos" :key="e.id" :name="e.id" :label="e.titulo()" />
      </q-tabs>
      <q-separator />
      <q-card-section>
        <div class="bloque-codigo sin-ajuste">{{ ejemplo.codigo(verClave ? clave : ocultar(clave)) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('conectar.copiarEjemplo')" @click="copiar(ejemplo.codigo(clave), t('conectar.ejemploCopiado'))" /></div>
      </q-card-section>
    </q-card>

    <q-card>
      <q-card-section class="row items-center q-col-gutter-md">
        <div class="col-12 col-sm">
          <div class="titulo-seccion">{{ t('puntos.titulo') }}</div>
          <div class="texto-suave text-body2">{{ t('puntos.texto') }}</div>
        </div>
        <div class="col-12 col-sm-auto">
          <q-btn unelevated color="primary" icon="add" :label="t('puntos.nuevo')" no-caps @click="abrirNuevo" />
        </div>
      </q-card-section>
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
          <q-list separator>
            <q-item v-for="p in g.puntos" :key="p.nombre" class="q-py-md">
              <q-item-section avatar><q-icon name="o_hub" :color="activo(p) ? 'primary' : 'grey-5'" /></q-item-section>
              <q-item-section>
                <q-item-label class="text-weight-medium">{{ p.nombre }}</q-item-label>
                <q-item-label caption class="codigo">{{ p.url || t('puntos.sinDireccion') }}</q-item-label>
                <q-item-label caption>
                  {{ p.contratos.map(nombreContrato).join(' · ') }}
                  <template v-if="p.agente || p.modelo"> · {{ [nombreAgente(p.agente), p.modelo].filter(Boolean).join(' · ') }}</template>
                </q-item-label>
                <q-item-label caption>
                  <a v-if="uso[p.nombre]" href="#historial" class="enlace" @click.prevent="verPeticiones(p.nombre)">{{ textoUso(p.nombre) }}</a>
                  <span v-else>{{ t('puntos.sinUso') }}</span>
                </q-item-label>
                <q-item-label v-if="p.instrucciones" caption class="row no-wrap items-start">
                  <q-icon name="o_notes" class="q-mr-xs q-mt-xs" /><span class="recortar-2">{{ p.instrucciones }}</span>
                </q-item-label>
                <q-item-label v-if="p.error && p.error !== 'desactivado'" caption class="text-negative">{{ p.error }}</q-item-label>
              </q-item-section>
              <q-item-section side>
                <div class="row items-center no-wrap q-gutter-x-xs">
                  <q-toggle :model-value="activo(p)" :aria-label="t('puntos.activar', { nombre: p.nombre })" @update:model-value="(v: boolean) => activar(p.nombre, v)">
                    <q-tooltip>{{ activo(p) ? t('puntos.activo') : t('puntos.desactivado') }}</q-tooltip>
                  </q-toggle>
                  <q-btn v-if="p.url" flat dense round icon="content_copy" :aria-label="t('puntos.copiarUrl')" @click="copiar(p.url)"><q-tooltip>{{ t('puntos.copiarUrl') }}</q-tooltip></q-btn>
                  <q-btn flat dense round icon="o_key" :aria-label="t('puntos.verClave')" @click="mostrarClave(p)"><q-tooltip>{{ t('puntos.verClave') }}</q-tooltip></q-btn>
                  <q-btn flat dense round icon="o_edit_note" :aria-label="t('puntos.editarInstrucciones')" @click="abrirInstrucciones(p)"><q-tooltip>{{ t('puntos.editarInstrucciones') }}</q-tooltip></q-btn>
                  <q-btn flat dense round icon="o_delete" color="negative" :aria-label="t('comun.eliminar')" @click="eliminar(p.nombre)"><q-tooltip>{{ t('comun.eliminar') }}</q-tooltip></q-btn>
                </div>
              </q-item-section>
            </q-item>
          </q-list>
        </div>
      </template>
      <div v-else class="vacio">
        <q-icon name="o_hub" size="40px" />
        <div class="q-mt-sm">{{ t('puntos.vacio') }}</div>
      </div>
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
            <q-input v-model="formulario.instrucciones" type="textarea" autogrow outlined dense :label="t('puntos.instrucciones')"
              :placeholder="t('puntos.instruccionesEjemplo')" :hint="t('puntos.instruccionesHint')" />
            <q-select :model-value="formulario.grupo" :options="sugerencias" outlined dense use-input hide-selected fill-input input-debounce="0"
              :label="t('puntos.grupo')" :hint="t('puntos.grupoHint')" maxlength="40" hide-dropdown-icon
              @input-value="(v: string) => (formulario.grupo = v)" @filter="filtrarGrupos" @update:model-value="(v: string) => (formulario.grupo = v)" />
            <q-select v-model="formulario.modelo" :options="opciones" emit-value map-options outlined dense clearable :label="t('puntos.modelo')">
              <template #option="scope">
                <q-item v-bind="scope.itemProps">
                  <q-item-section><q-item-label>{{ scope.opt.label }}</q-item-label><q-item-label caption>{{ scope.opt.caption }}</q-item-label></q-item-section>
                </q-item>
              </template>
            </q-select>
            <q-toggle v-model="formulario.forzar" :disable="!formulario.modelo" :label="t('puntos.forzar')" />
            <q-expansion-item dense icon="o_tune" :label="t('comun.opcionesAvanzadas')" header-class="q-px-none text-body2">
              <div class="q-gutter-y-md q-pt-sm">
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
                  :label="t('puntos.permitidos')" :hint="t('puntos.permitidosHint')" />
                <q-input v-model="formulario.alias" type="textarea" autogrow outlined dense :label="t('puntos.alias')" input-class="codigo"
                  :hint="t('puntos.aliasHint')" />
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
          <div class="titulo-seccion">{{ t('puntos.instruccionesDe', { nombre: editando }) }}</div>
          <div class="texto-suave text-body2">{{ t('puntos.instruccionesHint') }}</div>
        </q-card-section>
        <q-card-section class="q-pt-none">
          <q-input v-model="textoInstrucciones" type="textarea" autogrow autofocus outlined :placeholder="t('puntos.instruccionesEjemplo')"
            :input-style="{ minHeight: '120px' }" />
        </q-card-section>
        <q-card-actions align="right" class="q-pa-md">
          <q-btn v-close-popup flat no-caps :label="t('comun.cancelar')" />
          <q-btn unelevated color="primary" no-caps :label="t('comun.guardar')" :loading="guardando" @click="guardarInstrucciones" />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <q-dialog v-model="dialogoClave">
      <q-card style="width: 560px; max-width: 95vw">
        <q-card-section>
          <div class="titulo-seccion">{{ recienCreado ? t('puntos.listo') : t('puntos.conexionDe', { nombre: elegido?.nombre ?? '' }) }}</div>
          <div class="texto-suave text-body2">{{ t('puntos.configura') }}</div>
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
import { api, aplicarModelo, avisar, avisarError, copiar, haceCuanto, nombreAgente, opcionModelo, origenHistorial, type ConfigPunto, type EstadoPunto, type Modelo, type UsoPunto } from '../api'
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
  { titulo: t('conectar.urlOpenai'), valor: base.value + '/openai/v1' },
  { titulo: t('conectar.urlAnthropic'), valor: base.value + '/anthropic' },
  { titulo: t('conectar.apiPropia'), valor: base.value + '/api/v1' },
])
const ejemplosModelo = computed(() => {
  const perfiles = ids.value.filter((id) => id.startsWith('perfil/'))
  const modelos = ids.value.filter((id) => !id.startsWith('perfil/') && id.includes('/'))
  return [...perfiles, ...modelos].slice(0, 10)
})
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
const vacio = () => ({ nombre: '', instrucciones: '', grupo: '', direccion: 'ruta', puerto: '', contratos: [...contratos], modelo: null as string | null,
  permitidos: [] as string[], alias: '', forzar: false })
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
  const elegidoModelo = aplicarModelo({ mensajes: [] }, formulario.modelo ?? '')
  const punto: ConfigPunto = {
    nombre: formulario.nombre.trim(),
    grupo: formulario.grupo.trim() || undefined,
    instrucciones: formulario.instrucciones.trim() || undefined,
    puerto: formulario.direccion === 'puerto' ? Number(formulario.puerto) : undefined,
    contratos: formulario.contratos,
    agente: elegidoModelo.agente,
    modelo: elegidoModelo.modelo,
    perfil_modelo: elegidoModelo.perfil_modelo,
    modelos_permitidos: formulario.permitidos.length ? formulario.permitidos : undefined,
    alias_modelos: Object.keys(alias).length ? alias : undefined,
    forzar_modelo: (formulario.forzar && !!formulario.modelo) || undefined,
  }
  guardando.value = true
  try {
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
const textoInstrucciones = ref('')

function abrirInstrucciones(p: EstadoPunto) {
  editando.value = p.nombre
  textoInstrucciones.value = p.instrucciones ?? ''
  dialogoInstrucciones.value = true
}

async function guardarInstrucciones() {
  guardando.value = true
  try {
    await api.instruccionesPunto(editando.value, textoInstrucciones.value)
    dialogoInstrucciones.value = false
    avisar(t('puntos.instruccionesGuardadas'))
    await cargarPuntos()
  } catch (e) {
    avisarError(e)
  } finally {
    guardando.value = false
  }
}

async function mostrarClave(p: EstadoPunto) {
  try {
    elegido.value = { nombre: p.nombre, url: p.url ?? '', clave: await api.tokenPunto(p.nombre) }
    recienCreado.value = false
    dialogoClave.value = true
  } catch (e) {
    avisarError(e)
  }
}

onMounted(async () => {
  cargarPuntos()
  api.catalogo().then((m) => (catalogo.value = m)).catch(avisarError)
  api.instancia().then((i) => (base.value = i.base)).catch(() => {})
  api.claveInstancia().then((r) => (clave.value = r.token)).catch(avisarError)
})
</script>
