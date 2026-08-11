# Gera as imagens dos painéis para o README.
#
# Rode da raiz do projeto:  .\docs\gerar-imagens.ps1
#
# O script abre o Nimbus sozinho (com a Config e os Atalhos à mostra e sem
# transparência), recorta cada painel, monta a imagem final e fecha o programa.
# Não precisa preparar nada.
#
# ─── PRIVACIDADE: por que NÃO fotografamos a tela ────────────────────────
#
# O Nimbus é uma janela transparente que cobre TODOS os monitores. Fotografar
# "aquele pedaço da tela" significa fotografar o que estiver embaixo — e isso já
# deu errado de verdade aqui: numa geração das imagens os painéis não estavam
# aparecendo, o script recortou as mesmas coordenadas de sempre e as imagens
# saíram com o trabalho do dono do PC dentro. Iriam direto para o GitHub.
#
# Agora pedimos ao Windows os pixels DA JANELA DO NIMBUS (`PrintWindow` com
# PW_RENDERFULLCONTENT). Ele desenha só o conteúdo daquela janela, num bitmap
# nosso. Não é uma foto da tela: nenhuma outra janela pode entrar na imagem, nem
# se estiver por cima do overlay. O risco deixou de existir em vez de "ficar
# improvável".

Add-Type -AssemblyName System.Drawing
Add-Type @"
using System;
using System.Text;
using System.Runtime.InteropServices;

public class Janelas {
  public delegate bool Proc(IntPtr h, IntPtr l);
  [DllImport("user32.dll")] public static extern bool EnumWindows(Proc p, IntPtr l);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetWindowTextW(IntPtr h, StringBuilder s, int n);
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr h, IntPtr hdc, uint flags);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);

  public struct RECT { public int L, T, R, B; }

  // PW_RENDERFULLCONTENT: manda a janela se redesenhar inteira no nosso bitmap,
  // inclusive o conteudo desenhado pela placa de video (o caso do ImGui).
  public const uint ConteudoInteiro = 2;

  public static IntPtr Achar(string titulo) {
    IntPtr achado = IntPtr.Zero;
    EnumWindows(delegate(IntPtr h, IntPtr l) {
      var sb = new StringBuilder(300);
      GetWindowTextW(h, sb, 300);
      if (sb.ToString() == titulo) { achado = h; return false; }
      return true;
    }, IntPtr.Zero);
    return achado;
  }
}
"@

$raiz = Split-Path $PSScriptRoot -Parent
$exe = Join-Path $raiz 'build\nimbus.exe'
$destino = $PSScriptRoot

# O título da janela-mãe. Vive na constante `tituloJanela`, em
# internal/ui/overlay.go — se mudar lá, muda aqui.
$tituloDaJanela = 'Nimbus Overlay'

# Os painéis que entram nas imagens, pelo apelido que o Nimbus usa (o pedaço
# depois de "###" no título da janelinha).
$paineis = @('musica', 'sistema', 'config', 'atalhos')

if (-not (Test-Path $exe)) {
    Write-Output "compile primeiro: .\compilar.bat"
    exit 1
}

# ─── de onde saem as coordenadas de cada painel ──────────────────────────
#
# O Nimbus é aberto com NIMBUS_DEBUG_PAINEIS: ele grava num arquivo, 1x por
# segundo, o NOME e o retângulo de cada painel que está desenhando.
#
# Antes esses números ficavam escritos aqui dentro, e envelheciam a cada mudança
# de interface (painel maior, serviço novo) — aí o recorte cortava o painel pela
# metade. Lendo o arquivo, as coordenadas são sempre as de agora. E de graça vem
# uma conferência: painel que não está sendo desenhado não aparece no arquivo, e
# o script para em vez de gerar uma imagem errada.
$registro = Join-Path $env:TEMP 'nimbus-paineis-fotos.txt'
if (Test-Path $registro) { Remove-Item $registro }

Get-Process -Name nimbus -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 800

$env:NIMBUS_DEBUG_PAINEIS = $registro
$env:NIMBUS_DEBUG_CONFIG = '1'
$env:NIMBUS_DEBUG_ATALHOS = '1'
$env:NIMBUS_DEBUG_ALFA = '1.0'
Start-Process $exe -WorkingDirectory $raiz
Start-Sleep -Seconds 12   # tempo de abrir, aplicar o tema e gravar o registro

function Limpar() {
    Get-Process -Name nimbus -ErrorAction SilentlyContinue | Stop-Process -Force
    Remove-Item $registro -ErrorAction SilentlyContinue
    # Limpa as variáveis: senão elas continuariam valendo no terminal de quem
    # rodou o script, e o próximo Nimbus aberto ali nasceria com as abas abertas
    # "sem motivo".
    $env:NIMBUS_DEBUG_PAINEIS = ''
    $env:NIMBUS_DEBUG_CONFIG = ''
    $env:NIMBUS_DEBUG_ATALHOS = ''
    $env:NIMBUS_DEBUG_ALFA = ''
}

function Desistir($mensagem) {
    Write-Output "PAREI: $mensagem"
    Write-Output "(nenhuma imagem foi gravada)"
    Limpar
    exit 1
}

if (-not (Test-Path $registro)) {
    Desistir "o Nimbus nao gravou o registro dos paineis."
}

# Lê a ÚLTIMA posição de cada painel (o arquivo tem uma linha por segundo).
$onde = @{}
foreach ($linha in Get-Content $registro) {
    if ($linha -match 'painel (\S+) tela=\((-?\d+),(-?\d+)\) (\d+)x(\d+)') {
        $onde[$Matches[1]] = @{
            x = [int]$Matches[2]; y = [int]$Matches[3]
            w = [int]$Matches[4]; h = [int]$Matches[5]
        }
    }
}

$faltando = $paineis | Where-Object { -not $onde.ContainsKey($_) }
if ($faltando) {
    Desistir ("estes paineis nao estavam sendo desenhados: " + ($faltando -join ', ') +
        " (se o overlay estiver escondido, aperte Insert).")
}

# ─── fotografa a JANELA do Nimbus (não a tela) ───────────────────────────
$janela = [Janelas]::Achar($tituloDaJanela)
if ($janela -eq [IntPtr]::Zero) {
    Desistir "nao achei a janela '$tituloDaJanela'."
}

$r = New-Object Janelas+RECT
[Janelas]::GetWindowRect($janela, [ref]$r) | Out-Null
$largJanela = $r.R - $r.L
$altJanela = $r.B - $r.T

$tudo = New-Object System.Drawing.Bitmap $largJanela, $altJanela
$g = [System.Drawing.Graphics]::FromImage($tudo)
$hdc = $g.GetHdc()
$deuCerto = [Janelas]::PrintWindow($janela, $hdc, [Janelas]::ConteudoInteiro)
$g.ReleaseHdc($hdc)
$g.Dispose()
if (-not $deuCerto) {
    $tudo.Dispose()
    Desistir "o Windows nao devolveu o conteudo da janela do Nimbus (PrintWindow falhou)."
}
Write-Output "fotografei a janela do Nimbus ($($largJanela)x$($altJanela))"

# ─── recorta cada painel ─────────────────────────────────────────────────
#
# As coordenadas do registro são de TELA; o bitmap é da janela. A diferença é o
# canto da janela (que num PC com monitor à esquerda é negativo, tipo -1920).
# A folga de 2 pixels para dentro evita pegar a sombra da borda.
$folga = 2
$recortes = @{}
foreach ($nome in $paineis) {
    $p = $onde[$nome]
    $area = New-Object System.Drawing.Rectangle `
        ($p.x - $r.L + $folga), ($p.y - $r.T + $folga), `
        ($p.w - $folga * 2), ($p.h - $folga * 2)

    $recortes[$nome] = $tudo.Clone($area, $tudo.PixelFormat)
    $recortes[$nome].Save((Join-Path $destino "painel-$nome.png"),
        [System.Drawing.Imaging.ImageFormat]::Png)
    Write-Output "recortado: $nome ($($area.Width)x$($area.Height))"
}
$tudo.Dispose()

# ─── monta a imagem final ────────────────────────────────────────────────
#
# Três colunas: [Música em cima de Sistema] | [Config] | [Atalhos].
#
# Escrito como uma LISTA DE COLUNAS (e não com as contas na mão, como era antes)
# porque os painéis mudam de tamanho quando a interface muda: assim a imagem se
# ajusta sozinha, sem sobra de fundo nem painel cortado.
$margem = 24
$espaco = 20
$colunas = @(
    @('musica', 'sistema'),
    @('config'),
    @('atalhos')
)

# Largura = soma da coluna mais larga de cada uma; altura = a coluna mais alta.
#
# ⚠️ As contas são feitas com [int] de propósito. O jeito "elegante"
# (`Measure-Object -Sum`) devolve **Double**, e aí o `New-Object Bitmap` falha
# com um "Parâmetro inválido" que não explica nada — já quebrou aqui.
$largurasDeColuna = @()
$somaDasLarguras = 0
$maiorAltura = 0
foreach ($coluna in $colunas) {
    $largura = 0
    $altura = 0
    foreach ($nome in $coluna) {
        if ($recortes[$nome].Width -gt $largura) { $largura = [int]$recortes[$nome].Width }
        if ($altura -gt 0) { $altura += $espaco }
        $altura += [int]$recortes[$nome].Height
    }
    $largurasDeColuna += $largura
    $somaDasLarguras += $largura
    if ($altura -gt $maiorAltura) { $maiorAltura = $altura }
}
$larg = [int]($margem * 2 + $somaDasLarguras + $espaco * ($colunas.Count - 1))
$alt = [int]($margem * 2 + $maiorAltura)

$final = New-Object System.Drawing.Bitmap $larg, $alt
$g = [System.Drawing.Graphics]::FromImage($final)

$area = New-Object System.Drawing.Rectangle 0, 0, $larg, $alt
$pincel = New-Object System.Drawing.Drawing2D.LinearGradientBrush $area,
    ([System.Drawing.Color]::FromArgb(255, 22, 22, 28)),
    ([System.Drawing.Color]::FromArgb(255, 10, 10, 13)), 45.0
$g.FillRectangle($pincel, $area)

$x = $margem
for ($i = 0; $i -lt $colunas.Count; $i++) {
    $y = $margem
    foreach ($nome in $colunas[$i]) {
        $g.DrawImage($recortes[$nome], $x, $y)
        $y += $recortes[$nome].Height + $espaco
    }
    $x += $largurasDeColuna[$i] + $espaco
}
$g.Dispose()

$final.Save((Join-Path $destino 'paineis.png'), [System.Drawing.Imaging.ImageFormat]::Png)
Write-Output "imagem final: paineis.png ($($final.Width)x$($final.Height))"

foreach ($r2 in $recortes.Values) { $r2.Dispose() }
$final.Dispose()
Limpar
Write-Output "pronto (o Nimbus foi fechado)."
