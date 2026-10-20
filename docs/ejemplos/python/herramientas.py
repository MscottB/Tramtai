# Herramientas (tools): tu app ofrece funciones, el agente pide una, tu app la ejecuta y le devuelve el resultado.
import json
import os

from openai import OpenAI

URL = os.environ.get("TRAMTAI_URL", "http://127.0.0.1:17373")
CLAVE = os.environ.get("TRAMTAI_CLAVE", "<clave de Mis APIs>")
MODELO = os.environ.get("TRAMTAI_MODELO", "claude/haiku")

herramientas = [{"type": "function", "function": {
    "name": "clima",
    "description": "El tiempo actual de una ciudad",
    "parameters": {"type": "object", "properties": {"ciudad": {"type": "string"}}, "required": ["ciudad"]},
}}]


def clima(ciudad):
    """La función de tu app: aquí consultarías un servicio real."""
    return f"18 °C y despejado en {ciudad}"


cliente = OpenAI(base_url=f"{URL}/openai/v1", api_key=CLAVE)
mensajes = [{"role": "user", "content": "¿Qué tiempo hace en Santiago?"}]
while True:
    mensaje = cliente.chat.completions.create(model=MODELO, messages=mensajes, tools=herramientas).choices[0].message
    if not mensaje.tool_calls:
        print(mensaje.content)
        break
    # El turno del asistente con sus llamadas, y un resultado por cada una.
    mensajes.append({"role": "assistant", "content": mensaje.content, "tool_calls": [
        {"id": l.id, "type": "function", "function": {"name": l.function.name, "arguments": l.function.arguments}}
        for l in mensaje.tool_calls
    ]})
    for llamada in mensaje.tool_calls:
        argumentos = json.loads(llamada.function.arguments)
        print(f"El agente pide {llamada.function.name}({argumentos})")
        mensajes.append({"role": "tool", "tool_call_id": llamada.id, "content": clima(**argumentos)})
