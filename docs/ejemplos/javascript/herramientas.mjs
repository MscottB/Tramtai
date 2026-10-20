// Herramientas (tools): tu app ofrece funciones, el agente pide una, tu app la ejecuta y le devuelve el resultado.
import OpenAI from 'openai'

const BASE = process.env.TRAMTAI_URL ?? 'http://127.0.0.1:17373'
const CLAVE = process.env.TRAMTAI_CLAVE ?? '<clave de Mis APIs>'
const MODELO = process.env.TRAMTAI_MODELO ?? 'claude/haiku'

const herramientas = [{
  type: 'function',
  function: {
    name: 'clima',
    description: 'El tiempo actual de una ciudad',
    parameters: { type: 'object', properties: { ciudad: { type: 'string' } }, required: ['ciudad'] },
  },
}]

// clima es la función de tu app: aquí consultarías un servicio real.
const clima = ({ ciudad }) => `18 °C y despejado en ${ciudad}`

const cliente = new OpenAI({ baseURL: `${BASE}/openai/v1`, apiKey: CLAVE })
const mensajes = [{ role: 'user', content: '¿Qué tiempo hace en Santiago?' }]
let terminado = false
for (let ronda = 0; ronda < 5 && !terminado; ronda++) { // un tope: si el agente sigue pidiendo herramientas, no se gasta cuota sin fin
  const mensaje = (await cliente.chat.completions.create({ model: MODELO, messages: mensajes, tools: herramientas })).choices[0].message
  if (!mensaje.tool_calls?.length) {
    console.log(mensaje.content)
    terminado = true
    break
  }
  // El turno del asistente con sus llamadas, y un resultado por cada una.
  mensajes.push({ role: 'assistant', content: mensaje.content, tool_calls: mensaje.tool_calls })
  for (const llamada of mensaje.tool_calls) {
    const argumentos = JSON.parse(llamada.function.arguments)
    console.log(`El agente pide ${llamada.function.name}(${JSON.stringify(argumentos)})`)
    mensajes.push({ role: 'tool', tool_call_id: llamada.id, content: clima(argumentos) })
  }
}
if (!terminado) throw new Error('el agente siguió pidiendo herramientas tras 5 rondas')
