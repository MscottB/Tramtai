// A file with the message: it travels as base64 inside the request and Tramtai turns it into text for the agent.
import { readFile } from 'node:fs/promises'
import OpenAI from 'openai'

const BASE = process.env.TRAMTAI_URL ?? 'http://127.0.0.1:17373'
const KEY = process.env.TRAMTAI_KEY ?? '<key from My APIs>'
const MODEL = process.env.TRAMTAI_MODEL ?? 'claude/haiku'

const data = (await readFile(new URL('../../sample.txt', import.meta.url))).toString('base64')

const client = new OpenAI({ baseURL: `${BASE}/openai/v1`, apiKey: KEY })
const response = await client.chat.completions.create({
  model: MODEL,
  messages: [{
    role: 'user',
    content: [
      { type: 'text', text: 'What does this file say?' },
      { type: 'file', file: { filename: 'sample.txt', file_data: `data:text/plain;base64,${data}` } },
    ],
  }],
})
console.log(response.choices[0].message.content)
