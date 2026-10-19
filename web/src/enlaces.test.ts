import { test } from 'node:test'
import assert from 'node:assert/strict'
import { enlaceSeguro } from './enlaces.ts'

test('solo pasan los enlaces https', () => {
  assert.equal(enlaceSeguro('https://github.com/MscottB/Tramtai/releases'), 'https://github.com/MscottB/Tramtai/releases')
  for (const malo of ['javascript:alert(1)', 'http://ejemplo.com', 'JAVASCRIPT:x', ' https://x', '', undefined]) {
    assert.equal(enlaceSeguro(malo), undefined, String(malo))
  }
})
