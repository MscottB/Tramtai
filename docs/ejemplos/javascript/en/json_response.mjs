// A JSON response that follows a schema: Tramtai validates it before returning it.
import OpenAI from 'openai'

const BASE = process.env.TRAMTAI_URL ?? 'http://127.0.0.1:17373'
const KEY = process.env.TRAMTAI_KEY ?? '<key from My APIs>'
const MODEL = process.env.TRAMTAI_MODEL ?? 'claude/haiku'

const schema = {
  type: 'object',
  properties: { summary: { type: 'string' } },
  required: ['summary'],
  additionalProperties: false,
}

const client = new OpenAI({ baseURL: `${BASE}/openai/v1`, apiKey: KEY })
const response = await client.chat.completions.create({
  model: MODEL,
  messages: [{ role: 'user', content: 'Summarize in one sentence what Tramtai is.' }],
  response_format: { type: 'json_schema', json_schema: { name: 'summary', schema } },
})
const data = JSON.parse(response.choices[0].message.content)
console.log('summary:', data.summary)
