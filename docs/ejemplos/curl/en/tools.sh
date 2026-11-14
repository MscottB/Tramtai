#!/bin/sh
# Tool calling through the OpenAI-compatible URL: the agent asks for get_weather, it is "run" here and the result goes
# back in a second turn. Needs jq.
set -e
URL="${TRAMTAI_URL:-http://127.0.0.1:17373}"
KEY="${TRAMTAI_KEY:-<key from My APIs>}"
MODEL="${TRAMTAI_MODEL:-claude/haiku}"

TOOLS='[{"type": "function", "function": {"name": "get_weather", "description": "Current weather for a city",
  "parameters": {"type": "object", "properties": {"city": {"type": "string"}}, "required": ["city"]}}}]'
QUESTION='{"role": "user", "content": "What is the weather like in Santiago?"}'

ask() {
  curl -sf "$URL/openai/v1/chat/completions" -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
    -d '{"model": "'"$MODEL"'", "tools": '"$TOOLS"', "messages": '"$1"'}'
}

MESSAGE=$(ask "[$QUESTION]" | jq -c '.choices[0].message')
CALL=$(echo "$MESSAGE" | jq -c '.tool_calls[0] // empty')
if [ -z "$CALL" ]; then
  echo "$MESSAGE" | jq -r .content # it answered without asking for the tool
  exit 0
fi
echo "The agent asks for $(echo "$CALL" | jq -r '.function.name + .function.arguments')"
RESULT=$(jq -cn --arg id "$(echo "$CALL" | jq -r .id)" '{role: "tool", tool_call_id: $id, content: "18 °C and clear in Santiago"}')
ask "[$QUESTION, $MESSAGE, $RESULT]" | jq -r '.choices[0].message.content'
