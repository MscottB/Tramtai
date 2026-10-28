# Versionado

Formato: **`vX.Y.Z+DDMMAAAA`**, por ejemplo `v0.1.0+12102026`. La versión **se calcula a partir de git**: no hay que editar ningún archivo.

## Cómo se calcula

`scripts/version.ps1` parte de la última etiqueta `v*` (o de `0.0.0` si no hay) y la sube según los mensajes de los commits posteriores ([Conventional Commits](https://www.conventionalcommits.org/es/)). Se aplica el mayor salto encontrado:

| Commit | Salto | Ejemplo |
|---|---|---|
| `tipo!: …` o `BREAKING CHANGE:` en el cuerpo | mayor | `v1.4.2` → `v2.0.0` |
| `feat: …` / `feat(api): …` | menor | `v1.4.2` → `v1.5.0` |
| cualquier otro (`fix`, `docs`, `chore`, `refactor`, `test`…) | parche | `v1.4.2` → `v1.4.3` |

- `+DDMMAAAA` es la fecha del commit versionado. En SemVer es metadato: no cambia la precedencia.
- Sin commits nuevos desde la etiqueta, la versión es la etiqueta tal cual (también su fecha).
- Los cambios sin confirmar no cambian el número. El ejecutable los indica: `tramtai version` muestra `v0.1.0+12102026 (7deffc4, con cambios sin confirmar)`.

## Dónde se ve la versión

- `tramtai version` y `tramtai --version`, con el commit compilado.
- Cabecera de `tramtai agentes` y de `tramtai ejecutar`.
- El paquete [`internal/version`](../../internal/version/version.go) es la única fuente. La CLI, la Web UI y la aplicación de escritorio muestran solo la versión; `tramtai version --detalle` añade el commit.

## Etiquetas y notas de versión

Cada versión publicada tiene una **etiqueta anotada** cuyo mensaje son sus notas, que GitHub publica en el release y la app muestra en «Novedades». El repositorio es público, así que las notas se escriben para quien usa Tramtai:

- **Cortas y claras:** una frase de resumen y, si hace falta, secciones `Novedades:`, `Mejoras:` o `Correcciones:` con viñetas `- …`, una por cambio que se note al usarlo.
- **Sin detalles internos:** ni nombres de funciones, archivos o pruebas, ni decisiones de diseño, ni revisiones. Los commits no entran en las notas (`-Explicar` los muestra aparte).
- **En los dos idiomas:** `-Resumen` en español y `-ResumenEn` en inglés, obligatorios al etiquetar. Cada uno puede ser un texto o la ruta de un archivo. El inglés va tras `## English` y la app lo muestra si está en inglés.
- Al final se añade el enlace a la comparación en GitHub, para quien quiera el detalle técnico.

```powershell
git tag -n99 --sort=creatordate          # todas las versiones con sus notas
git show v0.1.0+12102026                 # una versión concreta
```

En GitHub se ven en *Releases* y en *Tags*.

| Etiqueta | Commit | Contenido |
|---|---|---|
| `v0.0.1+12102026` | `efc9b62` | Base del proyecto: hito M1 |
| `v0.0.2+12102026` | `7deffc4` | Documentación, README y versionado |
| `v0.1.0+12102026` | `8382107` | Hito M2: supervisión de procesos y adaptadores Claude y Codex |

## Comandos

```powershell
./scripts/version.ps1 -Explicar    # versión resultante y sus notas
./scripts/compilar.ps1             # compila bin/tramtai.exe con esa versión incrustada
./scripts/version.ps1 -Etiquetar -Resumen notas-es.txt -ResumenEn notas-en.txt   # crea la etiqueta (árbol limpio y commits nuevos)
./scripts/version.ps1 -Hasta <commit> -Etiquetar -Resumen notas-es.txt -ResumenEn notas-en.txt   # etiqueta un commit anterior
git push origin --tags             # sube las etiquetas
```

## Flujo para publicar

`main` está protegida: solo cambia por PR y con la CI en verde. No hace falta que la rama esté al día con `main`: así una fusión no obliga a repetir la CI de los demás PR, y la CI de `main` tras cada fusión avisa si dos cambios juntos rompen algo.

1. Una rama por release (`feat/…`, `fix/…`, `chore/…`) con commits con prefijo (`feat: …`, `fix: …`, `docs: …`) y la lista de cambios en el cuerpo.
2. Un PR a `main` (`gh pr create`), que lanza la CI en Windows, Linux y macOS.
3. `gh pr merge --auto --merge --delete-branch`: GitHub lo fusiona solo en cuanto la CI está en verde (el repositorio permite auto-merge). Si la CI falla, el PR se queda abierto hasta arreglarlo.
4. En `main` actualizada (`git pull`), `./scripts/version.ps1 -Explicar` para revisar la versión y sus notas.
5. `./scripts/version.ps1 -Etiquetar -Resumen notas-es.txt -ResumenEn notas-en.txt` y `git push origin <etiqueta>`.
6. Al subir la etiqueta, `.github/workflows/publicar.yml` pasa el mismo CI que los PR (lo llama con `workflow_call`: pruebas en Windows, Linux y macOS y análisis de dependencias), compila en cada sistema y crea el release de GitHub con las notas de la etiqueta: en Windows el instalador y el `.zip` portable; en Linux los `.deb` y `.rpm` de escritorio y del servidor (amd64 y arm64); en macOS el `.dmg` universal; los `.tar.gz` de la CLI para Linux y macOS; y `SHA256SUMS.txt` con todos. Falla si la versión calculada no coincide con la etiqueta o si alguna app no arranca.

Antes del paso 1 van la spec y el plan del release, en `docs/equipo/releases/<versión>-<tema>/` (índice en [README.md](README.md)).

**Dependabot** (`.github/dependabot.yml`) propone cada semana, en un PR por ecosistema, las versiones menores y los parches de Go (el servicio y la app de escritorio, sin Wails, que va fijado en una beta), npm (la Web UI) y GitHub Actions. Pasan por los mismos checks que cualquier PR y se fusionan igual; un cambio de versión mayor llega en su propio PR (hasta 10 abiertos por ecosistema). Una versión mayor que todavía no se puede usar se cierra con «@dependabot ignore this major version» y un comentario con el motivo (por ejemplo, TypeScript 7, que `vue-tsc` aún no admite).

Nota: Go no admite `+metadatos` como versión de módulo. No afecta, porque Tramtai es una aplicación y no una biblioteca importable.

## Firma de las actualizaciones y permisos de Publicar

- **Firma.** Publicar firma `SHA256SUMS.txt` con ed25519 (`scripts/firmar`) y sube `SHA256SUMS.txt.sig`. La clave privada es el secreto `FIRMA_ACTUALIZACIONES` del repositorio y solo llega al paso que firma; las públicas están en la lista `clavesPublicasB64` de `internal/actualizacion/firma.go` (una firma vale si la verifica cualquiera). Antes de crear el release, Publicar comprueba la firma con esas mismas claves (`go run ./scripts/firmar -comprobar`), en un job sin caché de Go porque es el que tiene el secreto. La actualización de un clic de Windows no instala una versión sin firma válida.
- **Cambiar la clave**, en tres pasos:
  1. `go run ./scripts/firmar -generar` imprime la pública nueva por la salida estándar y la semilla por stderr. La semilla va directa a otro secreto (por ejemplo, `FIRMA_ACTUALIZACIONES_NUEVA`), sin pasar por el disco.
  2. Un release añade la pública nueva a `clavesPublicasB64`, junto a la vieja, y se firma todavía con el secreto viejo.
  3. Cuando las copias instaladas ya tienen ese release, se pone la semilla nueva en `FIRMA_ACTUALIZACIONES`. Más adelante, otro release puede quitar la pública vieja.

  Si se cambia el secreto antes del paso 2, las copias instaladas no podrán actualizarse con un clic.
- **Protección de `main`.** Un ruleset del repositorio exige PR (sin aprobaciones: el proyecto tiene una sola persona), los checks de la CI (`Go` en los tres sistemas, `App de escritorio` en Linux y macOS y `Web UI`), sin exigir la rama al día, e impide borrar `main` o reescribir su historia. La única excepción es la deploy key «Publicar: paquetes a main» (secreto `PAQUETES_DEPLOY_KEY`), con la que el job `paquetes` sube el commit de Homebrew y winget de cada release; en un repositorio personal GitHub no deja que la app de Actions se salte un ruleset, pero sí una deploy key, que solo vale para este repositorio.
- **Permisos.** Publicar tiene `contents: read` por defecto: los jobs que compilan y prueban ejecutan código de terceros. Solo `publicar` (crea el release) tiene `contents: write`; `paquetes` sube Homebrew y winget a `main` con la deploy key, no con el token, y los checkouts no guardan el token (tampoco los del CI). wingetcreate va fijado por versión y SHA-256, y `WINGET_TOKEN` solo llega al paso que envía el manifiesto. NSIS también: el instalador oficial de la 3.13 (SourceForge) con el SHA-256 de su manifiesto de winget; si no se descarga o no coincide, Publicar se para en ese paso.

