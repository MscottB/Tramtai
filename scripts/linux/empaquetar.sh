#!/bin/sh
# Compila Tramtai para Linux (servicio y CLI sin CGO; app de escritorio con GTK 4 y WebKitGTK 6.0) y genera los
# paquetes .deb y .rpm en dist/, los de escritorio y los del servidor. Lo usan el CI y la publicación:
#   scripts/linux/empaquetar.sh v0.28.0+16102026 amd64
# Con «servidor» de tercer argumento solo genera los del servidor (sin GTK): lo usa el job «Servidor (Linux)» del CI.
set -eu
version=$1
arq=$2
solo=${3:-}
cd "$(dirname "$0")/../.."
mkdir -p paquete dist
ldflags="-s -w -X tramtai/internal/version.Version=$version"
CGO_ENABLED=0 go build -trimpath -ldflags "$ldflags" -o paquete/tramtai ./cmd/tramtai
[ "$solo" = servidor ] || (cd escritorio && go build -trimpath -ldflags "$ldflags" -o ../paquete/tramtai-escritorio .)
case $arq in
amd64) objetivo=x86_64-unknown-linux-musl ;;
arm64) objetivo=aarch64-unknown-linux-musl ;;
*) echo "arquitectura sin uv fijado: $arq" >&2 && exit 1 ;;
esac
sh scripts/uv.sh "$objetivo" paquete
go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.41.0
nfpm="$(go env GOPATH)/bin/nfpm"
export VERSION="${version#v}" ARCH="$arq"
if [ "$solo" != servidor ]; then
	"$nfpm" package -f scripts/linux/nfpm.yaml -p deb -t "dist/Tramtai-$version-linux-$arq.deb"
	"$nfpm" package -f scripts/linux/nfpm.yaml -p rpm -t "dist/Tramtai-$version-linux-$arq.rpm"
fi
: >paquete/.paquete # vacío: marca que el programa del servidor lo puso el paquete
"$nfpm" package -f scripts/linux/nfpm-servidor.yaml -p deb -t "dist/Tramtai-$version-servidor-linux-$arq.deb"
"$nfpm" package -f scripts/linux/nfpm-servidor.yaml -p rpm -t "dist/Tramtai-$version-servidor-linux-$arq.rpm"
ls -l dist
