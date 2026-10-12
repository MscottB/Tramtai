# Calcula la versión vX.Y.Z+DDMMAAAA a partir del archivo VERSION y la fecha de hoy.
# Uso: scripts/version.ps1             -> imprime la versión
#      scripts/version.ps1 -Etiquetar  -> además crea la etiqueta git anotada (no la sube)
param([switch]$Etiquetar)
$ErrorActionPreference = 'Stop'

$base = (Get-Content (Join-Path $PSScriptRoot '..\VERSION') -Raw).Trim()
if ($base -notmatch '^\d+\.\d+\.\d+$') { throw "VERSION debe tener la forma X.Y.Z (tiene '$base')" }
$version = "v$base+$(Get-Date -Format ddMMyyyy)"

if ($Etiquetar) {
    if (git status --porcelain) { throw 'Hay cambios sin confirmar: haz commit antes de etiquetar.' }
    if (git tag --list "v$base+*") { throw "Ya existe una etiqueta para $base; sube el número en VERSION." }
    git tag -a $version -m "Tramtai $version"
    if ($LASTEXITCODE) { throw 'git tag falló' }
    Write-Host "Etiqueta $version creada. Súbela con: git push origin $version"
}
$version
