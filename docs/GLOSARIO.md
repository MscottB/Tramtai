# Glosario

Los nombres que ve la persona en la Web UI, la terminal, la CLI, la Ayuda, el sitio y los documentos. Cada concepto
tiene un término en español y otro en inglés; los de la columna «No se usa» no aparecen en textos de usuario. La
prueba `web/scripts/estilo-textos.test.ts` falla si vuelven.

El código, las rutas y los campos JSON de la API, las claves de configuración y los nombres de los comandos no se
renombran: son nombres técnicos. Cuando un texto de usuario los cita, van en formato de código y, la primera vez, con
el término del glosario al lado.

## Conceptos

| Español | Inglés | No se usa | Uso |
|---|---|---|---|
| espacio (de trabajo) | workspace | instancia, instance | Donde se procesan y guardan las peticiones, las APIs y los ajustes: el global o el de un proyecto. En la API propia es la ruta `/api/v1/instancia` y la cabecera `Tramtai-Instancia`. |
| API (por aplicación) | API (per-app API) | punto de acceso, acceso, access point | Dirección, clave y receta propias de una app. Los nombres técnicos son `tramtai acceso` y `puntos_acceso`. |
| clave | key (API key) | token, contraseña | Lo que la app envía en `Authorization: Bearer <clave>`. «Token» solo queda en la documentación técnica de esa cabecera y en el comando `tramtai acceso token`. |
| clave del espacio | workspace key | token del espacio, clave principal | La clave de un espacio, con acceso a todo: para ti, la CLI de Tramtai y tus scripts. |
| token (del modelo) | token | | Unidad con la que el modelo mide la entrada, la salida y el contexto. Nunca es la clave. |
| petición | request | trabajo, solicitud, job | Lo que se envía a un agente y se sigue en el Historial. En la API propia es el recurso `trabajos` (rutas y campos). |
| mensaje | message | prompt | El texto que escribe la persona dentro de una petición. |
| respuesta | response | answer (como sustantivo) | Lo que devuelve el agente. «Answer» solo como verbo. |
| registro | log | record | Los pasos de una petición y lo que procesa el servicio. |
| agente | agent | asistente, programa de IA | Claude Code, Codex, GitHub Copilot, la Gemini API y Antigravity. |
| el CLI del agente | the agent’s CLI | | El programa de línea de comandos de un agente («su CLI»). |
| la CLI de Tramtai | the Tramtai CLI | | Los comandos `tramtai …` en una terminal. Si en la frase aparecen los dos, cada uno lleva su dueño. |
| la Gemini API | the Gemini API | la API de Gemini | El nombre del producto de Google, tal como lo escribe Google. |
| receta | recipe | | Instrucciones, agente y modelo, esfuerzo, respaldo y forma de la respuesta de una API. |
| respaldo | fallback | | Los agentes que siguen si el principal falla antes de responder. |
| costo | cost | coste | Lo que el agente informa que costó una petición. Español neutro latinoamericano. |
| recorrido guiado | guided tour | bienvenida (como nombre del recorrido), tour (en español) | El paseo por la pantalla que se ofrece al abrir la app. |
| Tramtai es una API local | Tramtai is a local API | pasarela, puente, gateway, bridge | La descripción de una línea del producto. |

## Páginas

| Español | Inglés | No se usa | Uso |
|---|---|---|---|
| Probar | Playground | Try (como nombre de página) | La página donde se prueba un agente. «Try» solo como verbo. |
| Mis APIs | My APIs | Accesos | La página de las APIs y la clave del espacio. |
| Historial | History | Trabajos | Las peticiones de un espacio. |
| Usar (grupo del menú) | Use | Conectar (como grupo) | El grupo de Mis APIs e Historial. |

## Verbos

Un verbo por acción y una acción por verbo.

| Objeto | Español | Inglés | No se usa |
|---|---|---|---|
| API y espacio | Activar / Detener | Start / Stop | Desactivar, Iniciar, Turn on / Turn off, Enable / Disable |
| servicio | Iniciar / Detener / Reiniciar | Start / Stop / Restart | Arrancar (como botón) |
| interruptor de Ajustes | Activado / Desactivado | On / Off | |
| agente | Conectar / Desconectar | Connect / Disconnect | Conectar para cualquier otra cosa |
| petición en marcha | Cancelar la petición | Cancel the request | «Cancelar» a secas: es el botón de cerrar un diálogo |
| petición terminada | Reintentar, Corregir, Eliminar | Retry, Correct, Delete | |
| lista o página | Refrescar | Refresh | Actualizar |
| versión nueva de Tramtai | Instalar la versión nueva | Install the update | Actualizar (como botón) |
| cambios de una API desde Probar | Guardar cambios en {API} | Save changes to {API} | Actualizar {API} |
| diálogo | Cancelar | Cancel | |

## Escritura

- **Comillas.** En español, angulares «». En inglés, curvas: “ ” para citar y ’ para el apóstrofo; ni rectas ni
  angulares.
- **Inglés.** Variante de Estados Unidos: «canceled», «canceling», «color», «organize».
- **Teclas.** Enter (no «Intro»), Esc, Tab, Ctrl + Enter.
- **Rayas.** Sin raya larga (U+2014): punto, coma, dos puntos o paréntesis. Un valor que falta es «Sin dato» / «No data».
- **Errores.** Cada mensaje dice qué falló y qué hacer, sin códigos internos ni nombres de configuración que la
  persona no ve en pantalla.
- **Palabras huecas.** Fuera «potente», «inteligente», «revolucionario», «sin fisuras», «de nueva generación» y sus
  equivalentes en inglés (antislop R-16): se dice lo que hace.
