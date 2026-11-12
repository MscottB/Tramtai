#!/bin/sh
# A long job through the native API: create it, follow it live until "fin" and read the result. If the connection
# drops, resume with -H "Last-Event-ID: <last id>". Needs jq.
set -e
URL="${TRAMTAI_URL:-http://127.0.0.1:17373}"
KEY="${TRAMTAI_KEY:-<key from My APIs>}"
MODEL="${TRAMTAI_MODEL:-claude/haiku}"

AGENT="${MODEL%%/*}"
SUBMODEL=""
case "$MODEL" in */*) SUBMODEL=', "modelo": "'"${MODEL#*/}"'"' ;; esac
JOB=$(curl -sf "$URL/api/v1/trabajos" -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"agente": "'"$AGENT"'"'"$SUBMODEL"', "mensajes": [{"rol": "usuario", "contenido": "Hi, who are you?"}]}' | jq -r .id)
echo "job $JOB"
curl -sN "$URL/api/v1/trabajos/$JOB/eventos" -H "Authorization: Bearer $KEY"
curl -s "$URL/api/v1/trabajos/$JOB/salida" -H "Authorization: Bearer $KEY"
echo
