# docs — imagens do README

| Arquivo | O que mostra |
|---|---|
| `paineis.png` | Os quatro painéis (Música, Sistema, Config, Atalhos) montados num fundo próprio |
| `painel-musica.png` | Só o painel Música |
| `painel-sistema.png` | Só o painel Sistema |
| `painel-config.png` | Só o painel Config |
| `painel-atalhos.png` | Só o painel Atalhos |
| `players.png` | Os quatro serviços de vídeo abertos, em grade 2x2 |
| `player-youtube.png` · `player-music.png` · `player-netflix.png` · `player-disney.png` | Cada serviço de vídeo separado |
| `player-whatsapp.png` | O WhatsApp Web aberto dentro do Nimbus (na tela do QR Code) |

## Scripts

| Script | Gera |
|---|---|
| `gerar-imagens.ps1` | As imagens dos painéis |
| `gerar-imagens-player.ps1` | As imagens dos serviços abertos |

## Como gerar de novo

Da raiz do projeto, com o programa já compilado (`compilar.bat`):

```powershell
.\docs\gerar-imagens.ps1
.\docs\gerar-imagens-player.ps1
```

Os scripts **abrem e fecham o Nimbus sozinhos**, com as abas certas à mostra e
sem transparência. Não precisa preparar nada nem deixar de mexer no computador
enquanto rodam.

O primeiro leva menos de meio minuto. O segundo leva uns dois minutos: ele abre
cada serviço e espera o site carregar (Netflix e Disney+ demoram).

## A sua privacidade nestas imagens

O Nimbus é uma janela **transparente que cobre todos os monitores**. Fotografar
"aquele pedaço da tela" significaria fotografar o que estiver embaixo. Duas
proteções cuidam disso, e as duas nasceram de problemas reais:

**1. Pedimos os pixels DA JANELA, não da tela.** Os scripts usam o
`PrintWindow` do Windows (com `PW_RENDERFULLCONTENT`), que manda a janela do
Nimbus se desenhar num bitmap nosso. Nenhuma outra janela pode entrar na imagem
— nem se estiver por cima do overlay.

> Por que isso é obrigatório: numa geração das imagens os painéis não estavam
> aparecendo na tela. O script recortou as mesmas coordenadas de sempre e as
> imagens saíram com o trabalho do dono do PC dentro, a caminho do GitHub. Foi
> pego por sorte, na conferência. Agora o risco não existe: não há como a foto
> conter outra janela.

**2. Os serviços abrem com um perfil vazio.** O `gerar-imagens-player.ps1` roda
uma **cópia do programa com outro nome** (`nimbus-fotos.exe`). O navegador
embutido guarda o perfil — logins, histórico, cookies — numa pasta derivada do
**nome do arquivo** do programa; com outro nome, ele usa um perfil novo. É por
isso que todos os sites aparecem **deslogados** nas imagens (repare no "Fazer
login" / "Entrar" em cada uma), e o WhatsApp aparece na tela do QR Code.

⚠️ **Se você fotografar na mão** (fora dos scripts), estará usando o seu perfil
de verdade e o seu monitor de verdade — confira a imagem antes de publicar.

## Onde os scripts pegam as coordenadas

Não estão escritas neles. O `gerar-imagens.ps1` abre o Nimbus com
`NIMBUS_DEBUG_PAINEIS=<arquivo>`: o programa grava ali, uma vez por segundo, o
**nome e o retângulo de cada painel que está desenhando**. O script lê dali.

São dois problemas resolvidos de uma vez:

- as coordenadas nunca envelhecem (antes, cada mudança de interface fazia o
  recorte cortar o painel pela metade);
- painel que **não** está sendo desenhado não aparece no arquivo — e aí o script
  **para** em vez de gerar uma imagem errada.
