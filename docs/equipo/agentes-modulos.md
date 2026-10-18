# Agentes por módulos y listas que crecen (v0.43)

Diseño acordado el 2026-10-18. El usuario quiere añadir agentes (Copilot CLI, OpenCode, Hermes Agent) sin que el código
se desordene: hoy un agente nuevo toca unos 15 sitios. Además, Mis APIs y Agentes deben seguir siendo cómodos cuando
haya muchas APIs y muchos agentes.

Decisiones del usuario:
- **Un agente por release:** v0.44 Copilot CLI, v0.45 OpenCode, v0.46 Hermes Agent (al final; antes necesita
  `hermes setup`).
- **Antes, este release:** cada agente actual en su módulo, una forma documentada de añadir uno, Mis APIs en pestañas
  con filtro y páginas, y Agentes ordenado para crecer.

La evidencia M0 de los tres agentes nuevos está en `m0-agentes.md`.

Fuera de alcance: añadir agentes nuevos y cambiar cómo ejecuta cada agente actual (mismas órdenes, mismos canarios).

## 1. Un módulo por agente

Hoy los nombres `claude`, `codex`, `gemini` y `antigravity` están repetidos en Go:

- `adaptadores/registro.go`, `esfuerzo.go`, `comun.go` (secretos);
- `api/nativa.go` (capacidades);
- `nucleo/configuracion.go` (comandos y concurrencia);
- `servicio/web.go` (suscripción), `servicio/puntos.go` y `planificador.go` (Antigravity en apps);
- `cmd/tramtai/servicio.go`.

Y en la Web UI:

- `api.ts` (nombres, códigos, orden);
- `estilos.css` (colores);
- `Agentes.vue`, `EditorReceta.vue`, `MapaRed.vue`, `Accesos.vue` y `Ejecutar.vue`.

**Nueva estructura:**
- `internal/adaptadores` queda como el contrato (`Adaptador`, `Evento`, `Resultado`, `Detectable`, `Resolvedor`,
  `Contextual`) y lo común (lanzar el proceso, leer JSONL, redactar secretos, entorno sin secretos).
- `internal/agentes/<nombre>/` (paquetes `claude`, `codex`, `gemini`, `antigravity`) tiene el adaptador de cada uno y su
  **ficha** (`agentes.Ficha`):
  - **Identidad:**
    - `Nombre` (id, `claude`);
    - `NombreVisible` («Claude»);
    - `Codigo` (`CL`, el de su línea);
    - `Color` (hex claro y oscuro);
    - `Orden`.
  - **Capacidades:**
    - streaming (`tokens` o `final`);
    - salida estructurada;
    - modelo efectivo informado.
  - **Esfuerzo:** niveles y valor del CLI para cada uno; predeterminado por modelo (lo que hoy es `esfuerzo.go`).
  - **Autenticación:**
    - modos admitidos (`suscripcion`, `clave_api`, solo uno o ninguno);
    - variables de su clave (lo que hoy reparte `Secretos()`);
    - orden para guardarla.
  - **Ejecución:**
    - comando predeterminado (`comandos_agentes.<nombre>`);
    - concurrencia predeterminada;
    - `PermitidoEnApps` (su canario pasó; Antigravity, no).
  - **Instalación:** texto y enlace para Agentes.
  - **`Crear(c *nucleo.Configuracion) adaptadores.Adaptador`**.
- `internal/agentes` (catálogo) lista los módulos en un solo sitio (`Todos()`), construye el registro
  (`Registro(c)`, que sustituye a `adaptadores.Registro`) y responde a las preguntas que hoy están repetidas:
  - qué niveles admite;
  - si admite suscripción;
  - si se permite en apps;
  - sus secretos;
  - sus valores predeterminados de configuración.
- **Fuera de `internal/agentes`, ningún archivo Go nombra un agente.** Lo comprueba una prueba que busca los nombres
  del catálogo en el resto del código: los textos de ayuda y las pruebas quedan exceptuados.
  - Excepción: `predeterminados.agente: claude` se queda como valor de configuración.

**Web UI:**
- Nueva ruta rápida `GET /api/v1/agentes`, sin comprobar los CLI. Devuelve las fichas: id, nombre, código, colores,
  orden, niveles de esfuerzo, modos de autenticación, permitido en apps e instalación.
- `api.ts` la carga una vez por sesión, y de ella salen:
  - `nombreAgente`, `codigoLinea` y `ordenAgentes`;
  - los colores de línea (variable `--c` en línea, en lugar de una clase CSS por agente);
  - lo que hoy se decide por nombre en Agentes, EditorReceta, MapaRed, Accesos y Probar.
- Un agente desconocido (servicio más nuevo que la página) se ve con su id y un color neutro.

**Cómo añadir un agente:** se documenta en `docs/equipo/nuevo-agente.md` como una lista que cada release de agente
sigue:
1. M0 con el CLI real: salida no interactiva, uso, modelo efectivo, esfuerzo, autenticación, carpeta privada,
   persistencia y canario.
2. Módulo `internal/agentes/<nombre>` con su adaptador, su ficha y sus pruebas:
   - con un CLI simulado que emite el JSONL capturado en la M0;
   - el caso en `pruebas/canario`.
3. Una línea en el catálogo.
4. Textos es/en de lo que diga su ficha.
5. Documentación: Ayuda, README, `site/` y `docs/`.

**Pruebas:**
- `TestFichasCompletas`: cada ficha tiene todo; el código, el orden y los colores son únicos.
- La prueba de nombres fuera del catálogo.
- Todas las pruebas actuales siguen pasando sin cambiar lo que comprueban; solo cambian las rutas de los paquetes.
- Los canarios con Claude (haiku) y Codex (`gpt-5.6-terra`, esfuerzo bajo) se repiten tras mover los módulos.

## 2. Mis APIs con pestañas, filtro y páginas

- Dos pestañas, que se recuerdan en el navegador:
  - **«APIs por aplicación (n)»**;
  - **«API principal»**, con la guía de URL, la clave, los modelos, los ejemplos y el presupuesto del espacio.
- En «APIs por aplicación»:
  - **Filtros:** los de ahora (buscador, grupo y orden), más estado (activas, detenidas, con error) y agente.
  - **Páginas:** de 20 APIs, con «Mostrando 21–40 de 57»; los grupos se ven dentro de cada página.
  - **Reinicio:** al cambiar un filtro, se vuelve a la primera página.
  - **Con pocas APIs:** si caben en una página, no se ve la paginación; si hay menos de 2, tampoco los filtros (como
    ahora).
- Llegar desde Inicio con un grupo elegido abre esa pestaña con el filtro puesto.

## 3. Agentes ordenado para crecer

- **Arriba:**
  - un buscador;
  - un filtro por estado (Listos, Falta conectar, No instalados), con su número;
  - el botón de volver a comprobar.
- **Orden:** primero los listos, luego los que falta conectar, en el orden del catálogo.
- **Filas compactas:** cada agente es una fila (chapa, nombre, versión, estado, esfuerzo y contexto en una línea) que
  se despliega para ver cómo conectarlo y sus detalles. Sustituye a las tarjetas grandes en dos columnas; se despliega
  uno a la vez.
- **No instalados:** van al final, plegados en «Otros agentes que puedes instalar», con la instalación de su ficha.

## Documentación

- `docs/equipo/nuevo-agente.md` (la lista del §1).
- `docs/arquitectura.md`: los módulos de agentes y el catálogo.
- `docs/api.md`: `GET /api/v1/agentes`; también se añade a `openapi.json`.
- Ayuda:
  - nueva pregunta «¿Qué agentes puedo usar?», con los que hay y cómo se conectan;
  - las guías y conceptos que mencionan Agentes o Mis APIs se ajustan a las pestañas y filas nuevas.
- `site/`: capturas de Mis APIs y Agentes.
- `docs/equipo/pendientes.md`: la fila de v0.43 y las de Copilot, OpenCode y Hermes con su versión prevista.

## Pruebas

- Go:
  - las del catálogo (§1);
  - `GET /api/v1/agentes` con y sin sesión;
  - la cobertura de traducciones;
  - `go vet` en Windows, Linux y macOS.
- Canarios: Claude y Codex.
- Navegador, en móvil y escritorio, claro y oscuro:
  - pestañas, filtros y páginas de Mis APIs con 60 APIs de prueba;
  - Agentes con los cuatro agentes y un agente simulado «no instalado»;
  - colores y nombres saliendo de las fichas.
