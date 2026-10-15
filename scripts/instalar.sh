#!/bin/sh
# Instala la última versión de Tramtai (servicio, API, CLI y Web UI) en Linux o macOS:
#
#   curl -fsSL https://raw.githubusercontent.com/MscottB/Tramtai/main/scripts/instalar.sh | sh
#
# Descarga el .tar.gz de tu sistema desde el último release, comprueba su SHA-256 con SHA256SUMS.txt y deja el
# ejecutable en ~/.local/bin (o en la carpeta de TRAMTAI_DESTINO). Para actualizar, vuelve a ejecutarlo.
set -eu

repo=MscottB/Tramtai
destino=${TRAMTAI_DESTINO:-$HOME/.local/bin}

case "$(uname -s)" in
Linux) so=linux ;;
Darwin) so=macos ;;
*) echo "Sistema no soportado: $(uname -s). En Windows usa el instalador de https://github.com/$repo/releases/latest" >&2; exit 1 ;;
esac
case "$(uname -m)" in
x86_64 | amd64) arq=amd64 ;;
arm64 | aarch64) arq=arm64 ;;
*) echo "Arquitectura no soportada: $(uname -m)" >&2; exit 1 ;;
esac

etiqueta=$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)
if [ -z "$etiqueta" ]; then
	echo "No se pudo consultar la última versión de Tramtai." >&2
	exit 1
fi
archivo="tramtai-$etiqueta-$so-$arq.tar.gz"
# El + de la versión (vX.Y.Z+DDMMAAAA) va codificado en la dirección de descarga.
codificar() { printf %s "$1" | sed 's/+/%2B/g'; }
base="https://github.com/$repo/releases/download/$(codificar "$etiqueta")"

temporal=$(mktemp -d)
trap 'rm -rf "$temporal"' EXIT
echo "Descargando Tramtai $etiqueta para $so/$arq…"
curl -fsSL -o "$temporal/$archivo" "$base/$(codificar "$archivo")"
curl -fsSL -o "$temporal/SHA256SUMS.txt" "$base/SHA256SUMS.txt"

esperada=$(awk -v a="$archivo" '$2 == a { print $1 }' "$temporal/SHA256SUMS.txt")
if command -v sha256sum >/dev/null 2>&1; then
	real=$(sha256sum "$temporal/$archivo" | cut -d ' ' -f 1)
else
	real=$(shasum -a 256 "$temporal/$archivo" | cut -d ' ' -f 1)
fi
if [ -z "$esperada" ] || [ "$esperada" != "$real" ]; then
	echo "La suma SHA-256 de $archivo no coincide: no se instala nada." >&2
	exit 1
fi

tar -xzf "$temporal/$archivo" -C "$temporal"
mkdir -p "$destino"
cp "$temporal/tramtai/tramtai" "$destino/tramtai"
chmod 755 "$destino/tramtai"
echo "Tramtai $etiqueta instalado en $destino/tramtai (suma SHA-256 comprobada)."

case ":$PATH:" in
*":$destino:"*) ;;
*) echo "Añade $destino a tu PATH, por ejemplo: echo 'export PATH=\"$destino:\$PATH\"' >> ~/.profile" ;;
esac
echo "Para empezar: tramtai abrir"
