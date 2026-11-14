# Tool calling: your app offers functions, the agent asks for one, your app runs it and sends back the result.
import json
import os

from openai import OpenAI

URL = os.environ.get("TRAMTAI_URL", "http://127.0.0.1:17373")
KEY = os.environ.get("TRAMTAI_KEY", "<key from My APIs>")
MODEL = os.environ.get("TRAMTAI_MODEL", "claude/haiku")

tools = [{"type": "function", "function": {
    "name": "get_weather",
    "description": "Current weather for a city",
    "parameters": {"type": "object", "properties": {"city": {"type": "string"}}, "required": ["city"]},
}}]


def get_weather(city):
    """Your app's function: here you would call a real service."""
    return f"18 °C and clear in {city}"


client = OpenAI(base_url=f"{URL}/openai/v1", api_key=KEY)
messages = [{"role": "user", "content": "What's the weather like in Santiago?"}]
for _ in range(5):  # a cap: if the agent keeps asking for tools, quota isn't spent forever
    message = client.chat.completions.create(model=MODEL, messages=messages, tools=tools).choices[0].message
    if not message.tool_calls:
        print(message.content)
        break
    # The assistant's turn with its calls, and one result for each.
    messages.append({"role": "assistant", "content": message.content, "tool_calls": [
        {"id": c.id, "type": "function", "function": {"name": c.function.name, "arguments": c.function.arguments}}
        for c in message.tool_calls
    ]})
    for call in message.tool_calls:
        arguments = json.loads(call.function.arguments)
        print(f"The agent asks for {call.function.name}({arguments})")
        messages.append({"role": "tool", "tool_call_id": call.id, "content": get_weather(**arguments)})
else:
    raise SystemExit("the agent kept asking for tools after 5 rounds")
