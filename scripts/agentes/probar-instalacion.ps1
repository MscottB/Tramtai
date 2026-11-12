# M0 de la v0.60 (docs/equipo/releases/v0.60-preparar-agentes/m0.md): instala un CLI con su instalador oficial en una
# carpeta de usuario temporal (USERPROFILE, HOME, APPDATA, LOCALAPPDATA y TEMP apuntan ahí) e imprime dónde quedó, su
# --version, los archivos que dejó y si cambió el PATH de usuario del registro. Con -Actualizar ejecuta después su orden
# de actualizar y, con -ProbarSesion, lanza su inicio de sesión con tuberías, guarda lo que imprime en 25 s y lo cierra.
# Lo usa el job agentes-compatibilidad.yml:
#   pwsh -NoProfile -File scripts/agentes/probar-instalacion.ps1 -Agente claude [-Version 2.1.311] [-Actualizar] [-ProbarSesion]
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
    [switch]$SinAislar
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$deVerdad = $env:CI -eq 'true' -or $SinAislar
if (-not $deVerdad -and -not $CambiarRegistro -and $Agente -in 'codex', 'copilot') {
    throw "$Agente cambia el PATH de usuario del registro al instalarse: fuera del CI hace falta -CambiarRegistro"
}

$casa = if ($SinAislar) { $env:USERPROFILE } else { [IO.Path]::GetFullPath($Carpeta) }
$entorno = @{
    CODEX_NON_INTERACTIVE = '1'          # sin «¿Iniciar Codex ahora?» ni otras preguntas
    BROWSER = "$casa\no-abrir-navegador.exe" # no existe: el CLI no puede abrir el navegador y enseña el enlace
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
        $exe = "$casa\AppData\Local\Programs\OpenAI\Codex\bin\codex.exe"
    }
    'copilot' {
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
        $exe = "$casa\AppData\Local\agy\bin\agy.exe"
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
