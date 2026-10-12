# Compila bin/tramtai.exe con la versión vX.Y.Z+DDMMAAAA incrustada.
$ErrorActionPreference = 'Stop'
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { $env:Path = "C:\Program Files\Go\bin;$env:Path" }

$raiz = Join-Path $PSScriptRoot '..'
$version = & (Join-Path $PSScriptRoot 'version.ps1')
Push-Location $raiz
try {
    go build -trimpath -ldflags "-s -w -X tramtai/internal/version.Version=$version" -o bin/tramtai.exe ./cmd/tramtai
    if ($LASTEXITCODE) { throw 'go build falló' }
    Write-Host "bin/tramtai.exe $version"
} finally { Pop-Location }
