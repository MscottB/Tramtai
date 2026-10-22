# 001. Lo derivado lo calcula el servicio, y un agente sin CLI es «no instalado»

Fecha: 2026-10-22. Estado: aceptada.

## Contexto

La Web UI y la terminal calculaban cada una lo que se deduce de los datos del servicio: el estado de cada agente, la
etapa de una petición, si se puede corregir, los Primeros pasos de Inicio y cuándo sugerir una API propia. Cada regla
vivía en dos lenguajes y ya no coincidían: para un agente desconectado cuyo CLI ya no está, la terminal decía «no
instalado» y la Web UI «desconectado» (auditoría 001, hallazgos 92 y 100).

## Decisión

1. **El servicio calcula lo derivado y lo manda en sus respuestas.** La Web UI y la terminal lo pintan y no repiten la
   regla:
   - `estado` de cada agente en `GET /api/v1/capacidades` (`api.estadoAgente`);
   - `etapa` y `corregible` de cada trabajo (`nucleo.Estado.Etapa`, `nucleo.Trabajo.SePuedeCorregir`);
   - los Primeros pasos en `GET /api/v1/instancia/pasos`;
   - `sugerir_api_propia` en `GET /api/v1/instancia`.
2. **Un agente desconectado cuyo CLI ya no está es `no_instalado`.** Es lo accionable: antes de conectarlo hay que
   instalarlo, y conectarlo no serviría de nada. La regla de `api.estadoAgente` mira primero si falta el CLI y después
   si está desconectado.

## Consecuencias

- Una regla nueva o un estado nuevo se cambia en un solo sitio, en Go, con su prueba en el servicio.
- La terminal necesita un servicio de la misma versión: no es un problema, porque van en el mismo ejecutable.
