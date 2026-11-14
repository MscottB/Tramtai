#!/bin/sh
# A file through the native API: it is uploaded to /cargas and the job cites it by its id. Needs jq.
# The field names (archivo, agente, mensajes, rol, contenido…) are the API's own and stay as they are.
set -e
URL="${TRAMTAI_URL:-http://127.0.0.1:17373}"
KEY="${TRAMTAI_KEY:-<key from My APIs>}"
MODEL="${TRAMTAI_MODEL:-claude/haiku}"

DIR="$(dirname "$0")"
AGENT="${MODEL%%/*}"
SUBMODEL=""
case "$MODEL" in */*) SUBMODEL=', "modelo": "'"${MODEL#*/}"'"' ;; esac
UPLOAD=$(curl -sf "$URL/api/v1/cargas" -H "Authorization: Bearer $KEY" -F "archivo=@$DIR/../../sample.txt" | jq -r .identificador_carga)
curl -sf "$URL/api/v1/ejecutar" -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"agente": "'"$AGENT"'"'"$SUBMODEL"', "cargas": ["'"$UPLOAD"'"], "mensajes": [{"rol": "usuario", "contenido": "What does this file say?"}]}' \
  | jq -r .salida
