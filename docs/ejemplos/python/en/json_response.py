# A JSON response that follows a schema: Tramtai validates it before returning it.
import json
import os

from openai import OpenAI

URL = os.environ.get("TRAMTAI_URL", "http://127.0.0.1:17373")
KEY = os.environ.get("TRAMTAI_KEY", "<key from My APIs>")
MODEL = os.environ.get("TRAMTAI_MODEL", "claude/haiku")

schema = {
    "type": "object",
    "properties": {"summary": {"type": "string"}},
    "required": ["summary"],
    "additionalProperties": False,
}

client = OpenAI(base_url=f"{URL}/openai/v1", api_key=KEY)
response = client.chat.completions.create(
    model=MODEL,
    messages=[{"role": "user", "content": "Summarize in one sentence what Tramtai is."}],
    response_format={"type": "json_schema", "json_schema": {"name": "summary", "schema": schema}},
)
data = json.loads(response.choices[0].message.content)
print("summary:", data["summary"])
