# Gera as imagens dos serviços (YouTube, YT Music, Netflix, Disney+, WhatsApp)
# para o README.
#
# Rode da raiz do projeto:  .\docs\gerar-imagens-player.ps1
#
# ─── PRIVACIDADE, parte 1: a conta ───────────────────────────────────────
#
# O script roda uma CÓPIA do programa com outro nome (`nimbus-fotos.exe`). O
# navegador embutido guarda o perfil (logins, histórico, recomendações) numa
# pasta derivada do NOME DO ARQUIVO do programa — com outro nome ele usa um
# perfil novo e vazio. Então os sites aparecem DESLOGADOS: as imagens nunca
# mostram a sua conta.
#
# ─── PRIVACIDADE, parte 2: a foto ────────────────────────────────────────
#
# Não fotografamos "aquele pedaço da tela": pedimos ao Windows os pixels DA
# JANELA DO VÍDEO (`PrintWindow` com PW_RENDERFULLCONTENT), que desenha só o
# conteúdo daquela janela num bitmap nosso.
#
# Isso importa porque já deu errado no script dos painéis: numa geração as
# janelas não estavam aparecendo, o recorte pegou as mesmas coordenadas de
# sempre e as imagens saíram com o trabalho do dono do PC dentro — a caminho do
# GitHub. Pedindo os pixels da janela, nenhuma outra janela pode entrar na
# imagem, nem se estiver por cima.

Add-Type -AssemblyName System.Drawing
Add-Type @"
using System;
using System.Text;
using System.Runtime.InteropServices;

public class JanelaVideo {
  public delegate bool Proc(IntPtr h, IntPtr l);
  [DllImport("user32.dll")] public static extern bool EnumWindows(Proc p, IntPtr l);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetWindowTextW(IntPtr h, StringBuilder s, int n);
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr h, IntPtr hdc, uint flags);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);

  public struct RECT { public int L, T, R, B; }
  public const uint ConteudoInteiro = 2;  // PW_RENDERFULLCONTENT

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
$exeOriginal = Join-Path $raiz 'build\nimbus.exe'
$exeFotos = Join-Path $raiz 'build\nimbus-fotos.exe'
$destino = $PSScriptRoot

# O título da janela do vídeo. Vive em internal/player/janela_video.go.
$tituloDoVideo = 'Nimbus Video'

# Os serviços a fotografar. Os quatro de vídeo entram na grade 2x2 do README; o
# WhatsApp fica numa imagem separada, usada na seção dele (a legenda da grade
# fala de "serviços de vídeo", e o WhatsApp não é).
$servicos = @('youtube', 'music', 'netflix', 'disney', 'whatsapp')
$naGrade = @('youtube', 'music', 'netflix', 'disney')

if (-not (Test-Path $exeOriginal)) {
    Write-Output "compile primeiro: .\compilar.bat"
    exit 1
}

Get-Process -Name nimbus, nimbus-fotos -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 800
Copy-Item $exeOriginal $exeFotos -Force

function Limpar() {
    Get-Process -Name nimbus-fotos -ErrorAction SilentlyContinue | Stop-Process -Force
    Remove-Item $exeFotos -Force -ErrorAction SilentlyContinue
    $env:NIMBUS_DEBUG_PLAYER = ''
    $env:NIMBUS_DEBUG_ALFA = ''
}

$recortes = @{}
foreach ($servico in $servicos) {
    $env:NIMBUS_DEBUG_PLAYER = $servico
    $env:NIMBUS_DEBUG_ALFA = '1.0'

    Start-Process $exeFotos -WorkingDirectory $raiz
    Start-Sleep -Seconds 22   # tempo do site carregar (Netflix e Disney+ demoram)

    $janela = [JanelaVideo]::Achar($tituloDoVideo)
    if ($janela -eq [IntPtr]::Zero) {
        Write-Output "PULEI $servico : a janela do video nao abriu."
    }
    else {
        $r = New-Object JanelaVideo+RECT
        [JanelaVideo]::GetWindowRect($janela, [ref]$r) | Out-Null
        $bmp = New-Object System.Drawing.Bitmap ($r.R - $r.L), ($r.B - $r.T)
        $g = [System.Drawing.Graphics]::FromImage($bmp)
        $hdc = $g.GetHdc()
        $deuCerto = [JanelaVideo]::PrintWindow($janela, $hdc, [JanelaVideo]::ConteudoInteiro)
        $g.ReleaseHdc($hdc)
        $g.Dispose()

        if ($deuCerto) {
            $bmp.Save((Join-Path $destino "player-$servico.png"),
                [System.Drawing.Imaging.ImageFormat]::Png)
            $recortes[$servico] = $bmp
            Write-Output "capturado: $servico ($($bmp.Width)x$($bmp.Height))"
        }
        else {
            $bmp.Dispose()
            Write-Output "PULEI $servico : PrintWindow falhou."
        }
    }

    Get-Process -Name nimbus-fotos -ErrorAction SilentlyContinue | Stop-Process -Force
    Start-Sleep -Milliseconds 1200
}

# ─── monta a grade 2x2 com os quatro serviços de vídeo ───────────────────
$faltando = $naGrade | Where-Object { -not $recortes.ContainsKey($_) }
if ($faltando) {
    Write-Output ("nao monto a grade: faltaram " + ($faltando -join ', '))
    foreach ($r2 in $recortes.Values) { $r2.Dispose() }
    Limpar
    exit 1
}

# O tamanho da célula sai da MAIOR imagem capturada (todas nascem do mesmo
# tamanho de janela, mas ler em vez de supor evita grade torta se isso mudar).
$celulaLarg = 0
$celulaAlt = 0
foreach ($nome in $naGrade) {
    if ($recortes[$nome].Width -gt $celulaLarg) { $celulaLarg = [int]$recortes[$nome].Width }
    if ($recortes[$nome].Height -gt $celulaAlt) { $celulaAlt = [int]$recortes[$nome].Height }
}

$margem = 20
$espaco = 16
$larg = [int]($margem * 2 + $celulaLarg * 2 + $espaco)
$alt = [int]($margem * 2 + $celulaAlt * 2 + $espaco)

$final = New-Object System.Drawing.Bitmap $larg, $alt
$g = [System.Drawing.Graphics]::FromImage($final)
$ret = New-Object System.Drawing.Rectangle 0, 0, $larg, $alt
$pincel = New-Object System.Drawing.Drawing2D.LinearGradientBrush $ret,
    ([System.Drawing.Color]::FromArgb(255, 22, 22, 28)),
    ([System.Drawing.Color]::FromArgb(255, 10, 10, 13)), 45.0
$g.FillRectangle($pincel, $ret)

for ($i = 0; $i -lt $naGrade.Count; $i++) {
    $coluna = $i % 2
    $linha = [Math]::Floor($i / 2)
    $x = [int]($margem + $coluna * ($celulaLarg + $espaco))
    $y = [int]($margem + $linha * ($celulaAlt + $espaco))
    $g.DrawImage($recortes[$naGrade[$i]], $x, $y)
}
$g.Dispose()

$final.Save((Join-Path $destino 'players.png'), [System.Drawing.Imaging.ImageFormat]::Png)
Write-Output "imagem final: players.png ($($final.Width)x$($final.Height))"

foreach ($r2 in $recortes.Values) { $r2.Dispose() }
$final.Dispose()
Limpar
Write-Output "pronto."
