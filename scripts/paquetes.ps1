# Escribe los paquetes de Homebrew y winget de una versión publicada a partir de su SHA256SUMS.txt:
#   Casks/tramtai.rb    la app de macOS (.dmg); el repositorio hace de tap: brew tap mscottb/tramtai <url del repo>
#   Formula/tramtai.rb  solo la CLI (tar.gz) para macOS y Linux
#   paquetes/winget/    los manifiestos de winget (MscottB.Tramtai) para enviarlos a microsoft/winget-pkgs
# Lo ejecuta publicar.yml después de crear el release; a mano: scripts/paquetes.ps1 -Etiqueta v1.2.3+DDMMAAAA -Sumas SHA256SUMS.txt
param([Parameter(Mandatory)][string]$Etiqueta, [Parameter(Mandatory)][string]$Sumas, [string]$Fecha = (Get-Date -Format 'yyyy-MM-dd'))
$ErrorActionPreference = 'Stop'
$raiz = Join-Path $PSScriptRoot '..'
if ($Etiqueta -notmatch '^v(\d+\.\d+\.\d+)(\+\d{8})?$') { throw "etiqueta no válida: $Etiqueta" }
$numero = $Matches[1]
$base = "https://github.com/MscottB/Tramtai/releases/download/$([uri]::EscapeDataString($Etiqueta))"

$porArchivo = @{}
foreach ($linea in Get-Content $Sumas) {
    $suma, $nombre = $linea -split '\s+\*?', 2
    if ($nombre) { $porArchivo[$nombre.Trim()] = $suma.ToLower() }
}
function Archivo([string]$nombre) {
    if (-not $porArchivo.ContainsKey($nombre)) { throw "SHA256SUMS.txt no incluye $nombre" }
    [pscustomobject]@{ Url = "$base/$([uri]::EscapeDataString($nombre))"; Suma = $porArchivo[$nombre] }
}
function Escribir([string]$ruta, [string]$texto) {
    $completa = Join-Path $raiz $ruta
    New-Item -ItemType Directory -Force (Split-Path $completa) | Out-Null
    [IO.File]::WriteAllText($completa, $texto.Replace("`r`n", "`n") + "`n", [Text.UTF8Encoding]::new($false))
    Write-Host "escrito $ruta"
}

$descripcion = 'Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic'
$dmg = Archivo "Tramtai-$Etiqueta-macos.dmg"
Escribir 'Casks/tramtai.rb' @"
# Generado por scripts/paquetes.ps1 al publicar $Etiqueta; no lo edites a mano.
cask "tramtai" do
  version "$numero"
  sha256 "$($dmg.Suma)"

  url "$($dmg.Url)"
  name "Tramtai"
  desc "$descripcion"
  homepage "https://mscottb.github.io/Tramtai/"

  depends_on macos: ">= :big_sur"

  app "Tramtai.app"
  binary "#{appdir}/Tramtai.app/Contents/MacOS/tramtai"

  # La app no está firmada por un desarrollador de Apple: sin la marca de cuarentena macOS la abre sin bloquearla.
  postflight do
    system_command "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "#{appdir}/Tramtai.app"]
  end

  uninstall launchctl: "com.mscottb.tramtai",
            quit:      "com.mscottb.tramtai"

  zap trash: [
    "~/Library/Application Support/tramtai",
    "~/Library/LaunchAgents/com.mscottb.tramtai.plist",
  ]
end
"@

$cli = @{}
foreach ($so in 'macos', 'linux') { foreach ($arq in 'amd64', 'arm64') { $cli["$so-$arq"] = Archivo "tramtai-$Etiqueta-$so-$arq.tar.gz" } }
Escribir 'Formula/tramtai.rb' @"
# Generado por scripts/paquetes.ps1 al publicar $Etiqueta; no lo edites a mano.
class Tramtai < Formula
  desc "$descripcion"
  homepage "https://mscottb.github.io/Tramtai/"
  version "$numero"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "$($cli['macos-arm64'].Url)"
      sha256 "$($cli['macos-arm64'].Suma)"
    end
    on_intel do
      url "$($cli['macos-amd64'].Url)"
      sha256 "$($cli['macos-amd64'].Suma)"
    end
  end

  on_linux do
    on_arm do
      url "$($cli['linux-arm64'].Url)"
      sha256 "$($cli['linux-arm64'].Suma)"
    end
    on_intel do
      url "$($cli['linux-amd64'].Url)"
      sha256 "$($cli['linux-amd64'].Suma)"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
"@

$instalador = Archivo "Tramtai-$Etiqueta-instalador-windows-x64.exe"
$carpeta = "paquetes/winget/$numero"
function Cabecera([string]$tipo) { "# yaml-language-server: `$schema=https://aka.ms/winget-manifest.$tipo.1.10.0.schema.json`n# Generado por scripts/paquetes.ps1 al publicar $Etiqueta.`n" }
Escribir "$carpeta/MscottB.Tramtai.yaml" ((Cabecera 'version') + @"
PackageIdentifier: MscottB.Tramtai
PackageVersion: $numero
DefaultLocale: es-ES
ManifestType: version
ManifestVersion: 1.10.0
"@)
Escribir "$carpeta/MscottB.Tramtai.installer.yaml" ((Cabecera 'installer') + @"
PackageIdentifier: MscottB.Tramtai
PackageVersion: $numero
InstallerType: nullsoft
Scope: user
InstallModes:
- interactive
- silent
- silentWithProgress
UpgradeBehavior: install
ReleaseDate: $Fecha
AppsAndFeaturesEntries:
- DisplayName: Tramtai
  Publisher: Michael Scott B.
  DisplayVersion: $Etiqueta
  ProductCode: Tramtai
Installers:
- Architecture: x64
  InstallerUrl: $($instalador.Url)
  InstallerSha256: $($instalador.Suma.ToUpper())
ManifestType: installer
ManifestVersion: 1.10.0
"@)
$comun = @"
Publisher: Michael Scott B.
PublisherUrl: https://github.com/MscottB
PublisherSupportUrl: https://github.com/MscottB/Tramtai/issues
Author: Michael Scott B.
PackageName: Tramtai
PackageUrl: https://mscottb.github.io/Tramtai/
License: PolyForm Noncommercial 1.0.0
LicenseUrl: https://github.com/MscottB/Tramtai/blob/main/LICENSE.md
Tags:
- ai
- anthropic
- api
- claude
- codex
- gemini
- llm
- openai
ReleaseNotesUrl: https://github.com/MscottB/Tramtai/releases/tag/$([uri]::EscapeDataString($Etiqueta))
"@
Escribir "$carpeta/MscottB.Tramtai.locale.es-ES.yaml" ((Cabecera 'defaultLocale') + @"
PackageIdentifier: MscottB.Tramtai
PackageVersion: $numero
PackageLocale: es-ES
$comun
Moniker: tramtai
ShortDescription: $descripcion.
Description: Tramtai convierte Claude Code, Codex, Antigravity y la API de Gemini en una API compatible con OpenAI y Anthropic que funciona en tu propio equipo, con app de escritorio, Web UI, terminal interactiva y línea de órdenes.
ManifestType: defaultLocale
ManifestVersion: 1.10.0
"@)
Escribir "$carpeta/MscottB.Tramtai.locale.en-US.yaml" ((Cabecera 'locale') + @"
PackageIdentifier: MscottB.Tramtai
PackageVersion: $numero
PackageLocale: en-US
$comun
ShortDescription: Your AI agents (Claude Code, Codex, the Gemini API) as a local OpenAI- and Anthropic-compatible API.
Description: Tramtai turns Claude Code, Codex, Antigravity and the Gemini API into an OpenAI- and Anthropic-compatible API that runs on your own computer, with a desktop app, Web UI, interactive terminal and CLI.
ManifestType: locale
ManifestVersion: 1.10.0
"@)
