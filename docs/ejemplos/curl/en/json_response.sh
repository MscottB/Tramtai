#!/bin/sh
# A JSON response that follows a schema: Tramtai validates it before returning it.
URL="${TRAMTAI_URL:-http://127.0.0.1:17373}"
KEY="${TRAMTAI_KEY:-<key from My APIs>}"
MODEL="${TRAMTAI_MODEL:-claude/haiku}"

curl -s "$URL/openai/v1/chat/completions" \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"model": "'"$MODEL"'", "messages": [{"role": "user", "content": "Summarize in one sentence what Tramtai is."}],
       "response_format": {"type": "json_schema", "json_schema": {"name": "summary",
         "schema": {"type": "object", "properties": {"summary": {"type": "string"}}, "required": ["summary"]}}}}'
