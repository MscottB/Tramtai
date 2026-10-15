#!/bin/sh
# Compila Tramtai para Linux (servicio y CLI sin CGO; app de escritorio con GTK 4 y WebKitGTK 6.0) y genera los
# paquetes .deb y .rpm en dist/. Lo usan el CI y la publicación:
#   scripts/linux/empaquetar.sh v0.28.0+16102026 amd64
set -eu
version=$1
arq=$2
cd "$(dirname "$0")/../.."
mkdir -p paquete dist
ldflags="-s -w -X tramtai/internal/version.Version=$version"
CGO_ENABLED=0 go build -trimpath -ldflags "$ldflags" -o paquete/tramtai ./cmd/tramtai
(cd escritorio && go build -trimpath -ldflags "$ldflags" -o ../paquete/tramtai-escritorio .)
go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.41.0
nfpm="$(go env GOPATH)/bin/nfpm"
export VERSION="${version#v}" ARCH="$arq"
"$nfpm" package -f scripts/linux/nfpm.yaml -p deb -t "dist/Tramtai-$version-linux-$arq.deb"
"$nfpm" package -f scripts/linux/nfpm.yaml -p rpm -t "dist/Tramtai-$version-linux-$arq.rpm"
ls -l dist
