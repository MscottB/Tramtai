# Compila bin/tramtai.exe y, en Windows, bin/tramtai-escritorio.exe con la versión vX.Y.Z+DDMMAAAA incrustada.
# La Web UI ya compilada está en internal/recursos/web; tras cambiar web/ ejecuta antes: cd web; npm run build
$ErrorActionPreference = 'Stop'
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { $env:Path = "C:\Program Files\Go\bin;$env:Path" }

$raiz = Join-Path $PSScriptRoot '..'
$version = & (Join-Path $PSScriptRoot 'version.ps1')
Push-Location $raiz
try {
    go build -trimpath -ldflags "-s -w -X tramtai/internal/version.Version=$version" -o bin/tramtai.exe ./cmd/tramtai
    if ($LASTEXITCODE) { throw 'go build falló' }
    Write-Host "bin/tramtai.exe $version"
    # Aplicación de escritorio (Wails v3, módulo aparte en escritorio/); en Windows no necesita CGO.
    if ($IsWindows -or $env:OS -eq 'Windows_NT') {
        Push-Location escritorio
        try {
            go build -trimpath -ldflags "-s -w -H windowsgui -X tramtai/internal/version.Version=$version" -o ../bin/tramtai-escritorio.exe .
            if ($LASTEXITCODE) { throw 'go build de la aplicación de escritorio falló' }
            Write-Host "bin/tramtai-escritorio.exe $version"
        } finally { Pop-Location }
    }
} finally { Pop-Location }
