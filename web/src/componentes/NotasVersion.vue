<template>
  <div class="notas">
    <template v-if="analizadas.secciones.length">
      <p v-if="analizadas.resumen" class="notas__resumen">{{ analizadas.resumen }}</p>
      <section v-for="s in analizadas.secciones" :key="s.titulo" class="notas__seccion" :class="'notas--' + s.tipo">
        <div class="notas__titulo"><span class="notas__chapa" />{{ s.titulo }}</div>
        <ol class="notas__linea">
          <li v-for="(item, i) in s.items" :key="i" class="notas__parada">
            <div class="notas__item">{{ item.titulo }}</div>
            <ul v-if="item.detalles.length" class="notas__detalles">
              <li v-for="(d, j) in item.detalles" :key="j">{{ d }}</li>
            </ul>
          </li>
        </ol>
      </section>
    </template>
    <p v-else class="notas__texto">{{ textoLibre }}</p>
    <q-btn v-if="enlace" flat dense no-caps color="primary" icon-right="o_open_in_new" class="q-px-none"
      :label="t('actualizacion.todosLosCambios')" :href="enlace" target="_blank" rel="noopener" />
  </div>
</template>

<script setup lang="ts">
// Las notas de cada versión salen de scripts/version.ps1: un título, un resumen y, si hace falta, secciones
// («Novedades:», «Mejoras:», «Correcciones:», o en inglés «New:», «Fixes:»…) con elementos «- …» y sus detalles «  - …».
// Se dibujan como la red de la app: cada sección es una línea y cada cambio, una estación. Sin secciones, el texto se
// muestra con sus párrafos y viñetas. Las notas terminan con el enlace a la comparación en GitHub, que se muestra como
// el botón «Ver todos los cambios en GitHub» (también en las versiones anteriores, con la página del release).
// Desde la v0.44 llevan un bloque «## English»: en inglés se muestra ese bloque; las anteriores se ven en español.
import { computed } from 'vue'
import { enlaceSeguro } from '../enlaces'
import { idioma, t } from '../i18n'

const props = defineProps<{ texto: string; pagina?: string }>()
const enlace = computed(() => enlaceSeguro(props.pagina))

const partes = computed(() => {
  const [es, en] = props.texto.split(/^## English[ \t]*\r?$/m, 2)
  return { es: es.trimEnd(), en: en?.trim() ?? '' }
})
const enIngles = computed(() => idioma.value === 'en' && partes.value.en !== '')
const elegido = computed(() => (enIngles.value ? partes.value.en : partes.value.es))
// Sin secciones, el texto tal cual, sin el título ni la línea con el enlace (va como botón).
const textoLibre = computed(() =>
  elegido.value
    .split('\n')
    .filter((l) => !/^Tramtai v\S+$/.test(l.trim()) && !/^(Full list of changes|(Cambios técnicos|Technical changes): https?:)/.test(l.trim()))
    .join('\n')
    .trim(),
)

type Tipo = 'nuevo' | 'arreglo' | 'incompatible' | 'otro'
interface Item {
  titulo: string
  detalles: string[]
}
interface Seccion {
  titulo: string
  tipo: Tipo
  items: Item[]
}

function tipoDe(titulo: string): Tipo {
  if (/incompatible|breaking/i.test(titulo)) return 'incompatible'
  if (/nueva|nuevo|novedad|funci|new|feature/i.test(titulo)) return 'nuevo'
  if (/correcci|arreglo|fix/i.test(titulo)) return 'arreglo'
  return 'otro'
}

const mayuscula = (t: string) => t.charAt(0).toUpperCase() + t.slice(1)

const analizadas = computed(() => {
  const secciones: Seccion[] = []
  const resumen: string[] = []
  let seccion: Seccion | null = null
  let item: Item | null = null
  for (const bruta of elegido.value.split('\n')) {
    const linea = bruta.trimEnd()
    const limpia = linea.trim()
    if (!limpia || /^Tramtai v\S+$/.test(limpia) || /^Cambios desde .*:$/.test(limpia) || /^Full list of changes/.test(limpia) ||
      /^(Cambios técnicos|Technical changes): https?:\/\//.test(limpia)) continue
    if (!/^[\s-]/.test(linea) && limpia.endsWith(':')) {
      seccion = { titulo: limpia.slice(0, -1), tipo: tipoDe(limpia), items: [] }
      secciones.push(seccion)
      item = null
    } else if (seccion && linea.startsWith('- ')) {
      item = { titulo: mayuscula(limpia.slice(2)), detalles: [] }
      seccion.items.push(item)
    } else if (item && limpia.startsWith('- ')) {
      item.detalles.push(mayuscula(limpia.slice(2)))
    } else if (item) {
      // línea que continúa la anterior (mensajes de commit partidos en varias líneas)
      if (item.detalles.length) item.detalles[item.detalles.length - 1] += ' ' + limpia
      else item.titulo += ' ' + limpia
    } else if (!seccion) {
      resumen.push(limpia)
    }
  }
  return { resumen: resumen.join(' '), secciones: secciones.filter((s) => s.items.length) }
})
</script>

<style scoped>
.notas {
  max-height: 340px;
  overflow-y: auto;
  padding: 2px 6px 2px 0;
}
.notas__resumen {
  margin: 0 0 14px;
  line-height: 1.55;
}
.notas__texto {
  margin: 0;
  white-space: pre-wrap;
  font-size: 0.875rem;
  line-height: 1.55;
}
.notas__seccion {
  --color: var(--borde-fuerte);
}
.notas__seccion + .notas__seccion {
  margin-top: 16px;
}
.notas--nuevo {
  --color: var(--acento);
}
.notas--arreglo {
  --color: var(--aviso);
}
.notas--incompatible {
  --color: var(--error);
}
.notas__titulo {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  font-family: var(--fuente-titulo);
  font-size: 0.74rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--texto-suave);
}
.notas__chapa {
  width: 22px;
  height: 6px;
  border-radius: 3px;
  background: var(--color);
}
.notas__linea {
  position: relative;
  list-style: none;
  margin: 0;
  padding: 0 0 0 26px;
}
.notas__linea::before {
  content: '';
  position: absolute;
  left: 8px;
  top: 8px;
  bottom: 8px;
  width: 4px;
  border-radius: 2px;
  background: var(--color);
}
.notas__parada {
  position: relative;
  padding-bottom: 12px;
}
.notas__parada:last-child {
  padding-bottom: 0;
}
.notas__parada::before {
  content: '';
  position: absolute;
  left: -24px;
  top: 3px;
  width: 16px;
  height: 16px;
  box-sizing: border-box;
  border-radius: 50%;
  background: var(--superficie);
  border: 4px solid var(--color);
}
.notas__item {
  font-weight: 600;
  line-height: 1.45;
}
.notas__detalles {
  margin: 4px 0 0;
  padding-left: 16px;
  color: var(--texto-suave);
  font-size: 0.84rem;
  line-height: 1.5;
}
.notas__detalles li + li {
  margin-top: 2px;
}
</style>
