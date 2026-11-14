// Your first message through the OpenAI-compatible URL, with the response streamed live.
import OpenAI from 'openai'

const BASE = process.env.TRAMTAI_URL ?? 'http://127.0.0.1:17373'
const KEY = process.env.TRAMTAI_KEY ?? '<key from My APIs>'
const MODEL = process.env.TRAMTAI_MODEL ?? 'claude/haiku'

const client = new OpenAI({ baseURL: `${BASE}/openai/v1`, apiKey: KEY })
const stream = await client.chat.completions.create({
  model: MODEL,
  messages: [{ role: 'user', content: 'Hi, who are you?' }],
  stream: true,
})
for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0]?.delta?.content ?? '')
}
process.stdout.write('\n')
