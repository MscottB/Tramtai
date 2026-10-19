# Descarga el uv fijado en scripts/uv.txt para un objetivo y deja uv.exe (o uv) en Destino; se para si el SHA-256 no
# coincide. Lo usan compilar.ps1 (instalador y portable), el CI y Publicar en Windows:
#   ./scripts/uv.ps1 -Objetivo x86_64-pc-windows-msvc -Destino bin
param(
    [Parameter(Mandatory)][string]$Objetivo,
    [Parameter(Mandatory)][string]$Destino
)
$ErrorActionPreference = 'Stop'
$filas = Get-Content (Join-Path $PSScriptRoot 'uv.txt') | Where-Object { $_ -and -not $_.StartsWith('#') } | ForEach-Object { , ($_ -split '\s+') }
$version = ($filas | Where-Object { $_[0] -eq 'version' })[1]
$fila = $filas | Where-Object { $_[0].StartsWith("$Objetivo.") }
if (-not $fila) { throw "objetivo de uv desconocido: $Objetivo" }
$archivo, $esperado = $fila[0], $fila[1]
$tmp = Join-Path ([IO.Path]::GetTempPath()) "tramtai-uv-$PID"
New-Item -ItemType Directory -Force $tmp | Out-Null
try {
    $ruta = Join-Path $tmp $archivo
    # Tres intentos: un corte de red al descargar no es un fallo del paquete.
    foreach ($intento in 1..3) {
        try { Invoke-WebRequest "https://github.com/astral-sh/uv/releases/download/$version/uv-$archivo" -OutFile $ruta; break }
        catch { if ($intento -eq 3) { throw "no se pudo descargar uv-${archivo}: $_" }; Start-Sleep -Seconds 10 }
    }
    $hash = (Get-FileHash $ruta -Algorithm SHA256).Hash.ToLower()
    if ($hash -ne $esperado) { throw "uv-$archivo no es el esperado (SHA-256 $hash)" }
    New-Item -ItemType Directory -Force $Destino | Out-Null
    if ($archivo.EndsWith('.zip')) {
        Expand-Archive $ruta (Join-Path $tmp 'x') -Force
        Copy-Item (Join-Path $tmp 'x\uv.exe') $Destino -Force
    } else {
        tar -xzf $ruta -C $tmp
        if ($LASTEXITCODE) { throw "no se pudo extraer uv-$archivo" }
        Copy-Item (Join-Path $tmp "uv-$Objetivo/uv") $Destino -Force
    }
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
