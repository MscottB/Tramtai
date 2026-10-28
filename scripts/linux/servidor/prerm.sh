#!/bin/sh
# Antes de quitar el paquete tramtai-servidor (deb: prerm remove; rpm: %preun con 0), no al actualizarlo (deb: upgrade;
# rpm: 1): detiene el servicio, que deja terminar las peticiones en curso, y lo deshabilita. Sin systemd en marcha (un
# contenedor) solo lo deshabilita.
set -e
case "$1" in
remove | 0)
	if [ -d /run/systemd/system ]; then
		systemctl disable --now tramtai
	else
		systemctl disable tramtai 2>/dev/null || :
	fi
	;;
esac
