# assets — arquivos visuais

Tudo aqui é lido **do disco** quando o programa abre. Isso é de propósito
(padrão do projeto): dá para trocar uma imagem e só reabrir o Nimbus, sem
recompilar nada.

| Arquivo / pasta | Para quê |
|---|---|
| `nimbus.ico` | O ícone do Nimbus: um halo (anel) com degradê ciano → violeta. Usado na bandeja do sistema e embutido no `.exe`. Tem vários tamanhos dentro (16 a 256 pixels) — o Windows escolhe o melhor para cada lugar. |
| `nimbus.png` | O mesmo ícone em PNG (usado pelo atalho no Linux). |
| `servicos/` | As logos dos botões do player. |

## servicos/ — as logos dos botões

| Arquivo | Serviço |
|---|---|
| `youtube.png` | YouTube |
| `music.png` | YouTube Music |
| `netflix.png` | Netflix |
| `disney.png` | Disney+ |
| `whatsapp.png` | WhatsApp Web |

**O nome do arquivo importa:** tem de ser igual ao campo `Qual` na lista
`servicos` (em `internal/ui/overlay.go`) mais `.png`. É assim que o programa
acha a imagem de cada botão.

**Como converter uma logo baixada da internet:** hoje quase tudo vem em **WebP**
(às vezes até sem extensão no nome). Rode, na pasta do projeto:

```
go run ./ferramentas/converter-logo caminho\da\imagem whatsapp
```

⚠️ **Não use o Paint nem conversor de site.** O leitor de WebP do próprio Windows
falha com WebP transparente: já entregou uma logo **toda preta**. A ferramenta
acima usa o mesmo leitor que o Nimbus usa por dentro — se ficar certo nela, fica
certo no botão.

**Formato:** PNG (com ou sem fundo transparente) ou JPG. O programa desenha a
imagem **sem distorcer** — calcula o maior tamanho que cabe no botão mantendo a
proporção — e arredonda os cantos, o que dá cara de ícone de aplicativo nas
logos que têm fundo próprio (Netflix, Disney+).

**Se um arquivo faltar** ou estiver corrompido, o botão desenha a marca em vetor
(um crachá de play, um "N", um "D+", um "W"). Nunca fica um botão vazio.

## Se quiser trocar o ícone do programa

Substitua o `nimbus.ico` e reabra o Nimbus. Duas regras:

1. Precisa ser um `.ico` no formato **clássico** (BMP dentro do ico). O
   `LoadImage` do Windows, que é o usado aqui, **não** lê PNG dentro de `.ico`.
2. Para o ícone aparecer também no **arquivo `.exe`** (no Explorer), ele é
   embutido em tempo de compilação pelo `rsrc_windows_amd64.syso`, na raiz do
   projeto. Gere um novo assim:

   ```powershell
   go run github.com/akavel/rsrc@latest -ico assets\nimbus.ico -o rsrc_windows_amd64.syso
   ```

   Depois recompile (`compilar.bat`).
