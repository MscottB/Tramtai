# Presupuestos por API y por instancia

Diseño acordado el 2026-10-16. Objetivo: que el dueño de una API (o de la instancia entera) limite cuánto
consume la aplicación que la llama, por día y por mes, con aviso al 80 % y corte al 100 %, para proteger la
cuota de su suscripción o de sus claves.

Es el primero de cuatro ciclos (presupuestos, webhooks, comparar agentes, servidor MCP). Fuera de alcance aquí:
el envío de webhooks (este ciclo solo deja registrados los cruces del 80 % y del 100 % que el siguiente enviará).

## 1. Unidades y periodos

- **Peticiones**: se cuentan al aceptar el trabajo (exactas).
- **Tokens**: entrada + salida que informa el agente al terminar. Los cuatro adaptadores los informan.
- **USD**: el coste que informa el agente. Hoy solo Claude (`total_cost_usd`; con suscripción es estimado). Los
  trabajos de otros agentes suman 0. La Web UI lo dice junto al campo.
- **Periodos**: día y mes de calendario en la hora local del equipo. Cada límite es opcional; 0 o ausente = sin
  límite.

## 2. Configuración

### Por API (`ConfigPunto`, internal/servicio/puntos.go)

```yaml
presupuesto:
  diario:  { peticiones: 500 }
  mensual: { tokens: 20000000, usd: 50 }
```

`ConfigPunto.Presupuesto *Presupuesto` con `Diario, Mensual Limites` y `Limites{Peticiones, Tokens int; USD
float64}`. `Normalizar` rechaza valores negativos. `api.Punto` copia el presupuesto. La CLI lo acepta en crear
y editar con `--presupuesto diario.peticiones=500 --presupuesto mensual.usd=50` (repetible; valor 0 lo quita).

### Por instancia

Claves nuevas de configuración: `presupuesto.diario.{peticiones,tokens,usd}` y
`presupuesto.mensual.{peticiones,tokens,usd}`. `usd` necesita un tipo decimal nuevo (`tipoDecimal`) en
internal/nucleo/configuracion.go. `presupuesto` entra en `prefijosAdministrados`: es un límite de seguridad y la
configuración versionada de un repositorio no puede subirlo. El presupuesto de la instancia cuenta **todos** sus
trabajos: los de sus API, los de la API nativa con la clave de la app y los de Probar.

## 3. Registro del consumo

Tabla nueva (migración del almacén):

```sql
CREATE TABLE consumo (
  instancia TEXT, punto TEXT, dia TEXT,          -- dia = AAAA-MM-DD local; punto '' = sin API
  peticiones INTEGER, terminados INTEGER, tokens INTEGER, usd REAL,
  PRIMARY KEY (instancia, punto, dia));
CREATE TABLE avisos_presupuesto (
  instancia TEXT, punto TEXT, periodo TEXT,      -- periodo = D:AAAA-MM-DD o M:AAAA-MM
  nivel INTEGER, en TEXT,                        -- nivel = 80 o 100
  PRIMARY KEY (instancia, punto, periodo, nivel));
```

- **Al aceptar** (`Enviar`/`EnviarPorPunto`, en la misma transacción que crea el trabajo): `peticiones + 1`.
- **Al terminar** (cualquier estado terminal, también con error o cancelación): `terminados + 1`, `tokens +=`,
  `usd +=`. Se suma en la fila del **día de aceptación** del trabajo (`CreadoEn`), no del día en que termina:
  así `peticiones − terminados` de una fila es siempre lo que sigue en curso.
- La retención de trabajos no toca estas tablas. La purga existente borra `consumo` y `avisos_presupuesto` con
  más de 13 meses.
- El consumo de un periodo es la suma de sus filas: día = una fila; mes = filas del mes; instancia = todas las
  filas de la instancia (con y sin API).

## 4. Comprobación al aceptar

Antes de crear el trabajo se evalúan el presupuesto de la API (si la petición llega por una) y el de la
instancia, cada uno en su periodo diario y mensual:

- **Peticiones**: `peticiones ≥ límite` → rechazo.
- **Tokens y USD, con reserva por adelantado**: `consumido + en_curso × promedio ≥ límite` → rechazo, donde
  `en_curso = peticiones − terminados` y `promedio = consumido / terminados` del mismo periodo (0 si aún no
  terminó ninguno: sin historial no se reserva). Así varios trabajos lanzados a la vez cerca del límite no lo
  pasan de largo. Sigue siendo un límite aproximado: un trabajo puede gastar más que el promedio; se documenta.

El rechazo es `presupuesto_agotado` (código nuevo en `nucleo`): HTTP 429 en las tres fachadas, con su formato de
error, `Retry-After` en segundos hasta el próximo reinicio del periodo agotado, y un mensaje que nombra el límite
("presupuesto mensual de tokens de la API facturas agotado"). No es saltable: el respaldo no lo evita, porque el
límite es de la API y no del agente.

Si falla la lectura o la escritura de `consumo`, el trabajo **no** se rechaza (se registra en el log): un fallo
del contador no debe tumbar la API.

## 5. Avisos

- **Cabecera** en cada respuesta de una API o instancia con presupuesto: `X-Tramtai-Presupuesto: <n>`, el
  porcentaje usado (entero) del límite más cercano a agotarse entre los de la API y los de la instancia.
- **Registro de cruces**: tras cada suma, si un límite pasa del 80 % o del 100 % en un periodo, se inserta su
  fila en `avisos_presupuesto` (la clave primaria evita repetirlo). El ciclo de webhooks enviará
  `presupuesto.aviso` y `presupuesto.agotado` desde aquí.
- **Web UI (Accesos)**: cada API con presupuesto muestra una barra por límite (amarilla desde el 80 %, roja al
  100 %) con el reinicio del periodo. Arriba, el presupuesto de la instancia con sus barras. El editor de la API
  gana los campos de presupuesto.

## 6. Reinicio manual

Borra las filas de `consumo` y `avisos_presupuesto` del día y del mes en curso del ámbito indicado (reiniciar el
mes reinicia también el día, porque comparten filas):

- API: `POST /api/v1/puntos/<nombre>/presupuesto/reiniciar`; botón "Reiniciar consumo" en Accesos, con
  confirmación; CLI `tramtai acceso reiniciar-presupuesto <nombre>`.
- Instancia: `POST /api/v1/instancia/presupuesto/reiniciar` (borra las filas de la instancia, con y sin API);
  botón en el bloque de la instancia; CLI `tramtai acceso reiniciar-presupuesto` sin nombre.

Mismo permiso que regenerar la clave de una API (sesión local de administración).

## 7. Lectura

`GET /api/v1/puntos` añade en `uso.<nombre>.presupuesto` el consumo y el límite de cada periodo. `GET
/api/v1/instancia/presupuesto` devuelve lo mismo para la instancia. Eliminar una API borra sus filas de consumo.

## 8. Pruebas

- Almacén: sumas al aceptar y al terminar; atribución al día de aceptación con un trabajo que cruza la
  medianoche; suma mensual; reinicio; purga de 13 meses; reloj inyectado.
- Comprobación: peticiones exactas; reserva por adelantado (dos trabajos en curso con promedio conocido se
  rechaza el tercero); sin historial no reserva; presupuesto de instancia que incluye trabajos sin API.
- API: 429 `presupuesto_agotado` en nativa, OpenAI y Anthropic con `Retry-After`; cabecera presente; el
  respaldo no se activa.
- Configuración: valores negativos rechazados; `presupuesto` administrado (rechazado en la configuración del
  repositorio); `tipoDecimal`.
- i18n: textos nuevos traducidos en internal/idioma/en_*.go y web/src/idiomas/en.ts.

## 9. Documentación

docs/api.md (cabecera, 429, rutas de reinicio y lectura), docs/operacion.md (configuración y límite
aproximado), Ayuda de la Web UI y la página del sitio (presupuestos como función de equipo).
