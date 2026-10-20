// Una respuesta en JSON que cumple un esquema: Tramtai la valida antes de devolverla.
import OpenAI from 'openai'

const BASE = process.env.TRAMTAI_URL ?? 'http://127.0.0.1:17373'
const CLAVE = process.env.TRAMTAI_CLAVE ?? '<clave de Mis APIs>'
const MODELO = process.env.TRAMTAI_MODELO ?? 'claude/haiku'

const esquema = {
  type: 'object',
  properties: { resumen: { type: 'string' } },
  required: ['resumen'],
  additionalProperties: false,
}

const cliente = new OpenAI({ baseURL: `${BASE}/openai/v1`, apiKey: CLAVE })
const respuesta = await cliente.chat.completions.create({
  model: MODELO,
  messages: [{ role: 'user', content: 'Resume en una frase qué es Tramtai.' }],
  response_format: { type: 'json_schema', json_schema: { name: 'resumen', schema: esquema } },
})
const datos = JSON.parse(respuesta.choices[0].message.content)
console.log('resumen:', datos.resumen)
