# Gera as imagens do README: recorta CADA painel do Nimbus e cola num fundo
# escuro criado aqui.
#
# Por que recortar painel por painel em vez de fotografar a tela: o Nimbus é
# transparente, então uma foto da região inteira mostraria o que estiver aberto
# no computador. Assim, nada do seu trabalho aparece na imagem.
#
# Antes de rodar:
#   $env:NIMBUS_DEBUG_CONFIG='1'; $env:NIMBUS_DEBUG_ALFA='1.0'
#   .\build\nimbus.exe
# e deixe os painéis nas posições padrão (botão "Recolocar painéis").

Add-Type -AssemblyName System.Drawing

$destino = Join-Path $PSScriptRoot ''
$paineis = @(
    @{ nome = 'musica';  x = 80;  y = 80;  w = 310; h = 306 },
    @{ nome = 'sistema'; x = 80;  y = 398; w = 310; h = 250 },
    @{ nome = 'config';  x = 410; y = 398; w = 300; h = 340 }
)

$recortes = @{}
foreach ($p in $paineis) {
    $bmp = New-Object System.Drawing.Bitmap $p.w, $p.h
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.CopyFromScreen($p.x, $p.y, 0, 0, (New-Object System.Drawing.Size $p.w, $p.h))
    $g.Dispose()
    $recortes[$p.nome] = $bmp
    $bmp.Save((Join-Path $destino "painel-$($p.nome).png"), [System.Drawing.Imaging.ImageFormat]::Png)
    Write-Output "recortado: $($p.nome) ($($p.w)x$($p.h))"
}

# Layout final: Música à esquerda; Sistema e Config em coluna à direita.
$margem = 24
$espaco = 20
$larg = $margem * 2 + $recortes['musica'].Width + $espaco + $recortes['sistema'].Width
$alt = $margem * 2 + [Math]::Max($recortes['musica'].Height,
    $recortes['sistema'].Height + $espaco + $recortes['config'].Height)

$final = New-Object System.Drawing.Bitmap $larg, $alt
$g = [System.Drawing.Graphics]::FromImage($final)

$area = New-Object System.Drawing.Rectangle 0, 0, $larg, $alt
$pincel = New-Object System.Drawing.Drawing2D.LinearGradientBrush $area,
    ([System.Drawing.Color]::FromArgb(255, 22, 22, 28)),
    ([System.Drawing.Color]::FromArgb(255, 10, 10, 13)), 45.0
$g.FillRectangle($pincel, $area)

$g.DrawImage($recortes['musica'], $margem, $margem)
$xDir = $margem + $recortes['musica'].Width + $espaco
$g.DrawImage($recortes['sistema'], $xDir, $margem)
$g.DrawImage($recortes['config'], $xDir, $margem + $recortes['sistema'].Height + $espaco)
$g.Dispose()

$final.Save((Join-Path $destino 'paineis.png'), [System.Drawing.Imaging.ImageFormat]::Png)
Write-Output "imagem final: paineis.png ($($final.Width)x$($final.Height))"

foreach ($r in $recortes.Values) { $r.Dispose() }
$final.Dispose()
