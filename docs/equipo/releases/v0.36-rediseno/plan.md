# Rediseño: plan de implementación

> **Para agentes:** SUB-SKILL REQUERIDA: superpowers:executing-plans (o subagent-driven-development), tarea por
> tarea. Los pasos usan casillas (`- [ ]`).

**Objetivo:** pulir la Web UI y la página del producto dentro de la dirección «red de tranvía», con navegación
coherente, piezas comunes, páginas más legibles, esfuerzo predeterminado por modelo, actualización sin pantalla en
blanco y selector de tema en la página.

**Arquitectura:** primero las piezas compartidas (`CabeceraPagina`, `EstadoVacio`, `MenuAcciones`, estilo de
ayudas) y la navegación en `App.vue`; luego cada página las usa. El esfuerzo predeterminado sale de una tabla en
`internal/adaptadores/esfuerzo.go` al catálogo `/api/v1/modelos`. El fallo de actualización se depura antes de
corregirse.

**Stack:** Vue 3 + Quasar (vue-tsc), Go 1.27, Wails v3 (escritorio), HTML/CSS/JS estático (`site/`).

**Spec:** docs/equipo/releases/v0.36-rediseno/spec.md

## Restricciones globales

- Sin cambiar la dirección visual: colores, tipografías y metáforas de `docs/diseno/DESIGN.md`.
- Claro y oscuro con la misma calidad; sin desbordes a 375 px.
- Textos nuevos en `web/src/idiomas/es.ts` y `en.ts` (mismas claves); los de Go en `internal/idioma/en_*.go`.
- Accesibilidad: todo botón de icono con `aria-label`; foco visible; contraste del tema.
- Verificación de cada página en el navegador con el servicio aislado (puerto 17999, `APPDATA` propio; Claude y
  Codex con `autenticacion: suscripcion`; sin ejecutar peticiones reales a agentes).
- `cd web; npm run build` tras cada tarea de Web UI (deja `internal/recursos/web` al día).

## Foco de revisión

1. Volver tras recargar la página (pila vacía): no aparece, y el logo sigue llevando a Inicio. → Tarea 1.
2. `MenuAcciones` con teclado: abre con Enter, navega con flechas y cierra con Escape. → Tarea 2.
3. Mis APIs con una API sin presupuesto, desactivada o con error: la fila no se descoloca. → Tarea 3.
4. Esfuerzo: modelo sin dato conocido muestra solo «El del agente», nunca un valor supuesto. → Tarea 8.
5. Página del producto sin `localStorage` (navegación privada): el selector de tema funciona sin recordar. → Tarea 10.

---

### Tarea 1: Navegación y `CabeceraPagina`

**Archivos:** `web/src/App.vue`, crear `web/src/componentes/CabeceraPagina.vue`, `web/src/idiomas/{es,en}.ts`.

- `App.vue`: el logo, el nombre y la versión pasan a ser un botón (`aria-label` `app.irInicio`) que llama a
  `ir('inicio')`. Se quita el `q-btn` de `arrow_back` de la barra superior.
- `App.vue` hace `provide('navegacion', { anterior: computed(() => pila.at(-1) ? tituloDe(pila.at(-1)) : ''), volver })`.
- `CabeceraPagina.vue`: props `titulo: string`, `descripcion?: string`; slot `acciones`. Inyecta `navegacion`; si
  `anterior` no está vacío, muestra arriba un enlace `t('app.volverA', { pagina: anterior })` con icono flecha
  que llama a `volver()`. Acciones a la derecha del título (`row items-start`), debajo en `lt-sm`.
- [ ] Implementar y usarla en `Panel.vue` (Inicio) para comprobarla.
- [ ] `npm run build` sin errores; en el navegador: ir Inicio → Agentes → Probar, «Volver a Agentes» aparece y
  retrocede un paso; el logo lleva a Inicio; tras recargar no hay Volver.
- [ ] Commit `feat(web): el logo lleva a Inicio y Volver pasa a la cabecera de cada página`.

### Tarea 2: `EstadoVacio`, `MenuAcciones` y ayudas legibles

**Archivos:** crear `web/src/componentes/EstadoVacio.vue` y `MenuAcciones.vue`; `web/src/estilos.css`.

- `EstadoVacio.vue`: props `titulo`, `texto?`, `accion?` (texto del botón); emite `accion`. Dibujo: SVG pequeño
  con tramo punteado (`stroke-dasharray: 0 9`, `stroke-linecap: round`, como `MapaRed`) y estación con aro, en
  `var(--borde-fuerte)`.
- `MenuAcciones.vue`: prop `acciones: { etiqueta: string; icono: string; peligro?: boolean; oculto?: boolean }[]`;
  emite `elegir(indice)`. `q-btn` flat round `icon="more_horiz"` `aria-label` `comun.masAcciones` con `q-menu` y
  `q-list` (icono + texto; las de `peligro` al final con `text-negative`).
- `estilos.css`: `.q-field__bottom, .ayuda-campo { font-size: 0.78rem; line-height: 1.45; }`; la clase
  `ayuda-campo` sustituye a `text-caption texto-suave` en las ayudas bajo un campo.
- [ ] Implementar; `npm run build`.
- [ ] Commit `feat(web): estado vacío, menú de acciones y ayudas legibles`.

### Tarea 3: Mis APIs

**Archivos:** `web/src/paginas/Accesos.vue`, `web/src/idiomas/{es,en}.ts`.

- `CabeceraPagina` (acción «Nueva API» en la sección de APIs por aplicación).
- API principal en dos bloques (`q-card-section` con subtítulo): «Conexión» (URLs y clave) y «Ejemplos».
- Modelos de `model` agrupados por agente: perfiles primero y luego por agente en el orden fijo, cada grupo con
  su chapa de color (`linea--<agente>`).
- Fila de API en rejilla de cuatro columnas (`grid-template-columns: auto minmax(0,1.3fr) minmax(0,1fr) auto`;
  una columna en `lt-md`): insignia; nombre, estado, etiquetas JSON/respaldo, URL con botón copiar, «agente ·
  contratos»; uso y `BarrasPresupuesto`; botón «Ajustes» y `MenuAcciones` con Activar/Desactivar, Abrir en Probar,
  Regenerar la clave, Reiniciar consumo (oculto sin presupuesto) y Eliminar (peligro). Se quitan el `q-toggle` y
  los iconos sueltos.
- Sin APIs: `EstadoVacio` con acción «Nueva API».
- [ ] Implementar; `npm run build`.
- [ ] Verificar en el navegador (claro, oscuro, 375 px) con: API activa con presupuesto, API desactivada, API sin
  presupuesto; menú con teclado.
- [ ] Commit `feat(web): Mis APIs más legible: fila en cuatro zonas y menú de acciones`.

### Tarea 4: Agentes

**Archivos:** `web/src/paginas/Agentes.vue`.

- Orden fijo `['claude', 'codex', 'gemini', 'antigravity']` (los desconocidos al final, por nombre).
- Rejilla sin altura forzada (`align-items: start`); `CabeceraPagina` con la acción «Volver a comprobar».
- [ ] Implementar; `npm run build`; verificar en claro y oscuro.
- [ ] Commit `feat(web): agentes en orden fijo y tarjetas a su medida`.

### Tarea 5: Probar

**Archivos:** `web/src/paginas/Ejecutar.vue`, `web/src/componentes/EditorReceta.vue`.

- `EditorReceta` gana la prop `plegarOpciones?: boolean`; con ella, esfuerzo, respaldo, salida JSON y «usar
  siempre este modelo» van en un `q-expansion-item` «Opciones de la receta», abierto si alguna tiene valor.
  Probar lo usa; Mis APIs no.
- La columna de respuesta lleva `position: sticky; top: 76px` en `gt-sm`.
- `CabeceraPagina`; respuesta vacía con `EstadoVacio` (las ideas para probar siguen debajo).
- [ ] Implementar; `npm run build`; verificar el plegado con una receta abierta desde Mis APIs con esquema.
- [ ] Commit `feat(web): Probar con lo esencial arriba y opciones de la receta plegables`.

### Tarea 6: Historial, Inicio y Ayuda

**Archivos:** `web/src/paginas/{Trabajos,Panel,Ayuda}.vue`, `web/src/idiomas/{es,en}.ts`.

- Historial: `CabeceraPagina` (Vaciar historial, Actualizar); vacío sin filtro «Aún no hay peticiones» con acción
  «Ir a Probar»; con filtro «Ninguna petición con este filtro» con acción «Quitar filtros».
- Inicio: `EstadoVacio` en recientes.
- Ayuda: `CabeceraPagina`; el paso «Haz que arranque solo» dice que el arranque con el sistema existe en Windows
  (macOS y Linux, pendiente).
- [ ] Implementar; `go test ./internal/idioma` y `npm run build`.
- [ ] Commit `feat(web): estados vacíos y cabecera común en Historial, Inicio y Ayuda`.

### Tarea 7: Ajustes

**Archivos:** `web/src/paginas/Configuracion.vue`.

- Una sola columna; cada sección con el mismo patrón (`.ajuste`: nombre y explicación a la izquierda, control a la
  derecha, una debajo de otra en `lt-sm`). «Valores técnicos» al final, plegado.
- [ ] Implementar; `npm run build`; verificar en claro, oscuro y 375 px.
- [ ] Commit `feat(web): Ajustes en una columna con el mismo patrón`.

### Tarea 8: Esfuerzo predeterminado por modelo

**Archivos:** `internal/adaptadores/esfuerzo.go` y su prueba, `internal/api/nativa.go`, `web/src/api.ts`,
`web/src/componentes/SelectorEsfuerzo.vue` y quienes lo usan.

- [ ] Investigar los predeterminados con la documentación oficial y el CLI real (`claude --help`,
  `codex --help`, configuración documentada de Codex `model_reasoning_effort`). Anotar la fuente en un comentario.
  Solo entran valores comprobados.
- [ ] Prueba que falla `TestEsfuerzoPredeterminado`: un agente/modelo conocido devuelve su nivel; uno desconocido
  (y los agentes sin esfuerzo) devuelve "".
- [ ] `func EsfuerzoPredeterminado(agente, modelo string) string` con la tabla `esfuerzoPredeterminado
  map[string]string` (clave `agente` o `agente/modelo`; el modelo exacto gana al agente).
- [ ] `modelo` en `nativa.go` gana `EsfuerzoPredeterminado string` (`json:"esfuerzo_predeterminado,omitempty"`),
  rellenado junto a `Esfuerzos`; para un perfil, con el agente/modelo resuelto.
- [ ] `SelectorEsfuerzo` gana la prop `predeterminado?: string`; con ella la opción vacía dice
  `t('esfuerzo.agenteCon', { nivel })` («El del agente (medio)»).
- [ ] `go test ./internal/adaptadores ./internal/api`, `npm run build`, verificar en Probar y Mis APIs.
- [ ] Commit `feat: el selector de esfuerzo muestra el predeterminado del modelo`.

### Tarea 9: Actualizar sin pantalla en blanco

**Archivos:** según la causa; candidatos `internal/servicio/web.go` (cabeceras de `index.html`),
`escritorio/main.go`, `web/src/App.vue`.

- [ ] Depurar con superpowers:systematic-debugging. Hipótesis por orden: (1) `index.html` queda en caché y, tras
  instalar, apunta a archivos con hash de la versión anterior que ya no existen (comprobar `Cache-Control` de `/`
  y de `index.html`); (2) la ventana se recarga hacia el servicio nuevo (`repasar` → `cargar` en
  `escritorio/main.go`) antes de que responda. Reproducir con dos binarios de versiones distintas en el entorno
  aislado.
- [ ] Prueba que reproduce la causa (por ejemplo, `index.html` servido con `Cache-Control: no-cache` en
  `internal/servicio/web_test.go`) y que falla antes del arreglo.
- [ ] Arreglo de la causa; además, en `App.vue`, `progreso` se fija a «descargando» antes de llamar a
  `api.actualizar()` para que el avance se vea al instante.
- [ ] `go test ./internal/servicio`, `npm run build`; repetir la reproducción.
- [ ] Commit `fix: actualizar ya no deja la ventana en blanco`.

### Tarea 10: Página del producto

**Archivos:** `site/index.html`, `site/estilo.css`, `site/sitio.js`, `site/ayuda.html`, capturas nuevas en `site/`.

- Etiquetas del mapa del hero a la derecha de cada estación (texto tras el círculo, sin cruzar líneas).
- Selector de tema (Sistema, Claro, Oscuro) junto a ES/EN, en las dos páginas: atributo `data-tema` en `<html>`;
  `estilo.css` aplica las variables oscuras con `@media (prefers-color-scheme: dark)` solo si no hay
  `data-tema="claro"`, y siempre con `data-tema="oscuro"`; `sitio.js` guarda la elección en `localStorage` dentro de
  `try/catch` y un script corto en `<head>` la aplica antes de pintar.
- Sección «Así se ve» tras «Cómo funciona» con las capturas de Inicio y Mis APIs en claro y oscuro (según el tema
  efectivo), `alt` descriptivo y tamaño fijado.
- Funciones en tres bloques con titular (Conecta, Controla, Protege).
- [ ] Implementar; verificar en el navegador (`pagina`, puerto 8765) con los tres temas, en español e inglés y a
  375 px.
- [ ] Commit `feat(sitio): selector de tema, capturas reales y mapa sin cruces`.

### Tarea 11: Capturas, documentación y cierre

- [ ] Capturas nuevas 1440×900 con la versión compilada: `docs/imagenes/inicio.png` y
  `docs/imagenes/mis-apis-oscuro.png` (README) y las de la página (Tarea 10).
- [ ] `docs/diseno/DESIGN.md`: las piezas comunes (cabecera, estado vacío, menú de acciones) y el tamaño de las
  ayudas.
- [ ] `go vet ./...`, `go test ./...`, `npm run build`, `graphify update .`.
- [ ] Revisión de diseño y accesibilidad de toda la rama por un revisor nuevo; aplicar lo que encuentre.
- [ ] Commit `docs: capturas y sistema de diseño del rediseño`.
