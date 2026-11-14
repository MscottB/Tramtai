# Your first message through the OpenAI-compatible URL, with the response streamed live.
import os

from openai import OpenAI

URL = os.environ.get("TRAMTAI_URL", "http://127.0.0.1:17373")
KEY = os.environ.get("TRAMTAI_KEY", "<key from My APIs>")
MODEL = os.environ.get("TRAMTAI_MODEL", "claude/haiku")

client = OpenAI(base_url=f"{URL}/openai/v1", api_key=KEY)
stream = client.chat.completions.create(
    model=MODEL, messages=[{"role": "user", "content": "Hi, who are you?"}], stream=True
)
for chunk in stream:
    if chunk.choices and chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="", flush=True)
print()
