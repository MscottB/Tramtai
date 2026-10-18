# Calcula la versión vX.Y.Z+DDMMAAAA a partir de git: parte de la última etiqueta v* y la sube según los
# commits posteriores (Conventional Commits):
#   "tipo!:" o "BREAKING CHANGE" en el cuerpo -> mayor  (X+1.0.0)
#   "feat:"                                    -> menor  (X.Y+1.0)
#   cualquier otro commit (fix, docs, chore…)  -> parche (X.Y.Z+1)
# DDMMAAAA es la fecha del commit versionado. Sin commits nuevos devuelve la etiqueta tal cual; sin etiquetas parte de 0.0.0.
# Uso: scripts/version.ps1 [-Hasta <commit>] [-Resumen <texto>] [-ResumenEn <text>] [-Explicar] [-Etiquetar]
#   -Explicar   muestra las notas de la versión (los cambios agrupados por tipo)
#   -Etiquetar  crea la etiqueta anotada con esas notas como mensaje (no la sube)
#   -Resumen    texto que encabeza las notas, para explicar lo que los mensajes de commit no dicen
#   -ResumenEn  el mismo resumen en inglés: va al final, tras «## English», y la app lo muestra en inglés
#               (obligatorio al etiquetar con -Resumen; la lista de cambios sale de los commits, en español)
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

    $notas = "Tramtai $version"
    if ($Resumen) { $notas += "`n`n$Resumen" }
    $notas += "`n`nCambios desde $(if ($ultima) { $ultima } else { 'el inicio del proyecto' }):"
    foreach ($g in $grupos.Keys) {
        if ($grupos[$g].Count) { $notas += "`n`n${g}:`n" + ($grupos[$g] -join "`n") }
    }
    if ($ResumenEn) { $notas += "`n`n## English`n`nTramtai $version`n`n$ResumenEn`n`nFull list of changes (in Spanish) above." }
    if ($Explicar) { Write-Host $notas`n }

    if ($Etiquetar) {
        if ($nivel -eq 0) { throw "No hay commits nuevos desde $ultima." }
        if ($Resumen -and -not $ResumenEn) { throw 'Falta -ResumenEn: las notas van también en inglés.' }
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
