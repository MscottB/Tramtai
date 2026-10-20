# Tu primer mensaje por la URL compatible con OpenAI, con la respuesta en vivo.
import os

from openai import OpenAI

URL = os.environ.get("TRAMTAI_URL", "http://127.0.0.1:17373")
CLAVE = os.environ.get("TRAMTAI_CLAVE", "<clave de Mis APIs>")
MODELO = os.environ.get("TRAMTAI_MODELO", "claude/haiku")

cliente = OpenAI(base_url=f"{URL}/openai/v1", api_key=CLAVE)
flujo = cliente.chat.completions.create(
    model=MODELO, messages=[{"role": "user", "content": "Hola, ¿quién eres?"}], stream=True
)
for trozo in flujo:
    if trozo.choices and trozo.choices[0].delta.content:
        print(trozo.choices[0].delta.content, end="", flush=True)
print()
