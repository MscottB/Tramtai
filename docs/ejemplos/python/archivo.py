# Un archivo junto al mensaje: va en base64 dentro de la petición y Tramtai lo convierte a texto para el agente.
import base64
import os
import pathlib

from openai import OpenAI

URL = os.environ.get("TRAMTAI_URL", "http://127.0.0.1:17373")
CLAVE = os.environ.get("TRAMTAI_CLAVE", "<clave de Mis APIs>")
MODELO = os.environ.get("TRAMTAI_MODELO", "claude/haiku")

ruta = pathlib.Path(__file__).resolve().parent.parent / "muestra.txt"
datos = base64.b64encode(ruta.read_bytes()).decode()

cliente = OpenAI(base_url=f"{URL}/openai/v1", api_key=CLAVE)
respuesta = cliente.chat.completions.create(
    model=MODELO,
    messages=[{"role": "user", "content": [
        {"type": "text", "text": "¿Qué dice este archivo?"},
        {"type": "file", "file": {"filename": "muestra.txt", "file_data": f"data:text/plain;base64,{datos}"}},
    ]}],
)
print(respuesta.choices[0].message.content)
