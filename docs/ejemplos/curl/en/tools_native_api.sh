#!/bin/sh
# The same loop through the native API: /ejecutar waits for the response and brings "llamadas" with the arguments as
# an object. The field names (agente, mensajes, rol, contenido, herramientas…) are the API's own. Needs jq.
set -e
URL="${TRAMTAI_URL:-http://127.0.0.1:17373}"
KEY="${TRAMTAI_KEY:-<key from My APIs>}"
MODEL="${TRAMTAI_MODEL:-claude/haiku}"

AGENT="${MODEL%%/*}"
SUBMODEL=""
case "$MODEL" in */*) SUBMODEL=', "modelo": "'"${MODEL#*/}"'"' ;; esac
TOOLS='[{"nombre": "get_weather", "descripcion": "Current weather for a city",
  "parametros": {"type": "object", "properties": {"city": {"type": "string"}}, "required": ["city"]}}]'
QUESTION='{"rol": "usuario", "contenido": "What is the weather like in Santiago?"}'

run() {
  curl -sf "$URL/api/v1/ejecutar" -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
    -d '{"agente": "'"$AGENT"'"'"$SUBMODEL"', "herramientas": '"$TOOLS"', "mensajes": '"$1"'}'
}

JOB=$(run "[$QUESTION]")
CALLS=$(echo "$JOB" | jq -c '.llamadas // empty')
if [ -z "$CALLS" ]; then
  echo "$JOB" | jq -r .salida # it answered without asking for the tool
  exit 0
fi
echo "The agent asks for $(echo "$CALLS" | jq -c '.[0] | {nombre, argumentos}')"
ASSISTANT=$(jq -cn --argjson calls "$CALLS" '{rol: "asistente", contenido: "", llamadas: $calls}')
RESULT=$(jq -cn --arg id "$(echo "$CALLS" | jq -r '.[0].id')" '{rol: "herramienta", id_llamada: $id, contenido: "18 °C and clear in Santiago"}')
run "[$QUESTION, $ASSISTANT, $RESULT]" | jq -r .salida
