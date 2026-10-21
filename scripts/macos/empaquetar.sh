#!/bin/sh
# Compila Tramtai para macOS (Intel y Apple Silicon en un solo binario universal) y genera Tramtai.app dentro de
# dist/Tramtai-<versión>-macos.dmg. Lo usan el CI y la publicación en un runner de macOS:
#   scripts/macos/empaquetar.sh v0.28.0+16102026 [arquitecturas]
# Sin arquitecturas compila "arm64 amd64" (universal); el CI pasa solo la del runner, que tarda la mitad.
# La app no está firmada por un desarrollador de Apple, solo con firma ad-hoc (la mínima para ejecutarse en Apple
# Silicon): al abrirla por primera vez hay que autorizarla en Ajustes del Sistema → Privacidad y seguridad.
set -eu
version=$1
arquitecturas=${2:-arm64 amd64}
cd "$(dirname "$0")/../.."
numero=$(printf %s "${version#v}" | cut -d + -f 1)
ldflags="-s -w -X tramtai/internal/version.Version=$version"
app=paquete/Tramtai.app
rm -rf paquete
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources" dist
for arq in $arquitecturas; do
	CGO_ENABLED=0 GOOS=darwin GOARCH=$arq go build -trimpath -ldflags "$ldflags" -o "paquete/tramtai-$arq" ./cmd/tramtai
	(cd escritorio && CGO_ENABLED=1 GOOS=darwin GOARCH=$arq go build -trimpath -ldflags "$ldflags" -o "../paquete/escritorio-$arq" .)
done
# shellcheck disable=SC2046 # una ruta por arquitectura
lipo -create -output "$app/Contents/MacOS/tramtai" $(for arq in $arquitecturas; do echo "paquete/tramtai-$arq"; done)
# shellcheck disable=SC2046
lipo -create -output "$app/Contents/MacOS/tramtai-escritorio" $(for arq in $arquitecturas; do echo "paquete/escritorio-$arq"; done)
# uv (fijado en scripts/uv.txt) prepara el conversor de archivos; universal como el resto.
for arq in $arquitecturas; do
	case $arq in
	arm64) objetivo=aarch64-apple-darwin ;;
	amd64) objetivo=x86_64-apple-darwin ;;
	esac
	sh scripts/uv.sh "$objetivo" "paquete/uv-$arq"
done
# shellcheck disable=SC2046
lipo -create -output "$app/Contents/MacOS/uv" $(for arq in $arquitecturas; do echo "paquete/uv-$arq/uv"; done)
cp escritorio/icono/tramtai.icns "$app/Contents/Resources/tramtai.icns"
cp LICENSE.md "$app/Contents/Resources/LICENCIA.md"
cat >"$app/Contents/Info.plist" <<FIN
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleName</key><string>Tramtai</string>
	<key>CFBundleDisplayName</key><string>Tramtai</string>
	<key>CFBundleIdentifier</key><string>com.mscottb.tramtai</string>
	<key>CFBundleExecutable</key><string>tramtai-escritorio</string>
	<key>CFBundleIconFile</key><string>tramtai</string>
	<key>CFBundleShortVersionString</key><string>$numero</string>
	<key>CFBundleVersion</key><string>$numero</string>
	<key>LSMinimumSystemVersion</key><string>11.0</string>
	<key>NSHighResolutionCapable</key><true/>
	<key>NSHumanReadableCopyright</key><string>© 2026 Michael Scott B. · PolyForm Noncommercial 1.0.0</string>
</dict>
</plist>
FIN
codesign --force --deep --sign - "$app"
codesign --verify --deep "$app"

# El .dmg lleva la app y un acceso a Aplicaciones para arrastrarla.
disco=paquete/disco
mkdir -p "$disco"
cp -R "$app" "$disco/"
ln -s /Applications "$disco/Applications"
# Con solo -srcfolder, hdiutil calcula el tamaño de la imagen y con la app universal a veces se queda corto («No space
# left on device»): se le da el de la carpeta, con un 20 % más y 20 MB para el sistema de archivos.
megas=$(du -sm "$disco" | cut -f1)
hdiutil create -volname Tramtai -srcfolder "$disco" -size "$((megas + megas / 5 + 20))m" -ov -format UDZO "dist/Tramtai-$version-macos.dmg"
ls -l dist
