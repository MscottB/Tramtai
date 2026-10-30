#!/bin/sh
# Prueba el servidor de Linux con systemd de verdad, como root (lo usa el job «Servidor (Linux)» del CI en su runner de
# Ubuntu; necesita Go y curl). En dist/ espera dos versiones del .tar.gz de Linux (tramtai-<versión>-linux-amd64.tar.gz)
# y del paquete del servidor (Tramtai-<versión>-servidor-linux-amd64.deb):
#   sudo env "PATH=$PATH" sh scripts/servidor/probar-servidor.sh dist
# No lo ejecutes en tu equipo: instala y quita el servidor de Tramtai del sistema, con sus datos y su cuenta, y deja
# un Claude simulado en /usr/local/bin/claude mientras dura.
# Tres tramos, uno detrás de otro, limpiando entre ellos:
#   1. Desde el .tar.gz: instalar, contraseña del administrador, TLS propio con un certificado de prueba, iniciar
#      sesión, una petición al Claude simulado, guardar una clave con una petición larga en curso (debe terminar),
#      actualizar con «servidor instalar» desde el .tar.gz nuevo y desinstalar con --borrar-datos.
#   2. Con el paquete en la máquina limpia: apt install del .deb (no arranca nada) y «tramtai-servidor instalar»; lo
#      mismo que en 1; actualizar el paquete (try-restart) y quitarlo (prerm: disable --now).
#   3. Del .tar.gz al paquete: instalar desde el .tar.gz, el .deb encima y «tramtai-servidor instalar», que quita la
#      unidad de /etc y el envoltorio de /usr/local/bin y deja habilitada la del paquete sin enlaces colgando.
set -eu
[ "$(id -u)" = 0 ] || { echo "uso: sudo env \"PATH=\$PATH\" sh $0 <dist>" >&2 && exit 2; }
dist=$(cd "$1" && pwd)
cd "$(dirname "$0")/../.."
export TRAMTAI_IDIOMA=es # se comparan los textos de la CLI

falla() {
	echo "FALLA: $*" >&2
	journalctl -u tramtai --no-pager -n 40 >&2 || :
	exit 1
}

tar_viejo=$(ls "$dist"/tramtai-*-linux-amd64.tar.gz | sort -V | head -n 1)
tar_nuevo=$(ls "$dist"/tramtai-*-linux-amd64.tar.gz | sort -V | tail -n 1)
deb_viejo=$(ls "$dist"/Tramtai-*-servidor-linux-amd64.deb | sort -V | head -n 1)
deb_nuevo=$(ls "$dist"/Tramtai-*-servidor-linux-amd64.deb | sort -V | tail -n 1)
[ "$tar_viejo" != "$tar_nuevo" ] && [ "$deb_viejo" != "$deb_nuevo" ] || falla "hacen falta dos versiones del .tar.gz y del .deb en $dist"
version_nueva=$(basename "$tar_nuevo" | sed 's/^tramtai-v\{0,1\}\(.*\)-linux-amd64\.tar\.gz$/\1/')

trabajo=$(mktemp -d)
trap 'rm -rf "$trabajo" /usr/local/bin/claude' EXIT
mkdir "$trabajo/viejo" "$trabajo/nuevo"
tar -C "$trabajo/viejo" -xzf "$tar_viejo"
tar -C "$trabajo/nuevo" -xzf "$tar_nuevo"
# El Claude simulado, donde lo busca el servicio (su PATH empieza por /usr/local/bin).
CGO_ENABLED=0 go build -o /usr/local/bin/claude ./scripts/servidor/claude-simulado
# Certificado de prueba de tramtai.prueba; curl lo acepta como raíz.
(cd "$trabajo" && go run "$(go env GOROOT)/src/crypto/tls/generate_cert.go" --host tramtai.prueba --ecdsa-curve P256 --ca >/dev/null 2>&1)

contrasena=contrasena-de-prueba-del-ci
puerto=17380
https() { curl -sS --cacert "$trabajo/cert.pem" --resolve "tramtai.prueba:$puerto:127.0.0.1" "$@"; }
pid() { systemctl show -P MainPID tramtai; }
# esperar: el servicio en marcha responde a la CLI (con TLS propio, por su oyente local).
esperar() {
	for _ in $(seq 60); do
		tramtai-servidor servicio estado 2>/dev/null | grep -q "^servicio activo" && return 0
		sleep 1
	done
	falla "el servicio no responde: $(systemctl is-active tramtai || :)"
}

# comprobar_paquete: tras «tramtai-servidor instalar» con el paquete, el servicio queda habilitado con la unidad del
# paquete, con su drop-in, sin enlaces colgando y en marcha.
comprobar_paquete() {
	[ "$(systemctl is-enabled tramtai)" = enabled ] || falla "el servicio no quedó habilitado"
	[ "$(systemctl show -P FragmentPath tramtai)" = /usr/lib/systemd/system/tramtai.service ] ||
		falla "la unidad en uso es $(systemctl show -P FragmentPath tramtai)"
	# Solo los de Tramtai: la imagen del runner ya trae alguno colgando (apport.service).
	colgando=$(find /etc/systemd/system/multi-user.target.wants -name 'tramtai*' -xtype l)
	[ -z "$colgando" ] || falla "enlaces colgando en multi-user.target.wants: $colgando"
	[ -e /etc/systemd/system/tramtai.service.d/10-tramtai.conf ] || falla "falta el drop-in del paquete"
	esperar
}

# probar_en_marcha son los pasos 2 a 6 con el servidor ya instalado y en marcha.
probar_en_marcha() {
	printf '%s\n%s\n' "$contrasena" "$contrasena" | tramtai-servidor administrador contraseña >/dev/null 2>&1 ||
		falla "no se pudo poner la contraseña del administrador"

	# TLS propio con el certificado de prueba; reiniciar deja los archivos a nombre de la cuenta del servicio.
	mkdir -p /var/lib/tramtai/tls
	cp "$trabajo/cert.pem" "$trabajo/key.pem" /var/lib/tramtai/tls/
	cat >/var/lib/tramtai/config.yaml <<FIN
servicio:
  host: 127.0.0.1
  puerto: $puerto
  dominios: [tramtai.prueba]
  tls:
    modo: archivos
    certificado: /var/lib/tramtai/tls/cert.pem
    clave: /var/lib/tramtai/tls/key.pem
FIN
	tramtai-servidor reiniciar >/dev/null
	esperar
	for _ in $(seq 30); do
		https -o /dev/null "https://tramtai.prueba:$puerto/healthz" 2>/dev/null && break
		sleep 1
	done
	[ "$(stat -c %U /var/lib/tramtai/tls/key.pem)" = tramtai ] || falla "la clave del certificado no es de la cuenta tramtai"

	# Iniciar sesión por HTTPS: sin sesión 401, con la contraseña la sesión, y la entrada en el registro.
	codigo=$(https -o /dev/null -w '%{http_code}' "https://tramtai.prueba:$puerto/ui/acerca" || :)
	[ "$codigo" = 401 ] || falla "sin sesión, /ui/acerca da $codigo y no 401"
	codigo=$(https -c "$trabajo/galletas" -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' \
		-d "{\"contrasena\":\"$contrasena\",\"codigo\":\"\"}" "https://tramtai.prueba:$puerto/ui/acceder" || :)
	[ "$codigo" = 200 ] || falla "el inicio de sesión da $codigo"
	codigo=$(https -b "$trabajo/galletas" -o /dev/null -w '%{http_code}' "https://tramtai.prueba:$puerto/ui/acerca" || :)
	[ "$codigo" = 200 ] || falla "con la sesión, /ui/acerca da $codigo"
	grep -q "acceso correcto del administrador desde 127.0.0.1" /var/lib/tramtai/datos/servicio.log ||
		falla "el registro no anota el inicio de sesión"

	# Una petición al Claude simulado, con la clave guardada con systemd-creds.
	echo sk-ant-prueba-no-es-una-clave | tramtai-servidor claves guardar ANTHROPIC_API_KEY >/dev/null
	esperar
	salida=$(tramtai-servidor ejecutar hola -a claude 2>&1) || falla "la petición falló: $salida"
	echo "$salida" | grep -q "Hola desde el servidor" || falla "la petición no trajo la respuesta: $salida"

	# Revisión 4: guardar una clave reinicia el servicio con drenaje; la petición larga en curso termina.
	tramtai-servidor ejecutar "trabajo largo" -a claude >"$trabajo/largo.log" 2>&1 &
	cliente=$!
	id=
	for _ in $(seq 20); do
		id=$(tramtai-servidor trabajos listar 2>/dev/null | awk '$2 == "ejecutando" { print $1; exit }')
		[ -n "$id" ] && break
		sleep 0.5
	done
	[ -n "$id" ] || falla "la petición larga no llegó a ejecutarse: $(cat "$trabajo/largo.log")"
	antes=$(pid)
	echo sk-prueba-no-es-una-clave | tramtai-servidor claves guardar OPENAI_API_KEY >/dev/null
	esperar
	[ "$(pid)" != "$antes" ] || falla "guardar la clave no reinició el servicio"
	estado=$(tramtai-servidor trabajos listar | awk -v id="$id" '$1 == id { print $2 }')
	[ "$estado" = completado ] || falla "la petición larga quedó en «$estado» al guardar la clave"
	wait "$cliente" || : # su conexión puede cortarse con el reinicio; lo que cuenta es el trabajo
}

echo "== 1. desde el .tar.gz"
salida=$("$trabajo/viejo/tramtai/tramtai" servidor instalar --acepto-terminos 2>&1) || falla "instalar desde el .tar.gz: $salida"
for f in /opt/tramtai/tramtai /opt/tramtai/uv /usr/local/bin/tramtai-servidor /etc/systemd/system/tramtai.service; do
	[ -e "$f" ] || falla "instalar desde el .tar.gz no dejó $f"
done
[ "$(systemctl is-active tramtai)" = active ] || falla "el servicio no quedó en marcha"
esperar
probar_en_marcha
# 7. Actualizar desde un paquete local: «servidor instalar» desde el .tar.gz de otra versión.
antes=$(pid)
salida=$("$trabajo/nuevo/tramtai/tramtai" servidor instalar --acepto-terminos 2>&1) || falla "actualizar desde el .tar.gz nuevo: $salida"
esperar
tramtai-servidor version | grep -qF "$version_nueva" || falla "tras actualizar: $(tramtai-servidor version)"
[ "$(pid)" != "$antes" ] || falla "actualizar no reinició el servicio con el programa nuevo"
# 8. Desinstalar, también los datos y la cuenta.
echo s | tramtai-servidor desinstalar --borrar-datos >/dev/null 2>&1 || falla "desinstalar --borrar-datos"
for f in /opt/tramtai /usr/local/bin/tramtai-servidor /etc/systemd/system/tramtai.service /var/lib/tramtai; do
	[ ! -e "$f" ] || falla "desinstalar deja $f"
done
! getent passwd tramtai >/dev/null || falla "desinstalar --borrar-datos deja la cuenta tramtai"
echo "secuencia del .tar.gz: bien"

echo "== 2. con el paquete, en una máquina limpia"
salida=$(DEBIAN_FRONTEND=noninteractive apt-get install -y "$deb_viejo" 2>&1) || falla "apt-get install del paquete: $salida"
echo "$salida" | grep -q "Para empezar: sudo tramtai-servidor instalar" || falla "el paquete no dice cómo empezar: $salida"
[ "$(systemctl is-active tramtai || :)" != active ] || falla "el paquete arrancó el servicio antes de aceptar los términos"
tramtai-servidor instalar --acepto-terminos >/dev/null 2>&1 || falla "tramtai-servidor instalar con el paquete"
comprobar_paquete
probar_en_marcha
# Actualizar el paquete: el postinst reinicia el servicio habilitado con el programa nuevo (try-restart).
antes=$(pid)
salida=$(DEBIAN_FRONTEND=noninteractive apt-get install -y "$deb_nuevo" 2>&1) || falla "actualizar el paquete: $salida"
esperar
[ "$(pid)" != "$antes" ] || falla "actualizar el paquete no reinició el servicio (try-restart)"
tramtai-servidor version | grep -qF "$version_nueva" || falla "tras actualizar el paquete: $(tramtai-servidor version)"
# Quitar el paquete: el prerm detiene y deshabilita el servicio; los datos se quedan.
DEBIAN_FRONTEND=noninteractive apt-get remove -y tramtai-servidor >/dev/null 2>&1 || falla "apt-get remove"
[ "$(systemctl is-active tramtai || :)" != active ] || falla "quitar el paquete deja el servicio en marcha"
for exe in /proc/[0-9]*/exe; do
	case "$(readlink "$exe" 2>/dev/null || :)" in
	/opt/tramtai/tramtai*) falla "queda un tramtai del servidor en marcha" ;; # también «(deleted)», ya sin el archivo
	esac
done
[ ! -e /etc/systemd/system/tramtai.service.d/10-tramtai.conf ] || falla "quitar el paquete deja el drop-in"
colgando=$(find /etc/systemd/system/multi-user.target.wants -name 'tramtai*' -xtype l)
[ -z "$colgando" ] || falla "tras quitar el paquete, enlaces colgando: $colgando"
[ -d /var/lib/tramtai/datos ] || falla "quitar el paquete borró los datos"
# Los datos, las claves y la cuenta, con la copia del .tar.gz (el paquete ya no está): la máquina queda limpia.
echo s | "$trabajo/viejo/tramtai/tramtai" servidor desinstalar --borrar-datos >/dev/null 2>&1 || falla "desinstalar --borrar-datos tras quitar el paquete"
[ ! -e /var/lib/tramtai ] || falla "desinstalar --borrar-datos deja /var/lib/tramtai"
echo "secuencia del paquete: bien"

echo "== 3. del .tar.gz al paquete"
"$trabajo/viejo/tramtai/tramtai" servidor instalar --acepto-terminos >/dev/null 2>&1 || falla "instalar desde el .tar.gz (antes del paquete)"
esperar
antes=$(pid)
salida=$(DEBIAN_FRONTEND=noninteractive apt-get install -y "$deb_viejo" 2>&1) || falla "apt-get install del paquete sobre el .tar.gz: $salida"
echo "$salida" | grep -q "Para empezar: sudo tramtai-servidor instalar" || falla "sobre el .tar.gz, el paquete no dice cómo empezar: $salida"
esperar
[ "$(pid)" != "$antes" ] || falla "el paquete no reinició el servicio del .tar.gz, que estaba habilitado (try-restart)"
salida=$(tramtai-servidor instalar --acepto-terminos 2>&1) || falla "tramtai-servidor instalar sobre el .tar.gz: $salida"
echo "$salida" | grep -q "quitado lo que dejó la instalación desde el .tar.gz" || falla "instalar no avisa de lo que quitó: $salida"
for f in /etc/systemd/system/tramtai.service /usr/local/bin/tramtai-servidor; do
	[ ! -e "$f" ] || falla "con el paquete queda $f del .tar.gz"
done
comprobar_paquete
echo "del .tar.gz al paquete: bien"
echo "el servidor de Linux se instala, atiende, guarda claves sin cortar peticiones, se actualiza y se quita con systemd"

