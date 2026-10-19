# El conversor de archivos

Tramtai convierte PDF, Word, PowerPoint, Excel y HTML a Markdown con [markitdown](https://github.com/microsoft/markitdown).
No se instala aparte: cada paquete trae `uv` y Tramtai prepara markitdown con su propio Python en la carpeta del usuario
(v0.50, [spec](releases/v0.50-archivos/spec.md)).

## Qué está fijado y dónde

| Qué | Dónde | Lo usan |
|---|---|---|
| markitdown y sus extras | `internal/documentos/conversor/conversor.in` | el lock |
| Todas las dependencias con sus hashes | `internal/documentos/conversor/conversor.lock` (incrustado con `go:embed`) | la preparación (`--require-hashes`) |
| uv: versión y SHA-256 por objetivo | `scripts/uv.txt` | `scripts/uv.sh` (Linux y macOS), `scripts/uv.ps1` (Windows) |
| Python | `3.12` en `internal/documentos/preparacion.go` | `uv python install` |
| onnxruntime `<1.20` | `conversor.in` | la 1.20 solo tiene ruedas para macOS 13+; Tramtai admite macOS 11 |

La huella del lock queda en `listo.json` (`lock_sha256`): si una versión nueva de Tramtai cambia el lock, el servicio
vuelve a preparar el conversor al iniciar y, mientras tanto, sigue usando el anterior.

## Cambiar la versión de markitdown (o sus dependencias)

1. Edita `conversor.in` y regenera el lock con el uv fijado:

   ```bash
   uv pip compile internal/documentos/conversor/conversor.in --universal --generate-hashes --python-version 3.12 --no-header -o internal/documentos/conversor/conversor.lock
   ```

2. Cambia la versión que se escribe en `listo.json` (`PrepararEn`, en `preparacion.go`) y los textos que la citan.
3. Prueba la preparación real: `go test -tags conversor -run TestConversorReal -v ./internal/documentos` (con
   `TRAMTAI_UV` o uv en el PATH). El CI la repite en Windows, Linux y macOS (job «Conversor») y comprueba que el lock
   tiene ruedas para los cinco objetivos de `uv.txt` con macOS 11 (`uv pip install --dry-run … --python-platform`),
   porque la matriz no ejecuta Intel Mac, macOS 11 ni Linux arm64.

## Cambiar la versión de uv

1. En `scripts/uv.txt`, la versión y el SHA-256 de cada objetivo: son los `.sha256` del release de
   [astral-sh/uv](https://github.com/astral-sh/uv/releases) (`uv-<objetivo>.<zip|tar.gz>.sha256`).
2. Comprueba la descarga: `./scripts/uv.ps1 -Objetivo x86_64-pc-windows-msvc -Destino bin` y
   `sh scripts/uv.sh x86_64-unknown-linux-musl /tmp/uv`; un hash equivocado para el script.

## Dónde se prepara

`documentos.RaizConversor`: `%LOCALAPPDATA%\tramtai\conversor` en Windows, `$XDG_DATA_HOME` o
`~/.local/share/tramtai/conversor` en Linux, la carpeta de Tramtai en macOS y `datos-tramtai/conversor` en la portable.
Se trabaja en `conversor.nuevo` y se reemplaza al final, cuando no hay ninguna conversión en curso (`Preparacion.uso`).

## Dónde va uv en cada paquete

Junto a `tramtai.exe` (instalador y portable de Windows), junto a `tramtai` en los `.tar.gz` y dentro de `Tramtai.app`
(universal con `lipo`); en `.deb`/`.rpm`, en `/usr/lib/tramtai/uv`, en Homebrew en `lib/tramtai` y con
`scripts/instalar.sh` en `<destino>/../lib/tramtai/uv`. Tramtai lo busca
junto al ejecutable, en `../lib/tramtai` y en el PATH (`documentos.BuscarUV`).

## El paquete del conversor

Publicar (job «Paquete del conversor») compila `tramtai.exe`, prepara el conversor con
`tramtai conversor preparar --en <carpeta>`, lo prueba sin red convirtiendo las muestras de
`internal/documentos/testdata` y lo comprime en `Tramtai-<versión>-conversor-windows-x64.zip` (unos 100 MB) con
`scripts/conversor-leeme.txt`. Va en `SHA256SUMS.txt` con su firma, como los demás.
