// Las plantillas de «Respuesta con forma»: un esquema listo para los usos más comunes, con los nombres de los campos
// en el idioma de la interfaz y unas instrucciones sugeridas. Solo tipos importados: se prueba con node --test.
import type { Campo } from './esquemaCampos'
import type { Clave } from './idiomas/es'

type Idioma = 'es' | 'en'

export interface Plantilla {
  id: string
  nombre: Clave
  descripcion: Clave
  campos: (idioma: Idioma) => Campo[]
  instrucciones: (idioma: Idioma) => string
}

const texto = (nombre: string, descripcion = '', obligatorio = true): Campo => ({ nombre, tipo: 'texto', obligatorio, descripcion })
const lista = (nombre: string, descripcion: string, extra: Partial<Campo> = {}): Campo =>
  ({ nombre, tipo: 'lista', elemento: 'texto', obligatorio: true, descripcion, ...extra })
const de = (idioma: Idioma, es: string, en: string) => (idioma === 'en' ? en : es)

export const plantillas: Plantilla[] = [
  {
    id: 'extraer', nombre: 'plantilla.extraer', descripcion: 'plantilla.extraerTexto',
    campos: (i) => [
      texto(de(i, 'nombre', 'name'), de(i, 'Nombre de la persona o empresa', 'Name of the person or company')),
      { nombre: de(i, 'fecha', 'date'), tipo: 'fecha', obligatorio: true, descripcion: de(i, 'Fecha del documento (AAAA-MM-DD)', 'Document date (YYYY-MM-DD)') },
      { nombre: 'total', tipo: 'numero', obligatorio: true, descripcion: de(i, 'Importe total, sin símbolo de moneda', 'Total amount, without currency symbol') },
    ],
    instrucciones: (i) => de(i, 'Extrae del texto los datos pedidos. Si un dato no aparece, déjalo vacío o en 0.',
      'Extract the requested data from the text. If a value does not appear, leave it empty or 0.'),
  },
  {
    id: 'clasificar', nombre: 'plantilla.clasificar', descripcion: 'plantilla.clasificarTexto',
    campos: (i) => [
      { nombre: de(i, 'categoria', 'category'), tipo: 'opcion', obligatorio: true, descripcion: de(i, 'La categoría que mejor encaja', 'The category that fits best'),
        opciones: i === 'en' ? ['inquiry', 'complaint', 'other'] : ['consulta', 'queja', 'otro'] },
      { nombre: de(i, 'confianza', 'confidence'), tipo: 'numero', obligatorio: true, minimo: 0, maximo: 1, descripcion: de(i, 'De 0 a 1', 'From 0 to 1') },
      texto(de(i, 'motivo', 'reason'), de(i, 'Por qué esa categoría, en una frase', 'Why that category, in one sentence')),
    ],
    instrucciones: (i) => de(i, 'Clasifica el texto en una de las categorías.', 'Classify the text into one of the categories.'),
  },
  {
    id: 'resumen', nombre: 'plantilla.resumen', descripcion: 'plantilla.resumenTexto',
    campos: (i) => [
      texto(de(i, 'resumen', 'summary'), de(i, 'Resumen en dos o tres frases', 'Summary in two or three sentences')),
      lista(de(i, 'puntos', 'points'), de(i, 'Las ideas clave', 'The key ideas'), { minimo: 3, maximo: 7 }),
    ],
    instrucciones: (i) => de(i, 'Resume el texto y enumera sus ideas clave.', 'Summarize the text and list its key ideas.'),
  },
  {
    id: 'sentimiento', nombre: 'plantilla.sentimiento', descripcion: 'plantilla.sentimientoTexto',
    campos: (i) => [
      { nombre: de(i, 'sentimiento', 'sentiment'), tipo: 'opcion', obligatorio: true, descripcion: '',
        opciones: i === 'en' ? ['positive', 'neutral', 'negative'] : ['positivo', 'neutral', 'negativo'] },
      texto(de(i, 'motivo', 'reason'), de(i, 'Qué lo indica, en una frase', 'What shows it, in one sentence')),
    ],
    instrucciones: (i) => de(i, 'Indica el sentimiento del texto.', 'State the sentiment of the text.'),
  },
  {
    id: 'si_no', nombre: 'plantilla.siNo', descripcion: 'plantilla.siNoTexto',
    campos: (i) => [
      { nombre: de(i, 'respuesta', 'answer'), tipo: 'si_no', obligatorio: true, descripcion: de(i, 'true para sí, false para no', 'true for yes, false for no') },
      texto(de(i, 'motivo', 'reason'), de(i, 'Por qué, en una frase', 'Why, in one sentence')),
    ],
    instrucciones: (i) => de(i, 'Responde a la pregunta con sí o no.', 'Answer the question with yes or no.'),
  },
  {
    id: 'tareas', nombre: 'plantilla.tareas', descripcion: 'plantilla.tareasTexto',
    campos: (i) => [
      lista(de(i, 'tareas', 'tasks'), de(i, 'Cada tarea que aparece en el texto', 'Each task that appears in the text'), {
        elemento: 'objeto',
        campos: [
          texto(de(i, 'tarea', 'task'), de(i, 'Qué hay que hacer', 'What needs to be done')),
          texto(de(i, 'responsable', 'owner'), de(i, 'Quién la hace, si se dice', 'Who does it, if stated'), false),
          { nombre: de(i, 'fecha', 'date'), tipo: 'fecha', obligatorio: false, descripcion: de(i, 'Fecha límite, si se dice', 'Due date, if stated') },
        ],
      }),
    ],
    instrucciones: (i) => de(i, 'Extrae las tareas del texto.', 'Extract the tasks from the text.'),
  },
  {
    id: 'entidades', nombre: 'plantilla.entidades', descripcion: 'plantilla.entidadesTexto',
    campos: (i) => [
      lista(de(i, 'personas', 'people'), ''),
      lista(de(i, 'organizaciones', 'organizations'), ''),
      lista(de(i, 'lugares', 'places'), ''),
      lista(de(i, 'fechas', 'dates'), ''),
    ],
    instrucciones: (i) => de(i, 'Enumera las personas, organizaciones, lugares y fechas que se mencionan. Una lista vacía si no hay.',
      'List the people, organizations, places and dates mentioned. An empty list if there are none.'),
  },
]
