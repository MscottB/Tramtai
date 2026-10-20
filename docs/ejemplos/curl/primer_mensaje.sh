#!/bin/sh
# Tu primer mensaje por la URL compatible con OpenAI, con la respuesta en vivo (eventos SSE).
URL="${TRAMTAI_URL:-http://127.0.0.1:17373}"
CLAVE="${TRAMTAI_CLAVE:-<clave de Mis APIs>}"
MODELO="${TRAMTAI_MODELO:-claude/haiku}"

curl -sN "$URL/openai/v1/chat/completions" \
  -H "Authorization: Bearer $CLAVE" -H "Content-Type: application/json" \
  -d '{"model": "'"$MODELO"'", "stream": true, "messages": [{"role": "user", "content": "Hola, ¿quién eres?"}]}'
