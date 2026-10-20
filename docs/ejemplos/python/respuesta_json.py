# Una respuesta en JSON que cumple un esquema: Tramtai la valida antes de devolverla.
import json
import os

from openai import OpenAI

URL = os.environ.get("TRAMTAI_URL", "http://127.0.0.1:17373")
CLAVE = os.environ.get("TRAMTAI_CLAVE", "<clave de Mis APIs>")
MODELO = os.environ.get("TRAMTAI_MODELO", "claude/haiku")

esquema = {
    "type": "object",
    "properties": {"resumen": {"type": "string"}},
    "required": ["resumen"],
    "additionalProperties": False,
}

cliente = OpenAI(base_url=f"{URL}/openai/v1", api_key=CLAVE)
respuesta = cliente.chat.completions.create(
    model=MODELO,
    messages=[{"role": "user", "content": "Resume en una frase qué es Tramtai."}],
    response_format={"type": "json_schema", "json_schema": {"name": "resumen", "schema": esquema}},
)
datos = json.loads(respuesta.choices[0].message.content)
print("resumen:", datos["resumen"])
