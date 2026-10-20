#!/bin/sh
# Una respuesta en JSON que cumple un esquema: Tramtai la valida antes de devolverla.
URL="${TRAMTAI_URL:-http://127.0.0.1:17373}"
CLAVE="${TRAMTAI_CLAVE:-<clave de Mis APIs>}"
MODELO="${TRAMTAI_MODELO:-claude/haiku}"

curl -s "$URL/openai/v1/chat/completions" \
  -H "Authorization: Bearer $CLAVE" -H "Content-Type: application/json" \
  -d '{"model": "'"$MODELO"'", "messages": [{"role": "user", "content": "Resume en una frase qué es Tramtai."}],
       "response_format": {"type": "json_schema", "json_schema": {"name": "resumen",
         "schema": {"type": "object", "properties": {"resumen": {"type": "string"}}, "required": ["resumen"]}}}}'
