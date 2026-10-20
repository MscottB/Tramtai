// Tu primer mensaje por la URL compatible con OpenAI, con la respuesta en vivo.
import OpenAI from 'openai'

const BASE = process.env.TRAMTAI_URL ?? 'http://127.0.0.1:17373'
const CLAVE = process.env.TRAMTAI_CLAVE ?? '<clave de Mis APIs>'
const MODELO = process.env.TRAMTAI_MODELO ?? 'claude/haiku'

const cliente = new OpenAI({ baseURL: `${BASE}/openai/v1`, apiKey: CLAVE })
const flujo = await cliente.chat.completions.create({
  model: MODELO,
  messages: [{ role: 'user', content: 'Hola, ¿quién eres?' }],
  stream: true,
})
for await (const trozo of flujo) {
  process.stdout.write(trozo.choices[0]?.delta?.content ?? '')
}
process.stdout.write('\n')
