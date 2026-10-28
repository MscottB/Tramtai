#!/bin/sh
# Tras instalar o actualizar el paquete tramtai-servidor (deb: configure; rpm: %post). Crea la cuenta tramtai y su
# carpeta. Al instalar no arranca nada: el paquete no puede pedir que se acepten los términos, eso lo hace
# «tramtai-servidor instalar», que también habilita el servicio. Al actualizar, si ya estaba habilitado, lo reinicia con
# el programa nuevo (systemctl deja terminar las peticiones en curso). Sin systemd en marcha (un chroot, una imagen) no
# llama a systemctl.
set -e
systemd-sysusers /usr/lib/sysusers.d/tramtai.conf
systemd-tmpfiles --create /usr/lib/tmpfiles.d/tramtai.conf
if [ -d /run/systemd/system ] && systemctl is-enabled --quiet tramtai 2>/dev/null; then
	systemctl daemon-reload
	systemctl try-restart tramtai
fi
# Sin el drop-in de «tramtai-servidor instalar» falta ese paso, aunque haya habilitada una unidad del .tar.gz.
if [ ! -e /etc/systemd/system/tramtai.service.d/10-tramtai.conf ]; then
	echo "Para empezar: sudo tramtai-servidor instalar"
	echo "To get started: sudo tramtai-servidor install"
fi
