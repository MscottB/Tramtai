# Calcula la versión vX.Y.Z+DDMMAAAA a partir de git: parte de la última etiqueta v* y la sube según los
# commits posteriores (Conventional Commits):
#   "tipo!:" o "BREAKING CHANGE" en el cuerpo -> mayor  (X+1.0.0)
#   "feat:"                                    -> menor  (X.Y+1.0)
#   cualquier otro commit (fix, docs, chore…)  -> parche (X.Y.Z+1)
# DDMMAAAA es la fecha del commit versionado. Sin commits nuevos devuelve la etiqueta tal cual; sin etiquetas parte de 0.0.0.
# Uso: scripts/version.ps1 [-Hasta <commit>] [-Resumen <texto|archivo>] [-ResumenEn <text|file>] [-Explicar] [-Etiquetar]
#   -Explicar   muestra las notas y, aparte, los commits que entran en la versión (estos no van en las notas)
#   -Etiquetar  crea la etiqueta anotada con las notas como mensaje (no la sube); exige -Resumen y -ResumenEn
#   -Resumen    las notas para quien usa Tramtai, cortas y sin detalles internos (el repositorio es público):
#               secciones «Novedades:», «Mejoras:» o «Correcciones:» con viñetas «- …». Texto o ruta de un archivo.
#   -ResumenEn  las mismas notas en inglés: van tras «## English» y la app las muestra si está en inglés
# Las notas terminan con el enlace a la comparación en GitHub, para quien quiera el detalle técnico.
param([string]$Hasta = 'HEAD', [string]$Resumen = '', [string]$ResumenEn = '', [switch]$Etiquetar, [switch]$Explicar)
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
Push-Location (Join-Path $PSScriptRoot '..')
try {
    $ultima = git describe --tags --abbrev=0 --match 'v[0-9]*' $Hasta 2>$null
    if ($LASTEXITCODE) { $ultima = $null }
    $base = if ($ultima) { ($ultima -replace '^v', '') -replace '\+.*$', '' } else { '0.0.0' }
    $rango = if ($ultima) { "$ultima..$Hasta" } else { $Hasta }
    [int]$mayor, [int]$menor, [int]$parche = $base.Split('.')

    # Los merges solo repiten sus commits, y los que solo tocan la página (site/) o los paquetes de Homebrew y winget
    # no cambian la app: ni suben la versión ni salen en sus notas.
    $crudo = git log --reverse --no-merges --format='%s%x1f%b%x1e' $rango -- . ':(exclude)site' ':(exclude).github/workflows/paginas.yml' ':(exclude)Casks' ':(exclude)Formula' ':(exclude)paquetes'
    if ($LASTEXITCODE) { throw 'git log falló: ¿es un repositorio git con commits?' }
    $commits = ($crudo -join "`n").Split([char]0x1e) | Where-Object { $_.Trim() }

    $nivel = 0  # 0 ninguno, 1 parche, 2 menor, 3 mayor
    $grupos = [ordered]@{ 'Cambios incompatibles' = @(); 'Nuevas funciones' = @(); 'Correcciones' = @(); 'Otros cambios' = @() }
    foreach ($c in $commits) {
        $asunto, $cuerpo = $c.Trim().Split([char]0x1f, 2)
        $incompatible = $asunto -match '^\w+(\([^)]*\))?!:' -or $cuerpo -match 'BREAKING CHANGE'
        $tipo = if ($incompatible) { 3 } elseif ($asunto -match '^feat(\([^)]*\))?:') { 2 } else { 1 }
        $nivel = [Math]::Max($nivel, $tipo)
        $grupo = if ($incompatible) { 'Cambios incompatibles' } elseif ($tipo -eq 2) { 'Nuevas funciones' }
                 elseif ($asunto -match '^fix(\([^)]*\))?:') { 'Correcciones' } else { 'Otros cambios' }
        $entrada = '- ' + ($asunto -replace '^\w+(\([^)]*\))?!?:\s*', '')
        foreach ($linea in "$cuerpo" -split "`n") {
            if ($linea.Trim() -and $linea -notmatch '^\s*#' -and $linea -notmatch '^(Co-Authored-By|Signed-off-by):') { $entrada += "`n  " + $linea.Trim() }
        }
        $grupos[$grupo] += $entrada
    }

    switch ($nivel) {
        3 { $mayor++; $menor = 0; $parche = 0 }
        2 { $menor++; $parche = 0 }
        1 { $parche++ }
    }
    $fecha = git log -1 --format='%cd' --date='format:%d%m%Y' $Hasta
    $version = if ($nivel -eq 0 -and $ultima) { $ultima } else { "v$mayor.$menor.$parche+$fecha" }

    # Un valor que es la ruta de un archivo se lee entero (notas de varias líneas).
    if ($Resumen -and (Test-Path -LiteralPath $Resumen -PathType Leaf)) { $Resumen = (Get-Content -Raw -Encoding UTF8 -LiteralPath $Resumen).Trim() }
    if ($ResumenEn -and (Test-Path -LiteralPath $ResumenEn -PathType Leaf)) { $ResumenEn = (Get-Content -Raw -Encoding UTF8 -LiteralPath $ResumenEn).Trim() }
    $comparar = if ($ultima) { "https://github.com/MscottB/Tramtai/compare/$([uri]::EscapeDataString($ultima))...$([uri]::EscapeDataString($version))" } else { '' }
    $notas = "Tramtai $version`n`n$Resumen"
    if ($comparar) { $notas += "`n`nCambios técnicos: $comparar" }
    $notas += "`n`n## English`n`nTramtai $version`n`n$ResumenEn"
    if ($comparar) { $notas += "`n`nTechnical changes: $comparar" }
    if ($Explicar) {
        Write-Host $notas`n
        Write-Host "Commits desde $(if ($ultima) { $ultima } else { 'el inicio' }) (no van en las notas):"
        foreach ($g in $grupos.Keys) {
            if ($grupos[$g].Count) { Write-Host "`n${g}:`n$($grupos[$g] -join "`n")" }
        }
        Write-Host ''
    }

    if ($Etiquetar) {
        if ($nivel -eq 0) { throw "No hay commits nuevos desde $ultima." }
        if (-not $Resumen -or -not $ResumenEn) { throw 'Faltan -Resumen y -ResumenEn: las notas se escriben para quien usa Tramtai, en español e inglés.' }
        if ($Hasta -eq 'HEAD' -and (git status --porcelain)) { throw 'Hay cambios sin confirmar: haz commit antes de etiquetar.' }
        $archivo = New-TemporaryFile
        try {
            [IO.File]::WriteAllText($archivo, $notas, [Text.UTF8Encoding]::new($false))
            git tag -a $version -F $archivo --cleanup=verbatim $Hasta
            if ($LASTEXITCODE) { throw 'git tag falló' }
        } finally { Remove-Item $archivo }
        Write-Host "Etiqueta $version creada en $(git rev-parse --short $Hasta). Súbela con: git push origin $version"
    }
    $version
} finally { Pop-Location }
