# Gera as imagens do player (YouTube, YT Music, Netflix, Disney+) para o README.
#
# PRIVACIDADE — por que existe uma cópia do .exe com outro nome:
#
# O navegador embutido guarda o perfil (logins, histórico) numa pasta derivada do
# NOME DO ARQUIVO do programa. Rodando uma cópia chamada "nimbus-fotos.exe", ele
# usa um perfil novo e vazio: os sites abrem DESLOGADOS, então as imagens não
# mostram sua conta, suas recomendações nem seu histórico.
#
# Rode da raiz do projeto:  .\docs\gerar-imagens-player.ps1

Add-Type -AssemblyName System.Drawing

$raiz = Split-Path $PSScriptRoot -Parent
$exeOriginal = Join-Path $raiz 'build\nimbus.exe'
$exeFotos = Join-Path $raiz 'build\nimbus-fotos.exe'
$destino = $PSScriptRoot

if (-not (Test-Path $exeOriginal)) {
    Write-Output "compile primeiro: .\compilar.bat"
    exit 1
}

# A janelinha do player nasce aqui (posição padrão, em coordenadas de tela).
$area = @{ x = 420; y = 80; w = 560; h = 380 }

$servicos = @('youtube', 'music', 'netflix', 'disney')

Get-Process -Name nimbus, nimbus-fotos -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 800
Copy-Item $exeOriginal $exeFotos -Force

$recortes = @{}
foreach ($servico in $servicos) {
    $env:NIMBUS_DEBUG_PLAYER = $servico
    $env:NIMBUS_DEBUG_ALFA = '1.0'

    Start-Process $exeFotos
    Start-Sleep -Seconds 16   # tempo para o site carregar

    $bmp = New-Object System.Drawing.Bitmap $area.w, $area.h
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.CopyFromScreen($area.x, $area.y, 0, 0, (New-Object System.Drawing.Size $area.w, $area.h))
    $g.Dispose()

    $caminho = Join-Path $destino "player-$servico.png"
    $bmp.Save($caminho, [System.Drawing.Imaging.ImageFormat]::Png)
    $recortes[$servico] = $bmp
    Write-Output "capturado: $servico"

    Get-Process -Name nimbus-fotos -ErrorAction SilentlyContinue | Stop-Process -Force
    Start-Sleep -Milliseconds 1200
}

# ─── monta a grade 2x2 ───────────────────────────────────────────────────
$margem = 20
$espaco = 16
$larg = $margem * 2 + $area.w * 2 + $espaco
$alt = $margem * 2 + $area.h * 2 + $espaco

$final = New-Object System.Drawing.Bitmap $larg, $alt
$g = [System.Drawing.Graphics]::FromImage($final)
$ret = New-Object System.Drawing.Rectangle 0, 0, $larg, $alt
$pincel = New-Object System.Drawing.Drawing2D.LinearGradientBrush $ret,
    ([System.Drawing.Color]::FromArgb(255, 22, 22, 28)),
    ([System.Drawing.Color]::FromArgb(255, 10, 10, 13)), 45.0
$g.FillRectangle($pincel, $ret)

$posicoes = @(
    @{ s = 'youtube'; x = $margem;                    y = $margem },
    @{ s = 'music';   x = $margem + $area.w + $espaco; y = $margem },
    @{ s = 'netflix'; x = $margem;                    y = $margem + $area.h + $espaco },
    @{ s = 'disney';  x = $margem + $area.w + $espaco; y = $margem + $area.h + $espaco }
)
foreach ($p in $posicoes) {
    if ($recortes.ContainsKey($p.s)) { $g.DrawImage($recortes[$p.s], $p.x, $p.y) }
}
$g.Dispose()
$final.Save((Join-Path $destino 'players.png'), [System.Drawing.Imaging.ImageFormat]::Png)
Write-Output "imagem final: players.png ($($final.Width)x$($final.Height))"

foreach ($r in $recortes.Values) { $r.Dispose() }
$final.Dispose()
Remove-Item $exeFotos -Force -ErrorAction SilentlyContinue
$env:NIMBUS_DEBUG_PLAYER = ''
$env:NIMBUS_DEBUG_ALFA = ''
