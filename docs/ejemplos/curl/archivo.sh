#!/bin/sh
# Un archivo por la API propia: se sube a /cargas y el trabajo lo cita por su identificador. Necesita jq.
set -e
URL="${TRAMTAI_URL:-http://127.0.0.1:17373}"
CLAVE="${TRAMTAI_CLAVE:-<clave de Mis APIs>}"
MODELO="${TRAMTAI_MODELO:-claude/haiku}"

CARPETA="$(dirname "$0")"
AGENTE="${MODELO%%/*}"
SUBMODELO=""
case "$MODELO" in */*) SUBMODELO=', "modelo": "'"${MODELO#*/}"'"' ;; esac
CARGA=$(curl -sf "$URL/api/v1/cargas" -H "Authorization: Bearer $CLAVE" -F "archivo=@$CARPETA/../muestra.txt" | jq -r .identificador_carga)
curl -sf "$URL/api/v1/ejecutar" -H "Authorization: Bearer $CLAVE" -H "Content-Type: application/json" \
  -d '{"agente": "'"$AGENTE"'"'"$SUBMODELO"', "cargas": ["'"$CARGA"'"], "mensajes": [{"rol": "usuario", "contenido": "¿Qué dice este archivo?"}]}' \
  | jq -r .salida
