// Un archivo junto al mensaje: va en base64 dentro de la petición y Tramtai lo convierte a texto para el agente.
import { readFile } from 'node:fs/promises'
import OpenAI from 'openai'

const BASE = process.env.TRAMTAI_URL ?? 'http://127.0.0.1:17373'
const CLAVE = process.env.TRAMTAI_CLAVE ?? '<clave de Mis APIs>'
const MODELO = process.env.TRAMTAI_MODELO ?? 'claude/haiku'

const datos = (await readFile(new URL('../muestra.txt', import.meta.url))).toString('base64')

const cliente = new OpenAI({ baseURL: `${BASE}/openai/v1`, apiKey: CLAVE })
const respuesta = await cliente.chat.completions.create({
  model: MODELO,
  messages: [{
    role: 'user',
    content: [
      { type: 'text', text: '¿Qué dice este archivo?' },
      { type: 'file', file: { filename: 'muestra.txt', file_data: `data:text/plain;base64,${datos}` } },
    ],
  }],
})
console.log(respuesta.choices[0].message.content)
