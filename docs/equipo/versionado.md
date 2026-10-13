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

Cada versión publicada tiene una **etiqueta anotada** cuyo mensaje son sus notas: los commits desde la etiqueta anterior agrupados en *Cambios incompatibles*, *Nuevas funciones*, *Correcciones* y *Otros cambios*, con el cuerpo de cada commit. Con `-Resumen` se añade un párrafo inicial que explica lo que los mensajes no dicen.

```powershell
git tag -n99 --sort=creatordate          # todas las versiones con sus notas
git show v0.1.0+12102026                 # una versión concreta
```

En GitHub se ven en *Tags*. Un buen cuerpo de commit produce buenas notas: escribe en él la lista de cambios.

| Etiqueta | Commit | Contenido |
|---|---|---|
| `v0.0.1+12102026` | `efc9b62` | Base del proyecto: hito M1 |
| `v0.0.2+12102026` | `7deffc4` | Documentación, README y versionado |
| `v0.1.0+12102026` | `8382107` | Hito M2: supervisión de procesos y adaptadores Claude y Codex |

## Comandos

```powershell
./scripts/version.ps1 -Explicar    # versión resultante y sus notas
./scripts/compilar.ps1             # compila bin/tramtai.exe con esa versión incrustada
./scripts/version.ps1 -Etiquetar   # crea la etiqueta anotada con las notas (exige árbol limpio y commits nuevos)
./scripts/version.ps1 -Etiquetar -Resumen "Hito M3: …"   # con párrafo de resumen
./scripts/version.ps1 -Hasta <commit> -Etiquetar          # etiqueta un commit anterior
git push origin --tags             # sube las etiquetas
```

## Flujo para publicar

1. Commits con prefijo (`feat: …`, `fix: …`, `docs: …`) y la lista de cambios en el cuerpo.
2. `./scripts/version.ps1 -Explicar` para revisar la versión y sus notas.
3. `./scripts/version.ps1 -Etiquetar`, y después `git push` y `git push origin --tags`.
4. El release de GitHub se creará desde la etiqueta cuando exista el CI/CD (ver [pendientes.md](pendientes.md)).

Nota: Go no admite `+metadatos` como versión de módulo. No afecta, porque Tramtai es una aplicación y no una biblioteca importable.
