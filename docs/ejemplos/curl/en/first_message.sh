#!/bin/sh
# Your first message through the OpenAI-compatible URL, with the response streamed live (SSE events).
URL="${TRAMTAI_URL:-http://127.0.0.1:17373}"
KEY="${TRAMTAI_KEY:-<key from My APIs>}"
MODEL="${TRAMTAI_MODEL:-claude/haiku}"

curl -sN "$URL/openai/v1/chat/completions" \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"model": "'"$MODEL"'", "stream": true, "messages": [{"role": "user", "content": "Hi, who are you?"}]}'
