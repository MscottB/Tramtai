# El mismo ciclo de herramientas por la API propia: POST /api/v1/ejecutar espera la respuesta y trae «llamadas».
import os

import requests

URL = os.environ.get("TRAMTAI_URL", "http://127.0.0.1:17373")
CLAVE = os.environ.get("TRAMTAI_CLAVE", "<clave de Mis APIs>")
MODELO = os.environ.get("TRAMTAI_MODELO", "claude/haiku")

agente, _, modelo = MODELO.partition("/")
cabeceras = {"Authorization": f"Bearer {CLAVE}"}
herramientas = [{
    "nombre": "clima",
    "descripcion": "El tiempo actual de una ciudad",
    "parametros": {"type": "object", "properties": {"ciudad": {"type": "string"}}, "required": ["ciudad"]},
}]


def clima(ciudad):
    """La función de tu app: aquí consultarías un servicio real."""
    return f"18 °C y despejado en {ciudad}"


mensajes = [{"rol": "usuario", "contenido": "¿Qué tiempo hace en Santiago?"}]
while True:
    solicitud = {"agente": agente, "mensajes": mensajes, "herramientas": herramientas}
    if modelo:
        solicitud["modelo"] = modelo
    trabajo = requests.post(f"{URL}/api/v1/ejecutar", headers=cabeceras, json=solicitud, timeout=600).json()
    if trabajo.get("estado") != "completado":
        raise SystemExit(f"el trabajo terminó en {trabajo.get('estado')}: {trabajo.get('mensaje_error') or trabajo}")
    llamadas = trabajo.get("llamadas", [])
    if not llamadas:
        print(trabajo["salida"])
        break
    mensajes.append({"rol": "asistente", "contenido": trabajo["salida"], "llamadas": llamadas})
    for llamada in llamadas:
        print(f"El agente pide {llamada['nombre']}({llamada['argumentos']})")
        mensajes.append({"rol": "herramienta", "id_llamada": llamada["id"], "contenido": clima(**llamada["argumentos"])})
