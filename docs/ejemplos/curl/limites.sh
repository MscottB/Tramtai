#!/bin/sh
# Los límites de una petición y los formatos de archivo admitidos, para que tu app los conozca antes de enviar.
URL="${TRAMTAI_URL:-http://127.0.0.1:17373}"
CLAVE="${TRAMTAI_CLAVE:-<clave de Mis APIs>}"
MODELO="${TRAMTAI_MODELO:-claude/haiku}"

curl -s "$URL/api/v1/limites" -H "Authorization: Bearer $CLAVE"
echo
