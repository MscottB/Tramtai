# Prueba el servidor de Windows de verdad (el servicio Tramtai con su cuenta virtual NT SERVICE\Tramtai), desde una
# consola de administrador. Lo usa el job «Servidor (Windows Server)» del CI en Windows Server 2025; necesita Go:
#   ./scripts/servidor/probar-servidor.ps1
# No lo ejecutes en tu equipo: instala y desinstala el servicio Tramtai del sistema, con sus datos.
# Compila dos versiones en dos .zip como el portable (tramtai.exe, uv.exe, portable.txt y la licencia) y sigue la
# secuencia de probar-servidor.sh: «servidor instalar» desde el .zip, la contraseña del administrador, TLS propio con un
# certificado de prueba, iniciar sesión, una petición al Claude simulado, guardar una clave con una petición larga en
# curso (en Windows vale sin reiniciar: la petición termina y el servicio sigue siendo el mismo proceso), actualizar con
# «servidor instalar» desde el .zip nuevo y desinstalar con --borrar-datos.
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true # una orden que sale con error para el script
$env:TRAMTAI_IDIOMA = 'es' # se comparan los textos de la CLI

function Falla([string]$texto) {
    $registro = Join-Path $env:ProgramData 'Tramtai\datos\servicio.log'
    if (Test-Path $registro) { Get-Content $registro -Tail 40 | Write-Host }
    throw "FALLA: $texto"
}

if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'probar-servidor.ps1 necesita una consola de administrador'
}
$raiz = Resolve-Path (Join-Path $PSScriptRoot '..\..')
$trabajo = Join-Path ([IO.Path]::GetTempPath()) "tramtai-servidor-prueba-$PID"
New-Item -ItemType Directory -Force $trabajo | Out-Null
$programa = Join-Path $env:ProgramFiles 'Tramtai Servidor'
$datos = Join-Path $env:ProgramData 'Tramtai'
$contrasena = 'contrasena-de-prueba-del-ci'
$puerto = 17380
$base = "https://127.0.0.1:$puerto"

Push-Location $raiz
try {
    # Dos .zip como el portable, con versiones distintas.
    & (Join-Path $raiz 'scripts\uv.ps1') -Objetivo x86_64-pc-windows-msvc -Destino (Join-Path $trabajo 'uv')
    $copias = @{}
    foreach ($version in 'v0.0.1+ci', 'v0.0.2+ci') {
        $carpeta = Join-Path $trabajo "zip-$version"
        New-Item -ItemType Directory -Force $carpeta | Out-Null
        go build -trimpath -ldflags "-s -w -X tramtai/internal/version.Version=$version" -o (Join-Path $carpeta 'tramtai.exe') ./cmd/tramtai
        Copy-Item (Join-Path $trabajo 'uv\uv.exe'), 'scripts\portable.txt' $carpeta
        Copy-Item LICENSE.md (Join-Path $carpeta 'LICENCIA.md')
        $zip = Join-Path $trabajo "Tramtai-$version-portable-windows-x64.zip"
        Compress-Archive -Path (Join-Path $carpeta '*') -DestinationPath $zip
        $copias[$version] = Join-Path $trabajo "portable-$version"
        Expand-Archive $zip $copias[$version]
    }
    # El Claude simulado, en una carpeta que la cuenta del servicio puede leer (Program Files deja leer a los usuarios).
    $simulado = Join-Path $env:ProgramFiles 'Tramtai Prueba\claude.exe'
    go build -o $simulado ./scripts/servidor/claude-simulado
    # Certificado de prueba (ECDSA P-256) para 127.0.0.1.
    Push-Location $trabajo
    try { go run "$(go env GOROOT)\src\crypto\tls\generate_cert.go" --host 127.0.0.1 --ecdsa-curve P256 2>&1 | Out-Null } finally { Pop-Location }
} finally { Pop-Location }
Set-Location $trabajo # fuera del repositorio: las órdenes van al espacio global del servidor

# Servidor es tramtai-servidor (el .cmd que deja instalar), con lo que le llegue por la tubería como entrada.
function Servidor { $input | & (Join-Path $programa 'tramtai.exe') --servidor @args }
function Pid-Servicio { (Get-CimInstance Win32_Service -Filter "Name='Tramtai'").ProcessId }
function Esperar {
    foreach ($i in 1..60) {
        try { if ((Servidor servicio estado 2>$null | Out-String) -match '(?m)^servicio activo') { return } } catch { }
        Start-Sleep 1
    }
    Falla "el servicio no responde: $((Get-Service Tramtai).Status)"
}
function Web([string]$ruta, $sesion, [string]$cuerpo) {
    $parametros = @{ Uri = "$base$ruta"; SkipCertificateCheck = $true; SkipHttpErrorCheck = $true; SslProtocol = 'Tls13' }
    if ($sesion) { $parametros.WebSession = $sesion }
    if ($cuerpo) { $parametros += @{ Method = 'Post'; Body = $cuerpo; ContentType = 'application/json' } }
    Invoke-WebRequest @parametros
}

Write-Host '== instalar desde el .zip portable'
& (Join-Path $copias['v0.0.1+ci'] 'tramtai.exe') servidor instalar --acepto-terminos
foreach ($f in 'tramtai.exe', 'uv.exe', 'tramtai-servidor.cmd') {
    if (-not (Test-Path (Join-Path $programa $f))) { Falla "instalar no dejó $f en $programa" }
}
if ((Get-Service Tramtai).Status -ne 'Running') { Falla 'el servicio Tramtai no quedó en marcha' }
if ((Get-CimInstance Win32_Service -Filter "Name='Tramtai'").StartName -ne 'NT SERVICE\Tramtai') { Falla 'el servicio no usa la cuenta NT SERVICE\Tramtai' }
Esperar

Write-Host '== contraseña, TLS propio e inicio de sesión'
$contrasena, $contrasena | Servidor administrador contraseña | Out-Null
New-Item -ItemType Directory -Force (Join-Path $datos 'tls') | Out-Null
Copy-Item (Join-Path $trabajo 'cert.pem'), (Join-Path $trabajo 'key.pem') (Join-Path $datos 'tls')
@"
servicio:
  host: 127.0.0.1
  puerto: $puerto
  tls:
    modo: archivos
    certificado: '$datos\tls\cert.pem'
    clave: '$datos\tls\key.pem'
comandos_agentes:
  claude: '$simulado'
"@ | Set-Content -Encoding utf8 (Join-Path $datos 'config.yaml')
Servidor servidor reiniciar | Out-Null
Esperar
foreach ($i in 1..30) {
    try { if ((Web '/healthz').StatusCode -eq 200) { break } } catch { }
    Start-Sleep 1
}
$codigo = (Web '/ui/acerca').StatusCode
if ($codigo -ne 401) { Falla "sin sesión, /ui/acerca da $codigo y no 401" }
$respuesta = Invoke-WebRequest -Uri "$base/ui/acceder" -Method Post -ContentType 'application/json' -SkipCertificateCheck -SkipHttpErrorCheck `
    -SslProtocol Tls13 -Body (@{ contrasena = $contrasena; codigo = '' } | ConvertTo-Json) -SessionVariable sesion
if ($respuesta.StatusCode -ne 200) { Falla "el inicio de sesión da $($respuesta.StatusCode)" }
$codigo = (Web '/ui/acerca' $sesion).StatusCode
if ($codigo -ne 200) { Falla "con la sesión, /ui/acerca da $codigo" }
if (-not (Select-String -Path (Join-Path $datos 'datos\servicio.log') -Pattern 'acceso correcto del administrador desde 127.0.0.1' -Quiet)) {
    Falla 'el registro no anota el inicio de sesión'
}

Write-Host '== una petición al Claude simulado'
'sk-ant-prueba-no-es-una-clave' | Servidor claves guardar ANTHROPIC_API_KEY | Out-Null
$salida = Servidor ejecutar hola -a claude 2>&1 | Out-String
if ($salida -notmatch 'Hola desde el servidor') { Falla "la petición no trajo la respuesta: $salida" }

Write-Host '== guardar una clave con una petición larga en curso'
$largo = Start-Process (Join-Path $programa 'tramtai.exe') -ArgumentList '--servidor', 'ejecutar', '"trabajo largo"', '-a', 'claude' -NoNewWindow -PassThru `
    -RedirectStandardOutput (Join-Path $trabajo 'largo.txt') -RedirectStandardError (Join-Path $trabajo 'largo-errores.txt')
$id = $null
foreach ($i in 1..20) {
    $fila = Servidor trabajos listar | Select-String '^\s+(\S+)\s+ejecutando\s'
    if ($fila) { $id = $fila.Matches[0].Groups[1].Value; break }
    Start-Sleep -Milliseconds 500
}
if (-not $id) { Falla "la petición larga no llegó a ejecutarse: $(Get-Content (Join-Path $trabajo 'largo-errores.txt') -Raw)" }
$antes = Pid-Servicio
'sk-prueba-no-es-una-clave' | Servidor claves guardar OPENAI_API_KEY | Out-Null
$largo | Wait-Process -Timeout 90
$fila = Servidor trabajos listar | Select-String "^\s+$id\s+(\S+)"
$estado = if ($fila) { $fila.Matches[0].Groups[1].Value }
if ($estado -ne 'completado') { Falla "la petición larga quedó en «$estado» al guardar la clave" }
if ((Pid-Servicio) -ne $antes) { Falla 'guardar la clave reinició el servicio (en Windows vale sin reiniciar)' }

Write-Host '== actualizar desde el .zip nuevo'
$antes = Pid-Servicio
& (Join-Path $copias['v0.0.2+ci'] 'tramtai.exe') servidor instalar --acepto-terminos
Esperar
$version = & (Join-Path $programa 'tramtai.exe') version
if ($version -notmatch [regex]::Escape('v0.0.2+ci')) { Falla "tras actualizar: $version" }
if ((Pid-Servicio) -eq $antes) { Falla 'actualizar no reinició el servicio con el programa nuevo' }

Write-Host '== desinstalar con --borrar-datos'
# Desde la copia del .zip: el programa instalado no puede borrarse a sí mismo mientras corre.
's' | & (Join-Path $copias['v0.0.2+ci'] 'tramtai.exe') --servidor servidor desinstalar --borrar-datos
if (Get-Service Tramtai -ErrorAction SilentlyContinue) { Falla 'desinstalar deja el servicio Tramtai' }
foreach ($d in $programa, $datos) {
    if (Test-Path $d) { Falla "desinstalar deja $d" }
}
if ([Environment]::GetEnvironmentVariable('Path', 'Machine') -match [regex]::Escape($programa)) { Falla 'desinstalar deja el programa en el PATH del sistema' }
Remove-Item -Recurse -Force (Split-Path $simulado)
Write-Host 'el servidor de Windows se instala desde el .zip, atiende por HTTPS, guarda claves sin cortar peticiones, se actualiza y se desinstala'
