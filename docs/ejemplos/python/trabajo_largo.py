# Un trabajo largo por la API propia: se crea, se sigue en vivo (y se puede retomar con Last-Event-ID) y se lee el
# resultado. Cancelar: POST /api/v1/trabajos/{id}/cancelar; reintentar: POST /api/v1/trabajos/{id}/reintentar.
import json
import os

import requests

URL = os.environ.get("TRAMTAI_URL", "http://127.0.0.1:17373")
CLAVE = os.environ.get("TRAMTAI_CLAVE", "<clave de Mis APIs>")
MODELO = os.environ.get("TRAMTAI_MODELO", "claude/haiku")

agente, _, modelo = MODELO.partition("/")
cabeceras = {"Authorization": f"Bearer {CLAVE}"}
solicitud = {"agente": agente, "mensajes": [{"rol": "usuario", "contenido": "Hola, ¿quién eres?"}]}
if modelo:
    solicitud["modelo"] = modelo

trabajo = requests.post(f"{URL}/api/v1/trabajos", headers=cabeceras, json=solicitud, timeout=60).json()
print("trabajo", trabajo["id"])

# Los eventos llegan por SSE hasta «fin»; si se corta, vuelve a pedirlos con Last-Event-ID: <último id>.
evento = ""
with requests.get(f"{URL}/api/v1/trabajos/{trabajo['id']}/eventos", headers=cabeceras, stream=True, timeout=600) as r:
    for linea in r.iter_lines(decode_unicode=True):
        if linea.startswith("event: "):
            evento = linea[7:]
        elif linea.startswith("data: ") and evento == "fin":
            print("fin:", json.loads(linea[6:])["estado"])
            break

salida = requests.get(f"{URL}/api/v1/trabajos/{trabajo['id']}/salida", headers=cabeceras, timeout=60)
print(salida.text)
