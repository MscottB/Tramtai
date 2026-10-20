#!/bin/sh
# Un trabajo largo por la API propia: se crea, se sigue en vivo hasta «fin» y se lee el resultado. Si la conexión se
# corta, se retoma con -H "Last-Event-ID: <último id>". Necesita jq.
set -e
URL="${TRAMTAI_URL:-http://127.0.0.1:17373}"
CLAVE="${TRAMTAI_CLAVE:-<clave de Mis APIs>}"
MODELO="${TRAMTAI_MODELO:-claude/haiku}"

AGENTE="${MODELO%%/*}"
TRABAJO=$(curl -sf "$URL/api/v1/trabajos" -H "Authorization: Bearer $CLAVE" -H "Content-Type: application/json" \
  -d '{"agente": "'"$AGENTE"'", "mensajes": [{"rol": "usuario", "contenido": "Hola, ¿quién eres?"}]}' | jq -r .id)
echo "trabajo $TRABAJO"
curl -sN "$URL/api/v1/trabajos/$TRABAJO/eventos" -H "Authorization: Bearer $CLAVE"
curl -s "$URL/api/v1/trabajos/$TRABAJO/salida" -H "Authorization: Bearer $CLAVE"
echo
