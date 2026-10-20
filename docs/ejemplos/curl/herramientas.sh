#!/bin/sh
# Herramientas por la URL compatible con OpenAI: el agente pide clima, se la «ejecuta» aquí y se le devuelve el
# resultado en un segundo turno. Necesita jq.
set -e
URL="${TRAMTAI_URL:-http://127.0.0.1:17373}"
CLAVE="${TRAMTAI_CLAVE:-<clave de Mis APIs>}"
MODELO="${TRAMTAI_MODELO:-claude/haiku}"

HERRAMIENTAS='[{"type": "function", "function": {"name": "clima", "description": "El tiempo actual de una ciudad",
  "parameters": {"type": "object", "properties": {"ciudad": {"type": "string"}}, "required": ["ciudad"]}}}]'
PREGUNTA='{"role": "user", "content": "¿Qué tiempo hace en Santiago?"}'

pedir() {
  curl -sf "$URL/openai/v1/chat/completions" -H "Authorization: Bearer $CLAVE" -H "Content-Type: application/json" \
    -d '{"model": "'"$MODELO"'", "tools": '"$HERRAMIENTAS"', "messages": '"$1"'}'
}

MENSAJE=$(pedir "[$PREGUNTA]" | jq -c '.choices[0].message')
LLAMADA=$(echo "$MENSAJE" | jq -c '.tool_calls[0] // empty')
if [ -z "$LLAMADA" ]; then
  echo "$MENSAJE" | jq -r .content # respondió sin pedir la herramienta
  exit 0
fi
echo "El agente pide $(echo "$LLAMADA" | jq -r '.function.name + .function.arguments')"
RESULTADO=$(jq -cn --arg id "$(echo "$LLAMADA" | jq -r .id)" '{role: "tool", tool_call_id: $id, content: "18 °C y despejado en Santiago"}')
pedir "[$PREGUNTA, $MENSAJE, $RESULTADO]" | jq -r '.choices[0].message.content'
