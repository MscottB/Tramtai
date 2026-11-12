# M0 de la v0.60 (docs/equipo/releases/v0.60-preparar-agentes/m0.md): instala un CLI con su instalador oficial en una
# carpeta de usuario temporal (USERPROFILE, HOME, APPDATA, LOCALAPPDATA y TEMP apuntan ahí) e imprime dónde quedó, su
# --version, los archivos que dejó y si cambió el PATH de usuario del registro. Con -Actualizar ejecuta después su orden
# de actualizar; con -ProbarSesion, lanza su inicio de sesión con tuberías, guarda lo que imprime en 25 s y lo cierra; y
# con -Desinstalar lo quita como lo haría la receta. Lo usa el job agentes-compatibilidad.yml:
#   pwsh -NoProfile -File scripts/agentes/probar-instalacion.ps1 -Agente claude [-Version 2.1.311] [-Actualizar] [-ProbarSesion] [-Desinstalar]
# Copilot va con winget (la orden oficial en Windows) o, con -Descarga, con su descarga directa de GitHub Releases
# (copilot-win32-x64.zip y su SHA256SUMS.txt) en la carpeta donde lo deja su MSI.
#
# El registro (PATH de usuario, HKCU) no se puede aislar: Codex (install.ps1) y Copilot (winget) lo cambian, así que
# fuera del CI se niegan salvo con -CambiarRegistro. Antigravity, fuera del CI, va con --skip-path --skip-aliases.
# -SinAislar deja las carpetas de la cuenta tal cual: es para la cuenta virtual de un servicio de prueba, en el CI.
param(
    [Parameter(Mandatory)][ValidateSet('claude', 'codex', 'copilot', 'antigravity')][string]$Agente,
    [string]$Version = '',
    [string]$Carpeta = (Join-Path ([IO.Path]::GetTempPath()) "m0-agentes-$Agente-$PID"),
    [switch]$Actualizar,
    [switch]$ProbarSesion,
    [switch]$CambiarRegistro,
    [switch]$SinAislar,
    [switch]$Desinstalar,
    [switch]$Descarga
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
# -SinAislar instala, escribe .claude\settings.json y desinstala en las carpetas reales de la cuenta: solo se admite con
# la cuenta virtual de un servicio de prueba, nunca con la de una persona (ni aunque CI esté puesta).
$identidad = [Security.Principal.WindowsIdentity]::GetCurrent().Name
if ($SinAislar -and $identidad -notlike 'NT SERVICE\*') {
    throw "-SinAislar usa las carpetas reales de la cuenta: solo con la cuenta virtual de un servicio de prueba (NT SERVICE\…), no con $identidad"
}
if ($Descarga -and $Agente -ne 'copilot') { throw "-Descarga solo vale para copilot (su descarga directa), no para $Agente" }
$deVerdad = $env:CI -eq 'true' -or $SinAislar
if (-not $deVerdad -and -not $CambiarRegistro -and $Agente -in 'codex', 'copilot') {
    throw "$Agente cambia el PATH de usuario del registro al instalarse: fuera del CI hace falta -CambiarRegistro"
}

$casa = if ($SinAislar) { $env:USERPROFILE } else { [IO.Path]::GetFullPath($Carpeta) }
$entorno = @{
    CODEX_NON_INTERACTIVE = '1'          # sin «¿Iniciar Codex ahora?» ni otras preguntas
    BROWSER = "$casa\no-abrir-navegador.exe" # no existe: Claude no abre el navegador y enseña el enlace
    # Copilot, si BROWSER falla, abre el navegador del sistema; con esta variable usa solo la orden que se le da.
    COPILOT_DEBUG_BROWSER = (ConvertTo-Json -Compress @("$casa\no-abrir-navegador.exe"))
}
if (-not $SinAislar) {
    $entorno += @{
        USERPROFILE = $casa; HOME = $casa; APPDATA = "$casa\AppData\Roaming"; LOCALAPPDATA = "$casa\AppData\Local"
        TEMP = "$casa\tmp"; TMP = "$casa\tmp"
        CLAUDE_CONFIG_DIR = "$casa\.claude"; CODEX_HOME = "$casa\.codex"; COPILOT_HOME = "$casa\.copilot"
    }
}
foreach ($c in $entorno.Keys) { Set-Item "env:$c" $entorno[$c] }
Write-Host "Cuenta: $([Security.Principal.WindowsIdentity]::GetCurrent().Name); USERPROFILE=$env:USERPROFILE; LOCALAPPDATA=$env:LOCALAPPDATA"
'AppData\Roaming', 'AppData\Local', 'tmp', '.claude' | ForEach-Object { New-Item -ItemType Directory -Force (Join-Path $casa $_) | Out-Null }
# Claude registra claude-cli:// en HKCU al arrancar; aquí no, para no tocar el registro del usuario.
'{"disableDeepLinkRegistration":"disable"}' | Set-Content -Encoding utf8NoBOM "$casa\.claude\settings.json"

function PathUsuario { [Environment]::GetEnvironmentVariable('Path', 'User') }
function Seccion([string]$t) { Write-Host "`n=== $t" }
$pathAntes = PathUsuario
$deepLinkAntes = Test-Path 'HKCU:\Software\Classes\claude-cli'

Seccion "Instalar $Agente $(if ($Version) { $Version } else { '(última)' }) en $casa"
$inicio = Get-Date
switch ($Agente) {
    'claude' {
        # https://claude.ai/install.ps1 → downloads.claude.ai: comprueba el SHA-256 del manifiesto y ejecuta «claude install».
        $s = Invoke-RestMethod 'https://claude.ai/install.ps1'
        if ($Version) { & ([scriptblock]::Create($s)) $Version } else { & ([scriptblock]::Create($s)) }
        $exe = "$casa\.local\bin\claude.exe"
    }
    'codex' {
        # https://chatgpt.com/codex/install.ps1 → releases.openai.com (SHA-256 de codex-package_SHA256SUMS). Versión: CODEX_RELEASE.
        if ($Version) { $env:CODEX_RELEASE = $Version }
        $s = Invoke-RestMethod 'https://chatgpt.com/codex/install.ps1'
        & ([scriptblock]::Create($s))
        Remove-Item env:CODEX_RELEASE -ErrorAction SilentlyContinue # «codex update» vuelve a ejecutar install.ps1, que la leería
        $exe = "$env:LOCALAPPDATA\Programs\OpenAI\Codex\bin\codex.exe"
    }
    'copilot' {
        if ($Descarga) {
            $base = if ($Version) { "https://github.com/github/copilot-cli/releases/download/v$Version" } else { 'https://github.com/github/copilot-cli/releases/latest/download' }
            $zip = "$env:TEMP\copilot-win32-x64.zip"
            Invoke-WebRequest "$base/copilot-win32-x64.zip" -OutFile $zip
            $sumas = (Invoke-WebRequest "$base/SHA256SUMS.txt").Content
            $sumas = if ($sumas -is [byte[]]) { [Text.Encoding]::UTF8.GetString($sumas) } else { $sumas }
            $esperada = ($sumas -split "`n" | Where-Object { $_ -match '\scopilot-win32-x64\.zip\s*$' }) -split '\s+' | Select-Object -First 1
            $real = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
            if (-not $esperada -or $real -ne $esperada.ToLower()) { throw "SHA-256 de copilot-win32-x64.zip: $real, esperada $esperada" }
            Write-Host "SHA-256 comprobada: $real"
            Expand-Archive $zip "$env:LOCALAPPDATA\GitHubCopilotCLI" -Force
            $exe = "$env:LOCALAPPDATA\GitHubCopilotCLI\copilot.exe"
            break
        }
        # La orden oficial en Windows es winget (GitHub.Copilot, un MSI); npm necesita Node.
        if (-not (Get-Command winget -ErrorAction SilentlyContinue)) { throw 'winget no está en este equipo' }
        $a = @('install', '--id', 'GitHub.Copilot', '--exact', '--silent', '--disable-interactivity', '--accept-package-agreements', '--accept-source-agreements')
        if ($Version) { $a += '--version', $Version }
        & winget @a
        Write-Host "winget salió con $LASTEXITCODE"
        $exe = $null
    }
    'antigravity' {
        # https://antigravity.google/cli/install.ps1 → manifiesto con SHA-512; siempre la última; luego «agy install».
        $s = Invoke-RestMethod 'https://antigravity.google/cli/install.ps1'
        $extra = if ($deVerdad) { @() } else { @('--skip-path', '--skip-aliases') }
        & ([scriptblock]::Create($s)) @extra
        $exe = "$env:LOCALAPPDATA\agy\bin\agy.exe"
    }
}
Write-Host "Tardó $([int]((Get-Date) - $inicio).TotalSeconds) s"

# El PATH de esta consola es el de antes: se busca en la ruta conocida y en el PATH de usuario y del equipo del registro.
$nombre = @{ claude = 'claude'; codex = 'codex'; copilot = 'copilot'; antigravity = 'agy' }[$Agente]
if (-not $exe -or -not (Test-Path $exe)) {
    $env:Path = (PathUsuario) + ';' + [Environment]::GetEnvironmentVariable('Path', 'Machine') + ';' + $env:Path
    $exe = (Get-Command $nombre -ErrorAction SilentlyContinue | Select-Object -First 1).Source
}
Seccion 'Dónde quedó'
if (-not $exe) { throw "no se encuentra $nombre tras instalar" }
Write-Host $exe
$item = Get-Item $exe
if ($item.LinkType) { Write-Host "($($item.LinkType) → $($item.Target))" }
Seccion 'Versión'
& $exe --version
Seccion 'PATH de usuario del registro'
$pathDespues = PathUsuario
if ($pathDespues -ceq $pathAntes) { Write-Host 'sin cambios' } else {
    $nuevas = ($pathDespues -split ';') | Where-Object { $_ -and ($pathAntes -split ';') -notcontains $_ }
    Write-Host "CAMBIADO; entradas nuevas: $($nuevas -join '; ')"
}
if ((Test-Path 'HKCU:\Software\Classes\claude-cli') -ne $deepLinkAntes) { Write-Host 'HKCU\Software\Classes\claude-cli CAMBIADO' }
Seccion 'Archivos en la carpeta de usuario'
Get-ChildItem $casa -Recurse -Force -Depth 6 -ErrorAction SilentlyContinue | Where-Object { -not $_.PSIsContainer } |
    Where-Object { $_.FullName -notmatch '\\tmp\\' } | Select-Object -First 80 |
    ForEach-Object { '{0,12}  {1}' -f $_.Length, $_.FullName.Substring($casa.Length) }

if ($Actualizar) {
    Seccion 'Actualizar'
    switch ($Agente) {
        'claude' { & $exe update }
        'codex' { & $exe update } # si no existe, la receta vuelve a ejecutar install.ps1
        'copilot' { & $exe update }
        'antigravity' { & $exe update }
    }
    Write-Host "salió con $LASTEXITCODE"
    & $exe --version
}

if ($ProbarSesion -and $Agente -ne 'antigravity') {
    $args_ = @{ claude = @('auth', 'login'); codex = @('login', '--device-auth'); copilot = @('login', '--device-code') }[$Agente]
    Seccion "Inicio de sesión: $nombre $($args_ -join ' ') (tuberías, entrada abierta, 25 s)"
    $p = [Diagnostics.ProcessStartInfo]::new($exe)
    $args_ | ForEach-Object { $p.ArgumentList.Add($_) }
    $p.RedirectStandardInput = $true; $p.RedirectStandardOutput = $true; $p.RedirectStandardError = $true
    $p.UseShellExecute = $false
    $proc = [Diagnostics.Process]::Start($p)
    $out = $proc.StandardOutput.ReadToEndAsync(); $err = $proc.StandardError.ReadToEndAsync()
    $termino = $proc.WaitForExit(25000)
    if (-not $termino) { $proc.Kill($true); $proc.WaitForExit() }
    Write-Host "terminó solo: $termino; código: $($proc.ExitCode)"
    Write-Host '--- stdout'; Write-Host $out.Result
    Write-Host '--- stderr'; Write-Host $err.Result
}

if ($Desinstalar) {
    # Lo que haría la receta: la orden oficial si la hay (winget) y si no, borrar lo que dejó el instalador.
    Seccion 'Desinstalar'
    switch ($Agente) {
        'claude' { Remove-Item -Force $exe; Remove-Item -Recurse -Force "$casa\.local\share\claude" }
        'codex' {
            # bin es un enlace (junction) a packages\standalone\current\bin: se quita el enlace y luego el paquete.
            Remove-Item -Force -Recurse "$env:LOCALAPPDATA\Programs\OpenAI\Codex\bin"
            Remove-Item -Recurse -Force "$(if ($env:CODEX_HOME) { $env:CODEX_HOME } else { "$casa\.codex" })\packages\standalone"
        }
        'copilot' {
            if ($Descarga) { Remove-Item -Recurse -Force (Split-Path $exe) } else {
                # El MSI va al LOCALAPPDATA real aunque se aísle: fuera del CI quitaría un Copilot que ya tuviera el usuario.
                if ($env:CI -ne 'true') { throw 'winget uninstall GitHub.Copilot solo en el CI' }
                & winget uninstall --id GitHub.Copilot --exact --silent --disable-interactivity --accept-source-agreements
                Write-Host "winget salió con $LASTEXITCODE"
            }
        }
        'antigravity' { Remove-Item -Force $exe }
    }
    Write-Host "¿Sigue ${exe}? $(Test-Path $exe)"
    $pathFinal = PathUsuario
    $quedan = ($pathFinal -split ';') | Where-Object { $_ -and ($pathAntes -split ';') -notcontains $_ }
    Write-Host "Entradas del PATH de usuario que dejó y siguen: $(if ($quedan) { $quedan -join '; ' } else { 'ninguna' })"
}