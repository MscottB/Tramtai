#!/bin/sh
# Abre la app de paquete/Tramtai.app y comprueba que levanta el servicio. Lo usan el CI y la publicación en macOS.
set -eu
cd "$(dirname "$0")/../.."
macos=paquete/Tramtai.app/Contents/MacOS
"$macos/tramtai" version
"$macos/uv" --version
("$macos/tramtai-escritorio" --oculto >/tmp/tramtai-escritorio.log 2>&1 || true) &
for _ in $(seq 1 30); do
	sleep 1
	if "$macos/tramtai" servicio estado 2>/dev/null | grep -q "^servicio activo"; then
		break
	fi
done
"$macos/tramtai" servicio estado
if ! "$macos/tramtai" servicio estado | grep -q "^servicio activo"; then
	cat /tmp/tramtai-escritorio.log
	exit 1
fi
"$macos/tramtai" servicio detener --si
pkill -f tramtai-escritorio || true
echo "la app de macOS arrancó y levantó el servicio"
