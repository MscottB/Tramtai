# Genera las imágenes del asistente de instalación (lateral.bmp y cabecera.bmp) con la línea de tranvía de Tramtai.
# Se ejecuta a mano cuando cambia el diseño; las .bmp generadas se versionan. Tamaño doble para pantallas con escala.
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$dir = $PSScriptRoot
function Color([string]$hex) { [System.Drawing.ColorTranslator]::FromHtml($hex) }

# Lateral de bienvenida y final: 164x314 lógicos.
$ancho, $alto = 328, 628
$img = New-Object System.Drawing.Bitmap $ancho, $alto, ([System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
$g = [System.Drawing.Graphics]::FromImage($img)
$g.SmoothingMode = 'AntiAlias'; $g.TextRenderingHint = 'AntiAliasGridFit'
$g.Clear((Color '#0f1b1e'))
$x, $arriba, $abajo = 70, 70, 455
$via = New-Object System.Drawing.Pen (Color '#2dd4bf'), 12
$via.StartCap = 'Round'; $via.EndCap = 'Round'
$g.DrawLine($via, $x, $arriba, $x, $abajo)
$fuente = New-Object System.Drawing.Font 'Segoe UI Semibold', 19, ([System.Drawing.FontStyle]::Regular), ([System.Drawing.GraphicsUnit]::Pixel)
$suave = New-Object System.Drawing.SolidBrush (Color '#9fb3b8')
$paradas = @(@('Claude Code', '#f28a4f'), @('Codex', '#5b8def'), @('Gemini', '#a78bfa'), @('Antigravity', '#f472b6'))
for ($i = 0; $i -lt $paradas.Count; $i++) {
    $y = 120 + $i * 100
    $g.FillEllipse((New-Object System.Drawing.SolidBrush (Color '#0f1b1e')), $x - 17, $y - 17, 34, 34)
    $g.DrawEllipse((New-Object System.Drawing.Pen (Color $paradas[$i][1]), 7), $x - 14, $y - 14, 28, 28)
    $g.DrawString($paradas[$i][0], $fuente, $suave, $x + 34, $y - 13)
}
$marca = New-Object System.Drawing.Font 'Segoe UI', 46, ([System.Drawing.FontStyle]::Bold), ([System.Drawing.GraphicsUnit]::Pixel)
$g.DrawString('Tramtai', $marca, (New-Object System.Drawing.SolidBrush (Color '#eef3f2')), 36, 500)
$g.FillRectangle((New-Object System.Drawing.SolidBrush (Color '#ffcf5c')), 42, 570, 56, 6)
$img.Save((Join-Path $dir 'lateral.bmp'), [System.Drawing.Imaging.ImageFormat]::Bmp)
$g.Dispose(); $img.Dispose()

# Cabecera de las páginas interiores: 150x57 lógicos, a la derecha.
$img = New-Object System.Drawing.Bitmap 300, 114, ([System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
$g = [System.Drawing.Graphics]::FromImage($img)
$g.SmoothingMode = 'AntiAlias'; $g.InterpolationMode = 'HighQualityBicubic'
$g.Clear([System.Drawing.Color]::White)
$via = New-Object System.Drawing.Pen (Color '#0b6e64'), 6
$via.StartCap = 'Round'; $via.EndCap = 'Round'
$g.DrawLine($via, 20, 57, 196, 57)
$colores = @('#c4561d', '#2563eb', '#7c3aed', '#db2777')
for ($i = 0; $i -lt 4; $i++) {
    $cx = 34 + $i * 50
    $g.FillEllipse([System.Drawing.Brushes]::White, $cx - 10, 47, 20, 20)
    $g.DrawEllipse((New-Object System.Drawing.Pen (Color $colores[$i]), 5), $cx - 8, 49, 16, 16)
}
$icono = [System.Drawing.Image]::FromFile((Resolve-Path (Join-Path $dir '..\..\escritorio\icono\tramtai.png')))
$g.DrawImage($icono, 214, 15, 84, 84)
$icono.Dispose()
$img.Save((Join-Path $dir 'cabecera.bmp'), [System.Drawing.Imaging.ImageFormat]::Bmp)
$g.Dispose(); $img.Dispose()
Write-Host 'lateral.bmp y cabecera.bmp generadas'
