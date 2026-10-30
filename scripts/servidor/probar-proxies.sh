#!/bin/sh
# Prueba los ejemplos de docs/servidor/proxies (Caddy, nginx, Apache, Traefik y HAProxy) delante de un Tramtai en modo
# servidor, en contenedores (docker-compose.proxies.yml; necesita Docker y Go). Lo usa el job «Proxies» del CI:
#   sh scripts/servidor/probar-proxies.sh
# Por cada proxy comprueba: solo TLS 1.3; una sola cabecera HSTS; sin sesión, 401 en /ui/; el inicio de sesión por el
# proxy (con un X-Forwarded-For falso del cliente, que no cuenta); el flujo SSE de una petición al Claude simulado en al
# menos 3 trozos separados en el tiempo; la dirección del cliente en el registro de Tramtai, y que un X-Forwarded-For
# enviado directamente al puerto de Tramtai, saltándose el proxy, no cambia la dirección registrada.
set -eu
cd "$(dirname "$0")/../.."

falla() {
	echo "FALLA: $*" >&2
	exit 1
}

TRAMTAI_PRUEBA=$(mktemp -d)
export TRAMTAI_PRUEBA COMPOSE_FILE=scripts/servidor/docker-compose.proxies.yml
trap 'docker compose down -t 1 >/dev/null 2>&1 || :; rm -rf "$TRAMTAI_PRUEBA"' EXIT
mkdir -p "$TRAMTAI_PRUEBA/bin" "$TRAMTAI_PRUEBA/tls"
CGO_ENABLED=0 GOOS=linux go build -o "$TRAMTAI_PRUEBA/bin/tramtai" ./cmd/tramtai
CGO_ENABLED=0 GOOS=linux go build -o "$TRAMTAI_PRUEBA/bin/claude" ./scripts/servidor/claude-simulado
# Certificado de prueba de tramtai.example.com (ECDSA P-256), que el cliente acepta como raíz.
(cd "$TRAMTAI_PRUEBA/tls" && go run "$(go env GOROOT)/src/crypto/tls/generate_cert.go" --host tramtai.example.com --ecdsa-curve P256 --ca >/dev/null 2>&1)
mv "$TRAMTAI_PRUEBA/tls/cert.pem" "$TRAMTAI_PRUEBA/tls/fullchain.pem"
mv "$TRAMTAI_PRUEBA/tls/key.pem" "$TRAMTAI_PRUEBA/tls/privkey.pem"
cat "$TRAMTAI_PRUEBA/tls/fullchain.pem" "$TRAMTAI_PRUEBA/tls/privkey.pem" >"$TRAMTAI_PRUEBA/tls/tramtai.pem"
chmod 644 "$TRAMTAI_PRUEBA/tls/"* # solo de prueba: HAProxy no lee como root

en_tramtai() { docker compose exec -T tramtai "$@"; }
curl_cliente() { docker compose exec -T cliente curl -sS --cacert /tls/fullchain.pem --resolve tramtai.example.com:443:127.0.0.1 "$@"; }
registro() { en_tramtai cat /var/lib/tramtai/datos/servicio.log; }

docker compose up -d --quiet-pull tramtai cliente
# La cuenta del servicio, como la crea «tramtai-servidor instalar»: las órdenes de administrador (root) le dejan los
# datos a su nombre y el servicio corre con ella (como root no arranca, R36).
en_tramtai useradd --system --home /var/lib/tramtai --shell /usr/sbin/nologin tramtai
# Lo de la parte de Tramtai de los ejemplos: el dominio y el proxy en este mismo equipo.
en_tramtai sh -c 'mkdir -p /var/lib/tramtai && cat >/var/lib/tramtai/config.yaml' <<'FIN'
servicio:
  host: 127.0.0.1
  dominios: [tramtai.example.com]
  proxy:
    confiables: [127.0.0.1/32]
FIN
contrasena=contrasena-de-prueba-del-ci
en_tramtai /opt/tramtai/tramtai terminos aceptar >/dev/null
printf '%s\n%s\n' "$contrasena" "$contrasena" | en_tramtai /opt/tramtai/tramtai administrador contraseña >/dev/null 2>&1
dueno=$(en_tramtai stat -c %U /var/lib/tramtai/datos)
[ "$dueno" = tramtai ] || falla "tras las órdenes de administrador, datos/ es de $dueno y no de la cuenta del servicio"
docker compose exec -d -u tramtai tramtai /opt/tramtai/tramtai servicio iniciar --registro /var/lib/tramtai/datos/servicio.log
activo() { en_tramtai /opt/tramtai/tramtai servicio estado 2>/dev/null | grep -q "^servicio activo"; }
for _ in $(seq 30); do
	activo && break
	sleep 1
done
activo || { registro || :; falla "Tramtai no arrancó"; }
clave=$(en_tramtai /opt/tramtai/tramtai acceso crear prueba --agente claude | tail -n 1)
case $clave in tt_*) ;; *) falla "acceso crear no dio una clave: $clave" ;; esac

numero=10
for proxy in caddy nginx apache traefik haproxy; do
	numero=$((numero + 1))
	cliente=127.0.0.$numero
	echo "== $proxy (el cliente sale desde $cliente)"
	docker compose up -d --quiet-pull "$proxy"
	listo=
	for _ in $(seq 30); do
		if curl_cliente -o /dev/null https://tramtai.example.com/healthz 2>/dev/null; then
			listo=1
			break
		fi
		sleep 1
	done
	[ -n "$listo" ] || { docker compose logs "$proxy" || :; falla "$proxy no responde por HTTPS"; }

	# Solo TLS 1.3: con TLS 1.2 como máximo no hay conexión.
	if curl_cliente --tls-max 1.2 -o /dev/null https://tramtai.example.com/healthz 2>/dev/null; then
		falla "$proxy acepta TLS 1.2"
	fi
	# HSTS lo pone Tramtai (llega X-Forwarded-Proto: https); el proxy no añade otro.
	hsts=$(curl_cliente -D - -o /dev/null https://tramtai.example.com/healthz | grep -ci '^strict-transport-security:' || :)
	[ "$hsts" = 1 ] || falla "$proxy: $hsts cabeceras Strict-Transport-Security (se esperaba una)"
	codigo=$(curl_cliente -o /dev/null -w '%{http_code}' https://tramtai.example.com/ui/acerca || :)
	[ "$codigo" = 401 ] || falla "$proxy: sin sesión, /ui/acerca da $codigo y no 401"

	# Iniciar sesión por el proxy, con un X-Forwarded-For falso del cliente que el proxy debe dejar atrás.
	docker compose exec -T cliente rm -f /tmp/galletas
	codigo=$(curl_cliente --interface "$cliente" -c /tmp/galletas -o /dev/null -w '%{http_code}' -H 'X-Forwarded-For: 203.0.113.7' \
		-H 'Content-Type: application/json' -d "{\"contrasena\":\"$contrasena\",\"codigo\":\"\"}" https://tramtai.example.com/ui/acceder || :)
	[ "$codigo" = 200 ] || falla "$proxy: el inicio de sesión da $codigo"
	codigo=$(curl_cliente -b /tmp/galletas -o /dev/null -w '%{http_code}' https://tramtai.example.com/ui/acerca || :)
	[ "$codigo" = 200 ] || falla "$proxy: con la sesión, /ui/acerca da $codigo"
	registro | grep -q "acceso correcto del administrador desde $cliente\$" ||
		{ registro | grep "web ui:" >&2 || :; falla "$proxy: el registro no tiene la dirección del cliente ($cliente)"; }

	# El flujo SSE: los trozos del Claude simulado (uno por segundo) llegan separados, no juntos al final.
	tiempos=$(curl_cliente -N --interface "$cliente" -H "Authorization: Bearer $clave" -H 'Content-Type: application/json' \
		-d '{"model":"claude","stream":true,"messages":[{"role":"user","content":"hola"}]}' \
		https://tramtai.example.com/p/prueba/openai/v1/chat/completions |
		while IFS= read -r linea; do
			case $linea in data:*'"content"'*) date +%s.%N ;; esac
		done)
	trozos=$(echo "$tiempos" | awk 'NF { if (n == 0 || $1 - antes > 0.4) n++; antes = $1 } END { print n + 0 }')
	[ "$trozos" -ge 3 ] || falla "$proxy: el flujo SSE llegó en $trozos trozos separados (se esperaban 3 o más): $tiempos"

	# Saltándose el proxy, desde una dirección que no es de confianza: las cabeceras reenviadas no cuentan.
	codigo=$(docker compose exec -T cliente curl -sS --interface 127.0.0.2 -o /dev/null -w '%{http_code}' -H 'Host: tramtai.example.com' \
		-H 'X-Forwarded-For: 203.0.113.7' -H 'X-Forwarded-Proto: https' -H 'Content-Type: application/json' \
		-d "{\"contrasena\":\"$contrasena\",\"codigo\":\"\"}" http://127.0.0.1:17373/ui/acceder || :)
	[ "$codigo" != 200 ] || falla "$proxy: saltándose el proxy se entra por HTTP con un X-Forwarded-Proto falso"
	[ "$(registro | grep -c 'intento de acceso sin HTTPS desde 127.0.0.2$')" = $((numero - 10)) ] || falla "$proxy: el registro no anota 127.0.0.2 al saltarse el proxy"
	if registro | grep -q 203.0.113.7; then
		falla "$proxy: el registro anota la dirección falsa de X-Forwarded-For"
	fi
	docker compose rm -sf "$proxy" >/dev/null
	echo "$proxy: TLS 1.3, una cabecera HSTS, 401 sin sesión, inicio de sesión, $trozos trozos SSE y la dirección del cliente en el registro"
done
echo "los cinco ejemplos de proxy funcionan delante de Tramtai"
