#!/bin/sh
# Instala el .deb de dist/ y comprueba que la app de escritorio arranca y levanta el servicio, sin pantalla (Xvfb).
# Lo usan el CI y la publicación en Ubuntu. El runner no tiene sesión D-Bus ni deja crear el sandbox de WebKitGTK
# (bubblewrap): solo en esta prueba se arranca dentro de dbus-run-session y sin ese sandbox; el paquete no cambia.
set -eu
cd "$(dirname "$0")/../.."
sudo apt-get install -y ./dist/*.deb xvfb dbus-x11
tramtai version
/usr/lib/tramtai/uv --version
(WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1 dbus-run-session -- xvfb-run -a timeout 40 tramtai-escritorio --oculto || true) &
for _ in $(seq 1 30); do
	sleep 1
	if tramtai servicio estado 2>/dev/null | grep -q "^servicio activo"; then
		break
	fi
done
tramtai servicio estado
tramtai servicio estado | grep -q "^servicio activo"
tramtai servicio detener --si
echo "la app de escritorio arrancó y levantó el servicio"
