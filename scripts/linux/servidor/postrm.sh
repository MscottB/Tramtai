#!/bin/sh
# Tras quitar el paquete tramtai-servidor (deb: postrm remove; rpm: %postun con 0). Quita el drop-in que ajusta la unidad
# del paquete (claves.conf, de las claves, se queda). Nunca borra los datos (/var/lib/tramtai) ni las credenciales,
# tampoco con purge (que no hace nada más): con el paquete ya quitado no queda tramtai-servidor, así que dice qué borrar
# a mano, lo mismo que «tramtai-servidor desinstalar --borrar-datos».
set -e
case "$1" in
remove | 0)
	rm -f /etc/systemd/system/tramtai.service.d/10-tramtai.conf
	rmdir /etc/systemd/system/tramtai.service.d 2>/dev/null || :
	if [ -d /run/systemd/system ]; then
		systemctl daemon-reload
	fi
	if [ -d /var/lib/tramtai ]; then
		echo "Los datos de /var/lib/tramtai y las claves se conservan. Para borrarlos, como root:"
		echo "The data in /var/lib/tramtai and the keys are kept. To delete them, as root:"
		echo "  rm -rf /var/lib/tramtai /etc/credstore.encrypted/tramtai.* /etc/tramtai/claves.env /etc/systemd/system/tramtai.service.d/claves.conf"
		echo "  userdel tramtai; systemctl daemon-reload"
	fi
	;;
esac
