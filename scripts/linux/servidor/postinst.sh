#!/bin/sh
# Tras instalar o actualizar el paquete tramtai-servidor (deb: configure; rpm: %post). Crea la cuenta tramtai y su
# carpeta. Al instalar no arranca nada: el paquete no puede pedir que se acepten los términos, eso lo hace
# «tramtai-servidor instalar», que también habilita el servicio. Al actualizar, si ya estaba habilitado, lo reinicia con
# el programa nuevo (systemctl deja terminar las peticiones en curso).
set -e
systemd-sysusers /usr/lib/sysusers.d/tramtai.conf
systemd-tmpfiles --create /usr/lib/tmpfiles.d/tramtai.conf
if systemctl is-enabled --quiet tramtai 2>/dev/null; then
	systemctl daemon-reload
	systemctl try-restart tramtai
else
	echo "Para empezar: sudo tramtai-servidor instalar"
	echo "To get started: sudo tramtai-servidor install"
fi
