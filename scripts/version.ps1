# Calcula la versión vX.Y.Z+DDMMAAAA a partir de git: parte de la última etiqueta v* y la sube según los
# commits posteriores (Conventional Commits):
#   "tipo!:" o "BREAKING CHANGE" en el cuerpo -> mayor  (X+1.0.0)
#   "feat:"                                    -> menor  (X.Y+1.0)
#   cualquier otro commit (fix, docs, chore…)  -> parche (X.Y.Z+1)
# Sin commits nuevos devuelve la etiqueta tal cual. Sin etiquetas parte de 0.0.0.
# Uso: scripts/version.ps1 [-Explicar] [-Etiquetar]
param([switch]$Etiquetar, [switch]$Explicar)
$ErrorActionPreference = 'Stop'
Push-Location (Join-Path $PSScriptRoot '..')
try {
    $ultima = git describe --tags --abbrev=0 --match 'v[0-9]*' 2>$null
    if ($LASTEXITCODE) { $ultima = $null }
    $base = if ($ultima) { ($ultima -replace '^v', '') -replace '\+.*$', '' } else { '0.0.0' }
    $rango = if ($ultima) { "$ultima..HEAD" } else { 'HEAD' }
    [int]$mayor, [int]$menor, [int]$parche = $base.Split('.')

    $crudo = git log --format='%s%x1f%b%x1e' $rango
    if ($LASTEXITCODE) { throw 'git log falló: ¿es un repositorio git con commits?' }
    $commits = ($crudo -join "`n").Split([char]0x1e) | Where-Object { $_.Trim() }

    $nivel = 0  # 0 ninguno, 1 parche, 2 menor, 3 mayor
    foreach ($c in $commits) {
        $asunto, $cuerpo = $c.Trim().Split([char]0x1f, 2)
        $tipo = if ($asunto -match '^\w+(\([^)]*\))?!:' -or $cuerpo -match 'BREAKING CHANGE') { 3 }
                elseif ($asunto -match '^feat(\([^)]*\))?:') { 2 } else { 1 }
        if ($Explicar) { Write-Host ('{0,-6} {1}' -f @('', 'parche', 'menor', 'mayor')[$tipo], $asunto) }
        $nivel = [Math]::Max($nivel, $tipo)
    }

    switch ($nivel) {
        3 { $mayor++; $menor = 0; $parche = 0 }
        2 { $menor++; $parche = 0 }
        1 { $parche++ }
    }
    $version = if ($nivel -eq 0 -and $ultima) { $ultima } else { "v$mayor.$menor.$parche+$(Get-Date -Format ddMMyyyy)" }
    if ($Explicar) { Write-Host "desde $(if ($ultima) { $ultima } else { 'el inicio (0.0.0)' }): $($commits.Count) commits -> $version" }

    if ($Etiquetar) {
        if ($nivel -eq 0) { throw "No hay commits nuevos desde $ultima." }
        if (git status --porcelain) { throw 'Hay cambios sin confirmar: haz commit antes de etiquetar.' }
        git tag -a $version -m "Tramtai $version"
        if ($LASTEXITCODE) { throw 'git tag falló' }
        Write-Host "Etiqueta $version creada. Súbela con: git push origin $version"
    }
    $version
} finally { Pop-Location }
