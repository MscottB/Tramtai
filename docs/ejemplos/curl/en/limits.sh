#!/bin/sh
# A request's limits and the accepted file formats, so your app knows them before sending.
URL="${TRAMTAI_URL:-http://127.0.0.1:17373}"
KEY="${TRAMTAI_KEY:-<key from My APIs>}"
MODEL="${TRAMTAI_MODEL:-claude/haiku}"

curl -s "$URL/api/v1/limites" -H "Authorization: Bearer $KEY"
echo
