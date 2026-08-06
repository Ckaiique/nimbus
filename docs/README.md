# docs — imagens do README

| Arquivo | O que mostra |
|---|---|
| `paineis.png` | Os três painéis (Música, Sistema, Config) montados num fundo próprio |
| `painel-musica.png` | Só o painel Música |
| `painel-sistema.png` | Só o painel Sistema |
| `painel-config.png` | Só o painel Config |
| `players.png` | Os quatro serviços abertos, em grade 2x2 |
| `player-youtube.png` · `player-music.png` · `player-netflix.png` · `player-disney.png` | Cada serviço separado |

## Scripts

| Script | Gera |
|---|---|
| `gerar-imagens.ps1` | As imagens dos painéis |
| `gerar-imagens-player.ps1` | As imagens dos serviços abertos |

## Como gerar de novo

As imagens são **recortes de cada painel**, colados num fundo escuro criado na
hora. É de propósito: o Nimbus é transparente, então uma foto da tela mostraria
tudo o que estivesse aberto no computador. Recortando painel por painel, nada
do seu trabalho aparece na imagem.

1. Abra o Nimbus com a Config já aberta e sem transparência:

   ```powershell
   $env:NIMBUS_DEBUG_CONFIG='1'; $env:NIMBUS_DEBUG_ALFA='1.0'
   .\build\nimbus.exe
   ```

2. **Não mexa nos painéis** (o script conta com eles nas posições padrão). Se
   já os tiver movido, clique em **Recolocar painéis** na aba Config.

3. Rode o script `gerar-imagens.ps1` (nesta pasta).

## As imagens dos serviços e a sua privacidade

As imagens do YouTube, Music, Netflix e Disney+ mostram os sites **deslogados**
(repare no botão "Fazer login" em todas). Isso não é sorte: o
`gerar-imagens-player.ps1` roda uma **cópia do programa com outro nome**
(`nimbus-fotos.exe`).

Por que isso funciona: o navegador embutido guarda o perfil — logins, histórico,
cookies — numa pasta derivada do **nome do arquivo** do programa. Com outro nome,
ele usa um perfil novo e vazio. Então as imagens nunca mostram a sua conta, as
suas recomendações nem o seu histórico.

⚠️ **Se você fotografar o player na mão** (fora do script), estará usando o seu
perfil de verdade — confira a imagem antes de publicar.
