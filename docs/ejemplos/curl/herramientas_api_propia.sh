#!/bin/sh
# El mismo ciclo por la API propia: /ejecutar espera la respuesta y trae «llamadas» con los argumentos como objeto.
# Necesita jq.
set -e
URL="${TRAMTAI_URL:-http://127.0.0.1:17373}"
CLAVE="${TRAMTAI_CLAVE:-<clave de Mis APIs>}"
MODELO="${TRAMTAI_MODELO:-claude/haiku}"

AGENTE="${MODELO%%/*}"
HERRAMIENTAS='[{"nombre": "clima", "descripcion": "El tiempo actual de una ciudad",
  "parametros": {"type": "object", "properties": {"ciudad": {"type": "string"}}, "required": ["ciudad"]}}]'
PREGUNTA='{"rol": "usuario", "contenido": "¿Qué tiempo hace en Santiago?"}'

ejecutar() {
  curl -sf "$URL/api/v1/ejecutar" -H "Authorization: Bearer $CLAVE" -H "Content-Type: application/json" \
    -d '{"agente": "'"$AGENTE"'", "herramientas": '"$HERRAMIENTAS"', "mensajes": '"$1"'}'
}

TRABAJO=$(ejecutar "[$PREGUNTA]")
LLAMADAS=$(echo "$TRABAJO" | jq -c .llamadas)
echo "El agente pide $(echo "$LLAMADAS" | jq -c '.[0] | {nombre, argumentos}')"
ASISTENTE=$(jq -cn --argjson llamadas "$LLAMADAS" '{rol: "asistente", contenido: "", llamadas: $llamadas}')
RESULTADO=$(jq -cn --arg id "$(echo "$LLAMADAS" | jq -r '.[0].id')" '{rol: "herramienta", id_llamada: $id, contenido: "18 °C y despejado en Santiago"}')
ejecutar "[$PREGUNTA, $ASISTENTE, $RESULTADO]" | jq -r .salida
