# A file with the message: it travels as base64 inside the request and Tramtai turns it into text for the agent.
import base64
import os
import pathlib

from openai import OpenAI

URL = os.environ.get("TRAMTAI_URL", "http://127.0.0.1:17373")
KEY = os.environ.get("TRAMTAI_KEY", "<key from My APIs>")
MODEL = os.environ.get("TRAMTAI_MODEL", "claude/haiku")

path = pathlib.Path(__file__).resolve().parent.parent.parent / "sample.txt"
data = base64.b64encode(path.read_bytes()).decode()

client = OpenAI(base_url=f"{URL}/openai/v1", api_key=KEY)
response = client.chat.completions.create(
    model=MODEL,
    messages=[{"role": "user", "content": [
        {"type": "text", "text": "What does this file say?"},
        {"type": "file", "file": {"filename": "sample.txt", "file_data": f"data:text/plain;base64,{data}"}},
    ]}],
)
print(response.choices[0].message.content)
