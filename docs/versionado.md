# Versionado

Formato: **`vX.Y.Z+DDMMAAAA`**, por ejemplo `v0.1.0+12102026`. La versión **se calcula a partir de git**: no hay que editar ningún archivo.

## Cómo se calcula

`scripts/version.ps1` parte de la última etiqueta `v*` (o de `0.0.0` si no hay) y la sube según los mensajes de los commits posteriores ([Conventional Commits](https://www.conventionalcommits.org/es/)). Se aplica el mayor salto encontrado:

| Commit | Salto | Ejemplo |
|---|---|---|
| `tipo!: …` o `BREAKING CHANGE:` en el cuerpo | mayor | `v1.4.2` → `v2.0.0` |
| `feat: …` / `feat(api): …` | menor | `v1.4.2` → `v1.5.0` |
| cualquier otro (`fix`, `docs`, `chore`, `refactor`, `test`…) | parche | `v1.4.2` → `v1.4.3` |

- `+DDMMAAAA` es el día de la compilación. En SemVer es metadato: no cambia la precedencia.
- Sin commits nuevos desde la etiqueta, la versión es la etiqueta tal cual (también su fecha).
- Los cambios sin confirmar no cambian el número. El ejecutable los indica: `tramtai version` muestra `v0.1.0+12102026 (7deffc4, con cambios sin confirmar)`.

## Dónde se ve la versión

- `tramtai version` y `tramtai --version`, con el commit compilado.
- Cabecera de `tramtai agentes` y de `tramtai ejecutar`.
- El paquete [`internal/version`](../internal/version/version.go) es la única fuente. El servicio HTTP (M4) y la aplicación de escritorio (M6) la mostrarán desde ahí.

## Comandos

```powershell
./scripts/version.ps1 -Explicar    # lista cada commit con su salto y la versión resultante
./scripts/compilar.ps1             # compila bin/tramtai.exe con esa versión incrustada
./scripts/version.ps1 -Etiquetar   # crea la etiqueta git anotada (exige árbol limpio y commits nuevos)
git push origin --tags             # sube las etiquetas
```

## Flujo para publicar

1. Commits con prefijo: `feat: …`, `fix: …`, `docs: …`.
2. `./scripts/version.ps1 -Explicar` para revisar la versión que saldrá.
3. `./scripts/version.ps1 -Etiquetar` y `git push origin <etiqueta>`.
4. El release de GitHub se creará desde la etiqueta cuando exista el CI/CD (ver [pendientes.md](pendientes.md)).

Nota: Go no admite `+metadatos` como versión de módulo. No afecta, porque Tramtai es una aplicación y no una biblioteca importable.
