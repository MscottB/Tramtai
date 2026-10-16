# Compila bin/tramtai.exe y, en Windows, bin/tramtai-escritorio.exe con la versión vX.Y.Z+DDMMAAAA incrustada.
# La Web UI ya compilada está en internal/recursos/web; tras cambiar web/ ejecuta antes: cd web; npm run build
# Con -Instalador deja además en dist/ el instalador de Windows, el .zip portable y SHA256SUMS.txt (lo usa el CI).
param([switch]$Instalador)
$ErrorActionPreference = 'Stop'
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { $env:Path = "C:\Program Files\Go\bin;$env:Path" }

$raiz = Join-Path $PSScriptRoot '..'
$version = & (Join-Path $PSScriptRoot 'version.ps1')

# Recursos de Windows (icono y Propiedades → Detalles con la versión): wails3 genera un .syso por ejecutable que
# go build incorpora. Se borran al terminar porque llevan la versión de esta compilación.
function Escribir-Recursos([string]$destino, [string]$descripcion) {
    $wails = Get-Command wails3 -ErrorAction SilentlyContinue
    if (-not $wails) { $wails = Get-Command (Join-Path $env:USERPROFILE 'go\bin\wails3.exe') -ErrorAction SilentlyContinue }
    if (-not $wails) { Write-Warning 'wails3 no está instalado: el .exe queda sin icono ni versión en sus propiedades'; return }
    $numero = ($version -replace '^v', '') -replace '\+.*$', ''
    $info = [ordered]@{
        fixed = @{ file_version = "$numero.0" }
        info  = @{ '0000' = [ordered]@{ ProductVersion = $version; FileVersion = $version; CompanyName = 'Tramtai'
                FileDescription = $descripcion; ProductName = 'Tramtai'; LegalCopyright = '(c) Tramtai'
                Comments = 'Pasarela local de agentes CLI' } }
    }
    $archivoInfo = Join-Path ([IO.Path]::GetTempPath()) "tramtai-info-$PID.json"
    $info | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 $archivoInfo
    # El manifiesto (obligatorio para wails3) declara asInvoker y DPI por monitor: no pide permisos de administrador.
    $argumentos = @('generate', 'syso', '-arch', 'amd64', '-icon', 'escritorio/icono/tramtai.ico', '-info', $archivoInfo,
        '-manifest', 'escritorio/icono/tramtai.exe.manifest', '-out', $destino)
    & $wails.Source @argumentos | Out-Null
    $codigo = $LASTEXITCODE
    Remove-Item $archivoInfo -ErrorAction SilentlyContinue
    if ($codigo) { throw "wails3 generate syso falló para $destino" }
}

Push-Location $raiz
try {
    $esWindows = $IsWindows -or $env:OS -eq 'Windows_NT'
    if ($esWindows) { Escribir-Recursos 'cmd/tramtai/rsrc_windows_amd64.syso' 'Tramtai: servicio, CLI, terminal y Web UI' }
    go build -trimpath -ldflags "-s -w -X tramtai/internal/version.Version=$version" -o bin/tramtai.exe ./cmd/tramtai
    $codigo = $LASTEXITCODE
    Remove-Item cmd/tramtai/rsrc_windows_amd64.syso -ErrorAction SilentlyContinue
    if ($codigo) { throw 'go build falló' }
    Write-Host "bin/tramtai.exe $version"
    # Aplicación de escritorio (Wails v3, módulo aparte en escritorio/); en Windows no necesita CGO.
    if ($esWindows) {
        Escribir-Recursos 'escritorio/rsrc_windows_amd64.syso' 'Tramtai: aplicación de escritorio'
        Push-Location escritorio
        try {
            go build -trimpath -ldflags "-s -w -H windowsgui -X tramtai/internal/version.Version=$version" -o ../bin/tramtai-escritorio.exe .
            $codigo = $LASTEXITCODE
            Remove-Item rsrc_windows_amd64.syso -ErrorAction SilentlyContinue
            if ($codigo) { throw 'go build de la aplicación de escritorio falló' }
            Write-Host "bin/tramtai-escritorio.exe $version"
        } finally { Pop-Location }
    }
} finally { Pop-Location }

if ($Instalador) {
    $makensis = Get-Command makensis -ErrorAction SilentlyContinue
    if (-not $makensis) { $makensis = Get-Command 'C:\Program Files (x86)\NSIS\makensis.exe' -ErrorAction Stop }
    $dist = Join-Path $raiz 'dist'
    New-Item -ItemType Directory -Force $dist | Out-Null
    Get-ChildItem $dist -File | Remove-Item
    # Licencia y términos en RTF para las páginas del instalador (acentos y formato correctos).
    Push-Location $raiz
    try { go run ./scripts/rtf; if ($LASTEXITCODE) { throw 'scripts/rtf falló' } } finally { Pop-Location }
    $archivoInstalador = Join-Path $dist "Tramtai-$version-instalador-windows-x64.exe"
    & $makensis.Source /V2 /INPUTCHARSET UTF8 "/DVERSION=$version" "/DSALIDA=$archivoInstalador" (Join-Path $raiz 'scripts\instalador\tramtai.nsi')
    if ($LASTEXITCODE) { throw 'makensis falló' }
    # Portable: la app y el servicio en un .zip, para usar Tramtai sin instalarlo (no se actualiza solo).
    $portable = Join-Path ([IO.Path]::GetTempPath()) "tramtai-portable-$PID"
    New-Item -ItemType Directory -Force $portable | Out-Null
    Copy-Item (Join-Path $raiz 'bin\tramtai.exe'), (Join-Path $raiz 'bin\tramtai-escritorio.exe') $portable
    Copy-Item (Join-Path $raiz 'LICENSE.md') (Join-Path $portable 'LICENCIA.md')
    Compress-Archive -Path (Join-Path $portable '*') -DestinationPath (Join-Path $dist "Tramtai-$version-portable-windows-x64.zip") -Force
    Remove-Item -Recurse -Force $portable
    Get-ChildItem $dist -File | Where-Object Name -ne 'SHA256SUMS.txt' | ForEach-Object { "$((Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLower())  $($_.Name)" } |
        Set-Content -Encoding ascii (Join-Path $dist 'SHA256SUMS.txt')
    Get-ChildItem $dist -File | ForEach-Object { Write-Host "dist/$($_.Name)" }
}
