# The same tool loop through the native API: POST /api/v1/ejecutar waits for the response and brings "llamadas".
# The field names (agente, mensajes, rol, contenido, herramientas…) are the API's own and stay as they are.
import os

import requests

URL = os.environ.get("TRAMTAI_URL", "http://127.0.0.1:17373")
KEY = os.environ.get("TRAMTAI_KEY", "<key from My APIs>")
MODEL = os.environ.get("TRAMTAI_MODEL", "claude/haiku")

agent, _, model = MODEL.partition("/")
headers = {"Authorization": f"Bearer {KEY}"}
tools = [{
    "nombre": "get_weather",
    "descripcion": "Current weather for a city",
    "parametros": {"type": "object", "properties": {"city": {"type": "string"}}, "required": ["city"]},
}]


def get_weather(city):
    """Your app's function: here you would call a real service."""
    return f"18 °C and clear in {city}"


messages = [{"rol": "usuario", "contenido": "What's the weather like in Santiago?"}]
for _ in range(5):  # a cap: if the agent keeps asking for tools, quota isn't spent forever
    request = {"agente": agent, "mensajes": messages, "herramientas": tools}
    if model:
        request["modelo"] = model
    job = requests.post(f"{URL}/api/v1/ejecutar", headers=headers, json=request, timeout=600).json()
    if job.get("estado") != "completado":
        raise SystemExit(f"the job ended as {job.get('estado')}: {job.get('mensaje_error') or job}")
    calls = job.get("llamadas", [])
    if not calls:
        print(job["salida"])
        break
    messages.append({"rol": "asistente", "contenido": job["salida"], "llamadas": calls})
    for call in calls:
        print(f"The agent asks for {call['nombre']}({call['argumentos']})")
        messages.append({"rol": "herramienta", "id_llamada": call["id"], "contenido": get_weather(**call["argumentos"])})
else:
    raise SystemExit("the agent kept asking for tools after 5 rounds")
