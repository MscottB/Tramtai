#!/bin/sh
# Prueba el paquete del servidor de dist/ junto al de escritorio, en un contenedor limpio: debian:13 para el .deb y
# rockylinux:9 para el .rpm (necesita Docker). Lo usan el CI y la publicación, después de empaquetar.sh:
#   sh scripts/linux/probar-paquete-servidor.sh deb
# Instala los dos a la vez (sin las dependencias de GTK del de escritorio, que rockylinux:9 no tiene y aquí no se
# prueban; con systemd, que el servidor necesita), comprueba los archivos y la cuenta, que /usr/bin/tramtai sigue siendo
# del de escritorio y que no se arrancó nada, y quita el del servidor (purge en el .deb) conservando /var/lib/tramtai y
# sin dejar carpetas ni el drop-in. Después lo instala encima de lo que deja una instalación desde el .tar.gz.
set -eu
tipo=$1
case $tipo in
deb) imagen=debian:13 ;;
rpm) imagen=rockylinux:9 ;;
*) echo "uso: $0 deb|rpm" >&2 && exit 2 ;;
esac
cd "$(dirname "$0")/../.."
if [ -z "${TRAMTAI_EN_CONTENEDOR:-}" ]; then
	exec docker run --rm -e TRAMTAI_EN_CONTENEDOR=1 -e TRAMTAI_IDIOMA=es -v "$PWD:/src:ro" -w /src "$imagen" sh scripts/linux/probar-paquete-servidor.sh "$tipo"
fi

falla() {
	echo "FALLA: $*" >&2
	exit 1
}
# sin_proceso: ningún tramtai del servidor en marcha; nada_en_marcha: además, el servicio sin habilitar.
sin_proceso() {
	for exe in /proc/[0-9]*/exe; do
		[ "$(readlink "$exe" 2>/dev/null || :)" != /opt/tramtai/tramtai ] || falla "hay un tramtai del servidor en marcha"
	done
}
nada_en_marcha() {
	[ "$(systemctl is-enabled tramtai 2>/dev/null || :)" != enabled ] || falla "el paquete habilitó el servicio"
	sin_proceso
}

servidor=$(ls dist/Tramtai-*-servidor-linux-*."$tipo")
escritorio=$(ls dist/Tramtai-*-linux-*."$tipo" | grep -v -- -servidor-linux-)
if [ "$tipo" = deb ]; then
	apt-get update -qq && apt-get install -y -qq systemd >/dev/null
	salida=$(dpkg -i --force-depends "$escritorio" "$servidor" 2>&1) || falla "dpkg -i: $salida"
	duenno=$(dpkg -S /usr/bin/tramtai)
else
	dnf install -y -q systemd >/dev/null
	salida=$(rpm -i --nodeps "$escritorio" "$servidor" 2>&1) || falla "rpm -i: $salida"
	duenno=$(rpm -qf --qf '%{NAME}' /usr/bin/tramtai)
fi
echo "$salida"
echo "$salida" | grep -q "Para empezar: sudo tramtai-servidor instalar" || falla "el paquete no dice cómo empezar"

for f in /opt/tramtai/tramtai /opt/tramtai/uv /opt/tramtai/.paquete /usr/bin/tramtai-servidor \
	/usr/lib/systemd/system/tramtai.service /usr/lib/sysusers.d/tramtai.conf /usr/lib/tmpfiles.d/tramtai.conf \
	/usr/share/doc/tramtai-servidor/TERMINOS.md /usr/share/doc/tramtai-servidor/LICENCIA.md /usr/bin/tramtai-escritorio; do
	[ -e "$f" ] || falla "falta $f"
done
case $duenno in tramtai | tramtai:*) ;; *) falla "/usr/bin/tramtai no es del paquete de escritorio: $duenno" ;; esac
cuenta=$(getent passwd tramtai) || falla "no se creó la cuenta tramtai"
case $cuenta in *:/var/lib/tramtai:/usr/sbin/nologin) ;; *) falla "cuenta tramtai: $cuenta" ;; esac
[ "$(stat -c '%a %U %G' /var/lib/tramtai)" = "700 tramtai tramtai" ] || falla "/var/lib/tramtai: $(stat -c '%a %U %G' /var/lib/tramtai)"
[ -z "$(ls -A /var/lib/tramtai)" ] || falla "el paquete escribió en /var/lib/tramtai: $(ls -A /var/lib/tramtai)"
nada_en_marcha

tramtai-servidor version
# Antes de «tramtai-servidor instalar» no se arranca nada, y actualizar remite a apt o dnf.
if salida=$(tramtai-servidor reiniciar 2>&1); then falla "reiniciar sin instalar: $salida"; fi
echo "$salida" | grep -q "no está instalado" || falla "reiniciar sin instalar: $salida"
if salida=$(tramtai-servidor actualizar 2>&1); then falla "actualizar con el paquete: $salida"; fi
echo "$salida" | grep -q "actualízalo con apt o dnf" || falla "actualizar con el paquete: $salida"
nada_en_marcha

# Lo que deja «tramtai-servidor instalar» (sin systemd en marcha no se puede ejecutar): el drop-in y, con claves,
# claves.conf. Al quitar el paquete, el drop-in se va y claves.conf se queda.
dropins=/etc/systemd/system/tramtai.service.d
mkdir -p "$dropins"
echo "[Service]" >"$dropins/10-tramtai.conf"
echo "[Service]" >"$dropins/claves.conf"
quitar() {
	if [ "$tipo" = deb ]; then
		dpkg --purge tramtai-servidor
	else
		rpm -e tramtai-servidor
	fi
}
quitar
[ -d /var/lib/tramtai ] || falla "quitar el paquete borró /var/lib/tramtai"
for d in /opt/tramtai /usr/bin/tramtai-servidor /usr/share/doc/tramtai-servidor "$dropins/10-tramtai.conf"; do
	[ ! -e "$d" ] || falla "quitar el paquete deja $d"
done
[ -e "$dropins/claves.conf" ] || falla "quitar el paquete se llevó claves.conf"
[ -e /usr/bin/tramtai ] || falla "quitar el del servidor se llevó el de escritorio"
echo "el paquete del servidor ($tipo) se instala junto al de escritorio, no arranca nada y al quitarlo conserva los datos"

# Del .tar.gz al paquete: con la unidad del .tar.gz habilitada, su envoltorio y un programa en /opt/tramtai, el paquete
# se instala encima y remite a «tramtai-servidor instalar», que es quien limpia (con systemd; lo prueba internal/servidor).
mkdir -p /opt/tramtai /etc/systemd/system/multi-user.target.wants
cp scripts/linux/servidor/tramtai.service /etc/systemd/system/tramtai.service
ln -s ../tramtai.service /etc/systemd/system/multi-user.target.wants/tramtai.service
cp scripts/linux/servidor/tramtai-servidor /usr/local/bin/tramtai-servidor
echo "del .tar.gz" >/opt/tramtai/tramtai
echo "de antes" >/opt/tramtai/tramtai.anterior
if [ "$tipo" = deb ]; then
	salida=$(dpkg -i "$servidor" 2>&1) || falla "dpkg -i sobre el .tar.gz: $salida"
else
	salida=$(rpm -i "$servidor" 2>&1) || falla "rpm -i sobre el .tar.gz: $salida"
fi
echo "$salida" | grep -q "Para empezar: sudo tramtai-servidor instalar" || falla "sobre el .tar.gz, el paquete no dice cómo empezar: $salida"
/usr/bin/tramtai-servidor version | grep -q "^tramtai " || falla "el programa del paquete no sustituyó al del .tar.gz"
[ -e /opt/tramtai/.paquete ] || falla "falta /opt/tramtai/.paquete sobre el .tar.gz"
sin_proceso
echo "el paquete del servidor ($tipo) se instala encima de una instalación desde el .tar.gz"
