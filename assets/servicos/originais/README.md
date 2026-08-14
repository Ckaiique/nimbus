# originais — as imagens do jeito que foram baixadas

Aqui ficam as logos **como vieram da internet**, sem nenhum tratamento. Elas
**não são lidas pelo programa**: o Nimbus só usa os `.png` da pasta de cima
(`assets/servicos/`).

## Por que guardar isto

Porque a logo que o botão usa é o **resultado** de uma conversão, e resultado a
gente sabe refazer — desde que ainda tenha o começo. Sem estes arquivos, se um
dia a logo do botão saísse errada (borrada, com fundo preto, no tamanho errado),
o único caminho seria caçar a imagem na internet de novo e torcer para achar a
mesma. Com eles, é só rodar o conversor outra vez.

Elas ocupam menos de 30 KB no total — bem barato pelo seguro que dão.

## O que tem aqui

⚠️ **Os nomes que vieram do download mentiam sobre o formato.** Ao trazer os
arquivos para cá eles foram renomeados para o nome do serviço (o mesmo campo
`Qual` usado no resto do projeto) **e para a extensão de verdade** — descoberta
olhando os primeiros bytes de cada arquivo, que é o que os programas realmente
leem. Vale conhecer a tabela, porque o nome antigo enganava:

| Arquivo aqui | Como veio baixado | Formato de verdade |
|---|---|---|
| `youtube.ico` | `youtube.png` | ícone do Windows (`.ico`) — **não** era PNG |
| `music.png` | `music.png` | PNG mesmo (o único em que o nome batia) |
| `netflix.webp` | `netflix.png` | WebP — **não** era PNG |
| `disney.ico` | `disnei.jfif` | ícone do Windows (`.ico`) — **não** era JFIF |
| `whatsapp.webp` | `whats` (sem extensão) | WebP |

Isso não é raro: o site manda a imagem no formato dele e o nome do arquivo é só
um rótulo que o navegador chuta. Renomear para a extensão certa evita a próxima
pessoa (ou a IA) abrir o arquivo esperando uma coisa e receber outra.

## Como gerar de novo a logo de um botão

Da pasta do projeto:

```
go run ./ferramentas/converter-logo assets\servicos\originais\whatsapp.webp whatsapp
```

Isso grava `assets/servicos/whatsapp.png`, que é o arquivo que o botão lê.

⚠️ **O conversor lê PNG, JPG/JFIF e WebP — mas não lê `.ico`.** Ou seja, os dois
ícones daqui (`youtube.ico` e `disney.ico`) não passam por ele: para eles, abra
o `.ico`, exporte a maior imagem que tiver dentro como PNG num editor que
respeite transparência, e só então salve em `assets/servicos/`. Guardamos o
`.ico` mesmo assim — é o original, e é dele que qualquer refazimento parte.

⚠️ E vale o aviso de sempre: **não use o Paint nem conversor de site** para
WebP. O leitor de WebP do Windows falha com fundo transparente e já entregou
uma logo **toda preta**. Detalhes em `assets/README.md`.
