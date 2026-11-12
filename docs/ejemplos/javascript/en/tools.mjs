// Tool calling: your app offers functions, the agent asks for one, your app runs it and sends back the result.
import OpenAI from 'openai'

const BASE = process.env.TRAMTAI_URL ?? 'http://127.0.0.1:17373'
const KEY = process.env.TRAMTAI_KEY ?? '<key from My APIs>'
const MODEL = process.env.TRAMTAI_MODEL ?? 'claude/haiku'

const tools = [{
  type: 'function',
  function: {
    name: 'get_weather',
    description: 'Current weather for a city',
    parameters: { type: 'object', properties: { city: { type: 'string' } }, required: ['city'] },
  },
}]

// getWeather is your app's function: here you would call a real service.
const getWeather = ({ city }) => `18 °C and clear in ${city}`

const client = new OpenAI({ baseURL: `${BASE}/openai/v1`, apiKey: KEY })
const messages = [{ role: 'user', content: "What's the weather like in Santiago?" }]
let done = false
for (let round = 0; round < 5 && !done; round++) { // a cap: if the agent keeps asking for tools, quota isn't spent forever
  const message = (await client.chat.completions.create({ model: MODEL, messages, tools })).choices[0].message
  if (!message.tool_calls?.length) {
    console.log(message.content)
    done = true
    break
  }
  // The assistant's turn with its calls, and one result for each.
  messages.push({ role: 'assistant', content: message.content, tool_calls: message.tool_calls })
  for (const call of message.tool_calls) {
    const args = JSON.parse(call.function.arguments)
    console.log(`The agent asks for ${call.function.name}(${JSON.stringify(args)})`)
    messages.push({ role: 'tool', tool_call_id: call.id, content: getWeather(args) })
  }
}
if (!done) throw new Error('the agent kept asking for tools after 5 rounds')
