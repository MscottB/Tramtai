# A long job through the native API: create it, follow it live (resumable with Last-Event-ID) and read the result.
# Cancel: POST /api/v1/trabajos/{id}/cancelar; retry: POST /api/v1/trabajos/{id}/reintentar.
import json
import os

import requests

URL = os.environ.get("TRAMTAI_URL", "http://127.0.0.1:17373")
KEY = os.environ.get("TRAMTAI_KEY", "<key from My APIs>")
MODEL = os.environ.get("TRAMTAI_MODEL", "claude/haiku")

agent, _, model = MODEL.partition("/")
headers = {"Authorization": f"Bearer {KEY}"}
request = {"agente": agent, "mensajes": [{"rol": "usuario", "contenido": "Hi, who are you?"}]}
if model:
    request["modelo"] = model

job = requests.post(f"{URL}/api/v1/trabajos", headers=headers, json=request, timeout=60).json()
print("job", job["id"])

# Events arrive over SSE until "fin"; if the connection drops, ask again with Last-Event-ID: <last id>.
event = ""
with requests.get(f"{URL}/api/v1/trabajos/{job['id']}/eventos", headers=headers, stream=True, timeout=600) as r:
    for line in r.iter_lines(decode_unicode=True):
        if line.startswith("event: "):
            event = line[7:]
        elif line.startswith("data: ") and event == "fin":
            print("end:", json.loads(line[6:])["estado"])
            break

output = requests.get(f"{URL}/api/v1/trabajos/{job['id']}/salida", headers=headers, timeout=60)
print(output.text)
