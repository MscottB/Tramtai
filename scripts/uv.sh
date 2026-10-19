#!/bin/sh
# Descarga el uv fijado en scripts/uv.txt para un objetivo y deja el binario en DESTINO/uv; se para si el SHA-256 no
# coincide. Lo usan el empaquetado de Linux y macOS, el CI y Publicar:
#   sh scripts/uv.sh aarch64-apple-darwin paquete/
set -eu
objetivo=$1
destino=$2
datos="$(dirname "$0")/uv.txt"
version=$(awk '$1 == "version" { print $2 }' "$datos")
archivo=$(awk -v o="$objetivo." 'index($1, o) == 1 { print $1 }' "$datos")
esperado=$(awk -v o="$objetivo." 'index($1, o) == 1 { print $2 }' "$datos")
[ -n "$archivo" ] || { echo "objetivo de uv desconocido: $objetivo" >&2; exit 1; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
# Tres intentos: un corte de red al descargar no es un fallo del paquete.
for intento in 1 2 3; do
	curl -fsSL -o "$tmp/$archivo" "https://github.com/astral-sh/uv/releases/download/$version/uv-$archivo" && break
	[ "$intento" = 3 ] && { echo "no se pudo descargar uv-$archivo" >&2; exit 1; }
	sleep 10
done
if command -v sha256sum >/dev/null 2>&1; then
	real=$(sha256sum "$tmp/$archivo" | cut -d ' ' -f 1)
else
	real=$(shasum -a 256 "$tmp/$archivo" | cut -d ' ' -f 1)
fi
[ "$real" = "$esperado" ] || { echo "uv-$archivo no es el esperado (SHA-256 $real)" >&2; exit 1; }
tar -xzf "$tmp/$archivo" -C "$tmp"
mkdir -p "$destino"
cp "$tmp/uv-$objetivo/uv" "$destino/uv"
chmod 755 "$destino/uv"
