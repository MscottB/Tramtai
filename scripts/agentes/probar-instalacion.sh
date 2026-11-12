#!/bin/sh
# M0 de la v0.60 (docs/equipo/releases/v0.60-preparar-agentes/m0.md): instala un CLI con su instalador oficial en una
# carpeta de usuario temporal (HOME y XDG_* apuntan ahí) e imprime dónde quedó, su --version, los archivos que dejó y
# qué archivos de arranque de la shell tocó. Con ACTUALIZAR=1 ejecuta después su orden de actualizar y, con SESION=1,
# lanza su inicio de sesión dos veces (con tuberías y con una pseudoterminal de `script`), guarda lo que imprime en 25 s
# y lo cierra; con DESINSTALAR=1, lo quita como lo haría la receta. Lo usa el job agentes-compatibilidad.yml:
#   [VERSION=2.1.311] [ACTUALIZAR=1] [SESION=1] [DESINSTALAR=1] sh scripts/agentes/probar-instalacion.sh claude [carpeta]
set -eu

agente="${1:?uso: probar-instalacion.sh claude|codex|copilot|antigravity [carpeta]}"
casa="${2:-${TMPDIR:-/tmp}/m0-agentes-$agente-$$}"
version="${VERSION:-}"
mkdir -p "$casa"
casa="$(cd "$casa" && pwd)"
export HOME="$casa" XDG_CONFIG_HOME="$casa/.config" XDG_DATA_HOME="$casa/.local/share" XDG_CACHE_HOME="$casa/.cache" \
    XDG_STATE_HOME="$casa/.local/state" CLAUDE_CONFIG_DIR="$casa/.claude" CODEX_HOME="$casa/.codex" \
    COPILOT_HOME="$casa/.copilot" CODEX_NON_INTERACTIVE=1 BROWSER="$casa/no-abrir-navegador"
# Copilot, si BROWSER falla, abre el navegador del sistema; con esta variable usa solo la orden que se le da.
export COPILOT_DEBUG_BROWSER="[\"$casa/no-abrir-navegador\"]"
mkdir -p "$CLAUDE_CONFIG_DIR"
# Los instaladores escriben el PATH en el archivo de arranque de la shell que les toca: se crean vacíos para ver cuál.
for f in .bashrc .bash_profile .profile .zshrc .zprofile; do : >"$casa/$f"; done

seccion() { printf '\n=== %s\n' "$1"; }
# El instalador se baja a un archivo: con «curl | bash </dev/null» bash leería el guion de /dev/null.
instalador="$casa/instalador.sh"
bajar() { curl -fsSL "$1" -o "$instalador"; }
seccion "Instalar $agente ${version:-(última)} en $casa"
inicio=$(date +%s)
case "$agente" in
claude)
    # https://claude.ai/install.sh → downloads.claude.ai: SHA-256 del manifiesto y «claude install [versión]».
    bajar https://claude.ai/install.sh && bash "$instalador" ${version:+"$version"} </dev/null
    exe="$casa/.local/bin/claude" ;;
codex)
    # https://chatgpt.com/codex/install.sh → releases.openai.com (codex-package_SHA256SUMS). Versión: CODEX_RELEASE.
    bajar https://chatgpt.com/codex/install.sh && CODEX_RELEASE="${version:-latest}" sh "$instalador" </dev/null
    exe="$casa/.local/bin/codex" ;;
copilot)
    # https://gh.io/copilot-install → GitHub Releases (SHA256SUMS.txt). Versión: VERSION; destino: PREFIX/bin.
    bajar https://gh.io/copilot-install && VERSION="${version:-latest}" PREFIX="$casa/.local" bash "$instalador" </dev/null
    exe="$casa/.local/bin/copilot" ;;
antigravity)
    # https://antigravity.google/cli/install.sh → manifiesto con SHA-512; siempre la última; luego «agy install».
    bajar https://antigravity.google/cli/install.sh && bash "$instalador" </dev/null
    exe="$casa/.local/bin/agy" ;;
*) echo "agente desconocido: $agente" >&2; exit 2 ;;
esac
echo "Tardó $(($(date +%s) - inicio)) s"

seccion 'Dónde quedó'
[ -x "$exe" ] || { echo "no está en $exe" >&2; exit 1; }
ls -l "$exe"
seccion 'Versión'
"$exe" --version </dev/null
seccion 'Archivos de arranque de la shell que cambiaron'
for f in .bashrc .bash_profile .profile .zshrc .zprofile .config/fish/conf.d/copilot.fish; do
    if [ -s "$casa/$f" ]; then echo "--- $f"; cat "$casa/$f"; fi
done
seccion 'Archivos en la carpeta de usuario'
find "$casa" -type f -o -type l | grep -v -e '/\.cache/' -e '/tmp/' | head -80 | sed "s|^$casa||"

if [ "${ACTUALIZAR:-}" = 1 ]; then
    seccion 'Actualizar'
    "$exe" update </dev/null || echo "salió con $?"
    "$exe" --version </dev/null
fi

# Lanza el inicio de sesión 25 s y lo cierra. Con tuberías: la entrada es una FIFO abierta que no escribe nadie (como
# el servicio, que la deja abierta para pasar lo pegado). Con pty: `script` (util-linux en Linux, BSD en macOS).
probar_sesion() {
    modo="$1"; shift
    seccion "Inicio de sesión ($modo): $(basename "$exe") $*"
    salida="$casa/sesion-$modo.txt"
    fifo="$casa/entrada-$modo"; rm -f "$fifo"; mkfifo "$fifo"
    sleep 40 >"$fifo" & retenedor=$!
    if [ "$modo" = tuberias ]; then
        "$exe" "$@" <"$fifo" >"$salida" 2>&1 &
    elif [ "$(uname -s)" = Darwin ]; then
        script -q "$salida" "$exe" "$@" <"$fifo" >/dev/null 2>"$salida.err" &
    else
        script -qfec "$(printf '%s ' "$exe" "$@")" "$salida" <"$fifo" >/dev/null 2>&1 &
    fi
    pid=$!
    n=0; while [ $n -lt 25 ] && kill -0 $pid 2>/dev/null; do sleep 1; n=$((n + 1)); done
    if kill -0 $pid 2>/dev/null; then
        echo "sigue esperando a los $n s: se cierra"
        pkill -TERM -P $pid 2>/dev/null || true; kill -TERM $pid 2>/dev/null || true
        sleep 1; pkill -KILL -P $pid 2>/dev/null || true; kill -KILL $pid 2>/dev/null || true
    else
        wait $pid && codigo=0 || codigo=$?
        echo "terminó solo a los $n s con $codigo"
    fi
    kill $retenedor 2>/dev/null || true
    cat -v "$salida"
    if [ -s "$salida.err" ]; then echo "--- errores de script"; cat "$salida.err"; fi
}
if [ "${SESION:-}" = 1 ] && [ "$agente" != antigravity ]; then
    case "$agente" in
    claude) set -- auth login ;;
    codex) set -- login --device-auth ;;
    copilot) set -- login --device-code ;;
    esac
    probar_sesion tuberias "$@"
    probar_sesion pty "$@"
    if [ "$agente" = copilot ]; then probar_sesion tuberias login; fi # sin --device-code: ¿flujo web o código?
fi

if [ "${DESINSTALAR:-}" = 1 ]; then
    # Ninguno tiene orden de desinstalar: se borra lo que dejó su instalador. Las líneas del PATH que añadieron a los
    # archivos de arranque de la shell se quedan.
    seccion 'Desinstalar'
    case "$agente" in
    claude) rm -f "$exe"; rm -rf "$casa/.local/share/claude" ;;
    codex) rm -f "$exe" "$casa/.local/bin/codex-code-mode-host"; rm -rf "$CODEX_HOME/packages/standalone" ;;
    copilot | antigravity) rm -f "$exe" ;;
    esac
    if [ -e "$exe" ]; then echo "sigue $exe"; else echo "ya no está $exe"; fi
fi
exit 0
