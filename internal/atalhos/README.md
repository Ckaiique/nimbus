# atalhos — as combinações de teclas (Alt+1, Alt+2...)

Aqui mora a **decisão**: qual combinação faz o quê, como ela é lida do teclado,
e como fica guardada em disco. Este pacote **não sabe** o que cada ação faz — ele
devolve o nome da ação ("youtube") e quem executa é a interface
(`internal/ui/overlay.go`, na função `conferirAtalhos`).

| Arquivo | Para quê |
|---|---|
| `atalhos.go` | O modelo, as regras e a leitura do teclado a cada quadro |
| `teclas.go` | A tabela "número que o Windows usa" ↔ "nome que a gente escreve" |
| `arquivo.go` | Salvar e ler `%LOCALAPPDATA%\Nimbus\atalhos.txt` |
| `atalhos_test.go` | Os testes das partes que dão para testar sem teclado |

## Como o atalho funciona (e por que não é do jeito "oficial")

O overlay do Nimbus **nunca fica com o foco do teclado** — é isso que faz ele não
roubar a janela que você está usando. E uma janela sem foco não recebe aviso de
tecla nenhum.

Então a leitura é por **pergunta**, não por aviso: a cada quadro o Nimbus
pergunta ao Windows "esta tecla está apertada?" (`GetAsyncKeyState`), exatamente
como a tecla **Insert** já era lida antes disso existir.

O Windows tem uma função própria para atalho global (`RegisterHotKey`), e ela
**não** foi usada por dois motivos: exige uma janela com laço de mensagens só
dela, e **reserva a combinação para o Nimbus no PC inteiro** — se outro programa
já usa Alt+1, o registro falha em silêncio e o atalho simplesmente não funciona,
sem explicação nenhuma para o dono.

## As duas regras que protegem o uso normal do PC

1. **Todo atalho precisa de Ctrl, Alt, Shift ou Win.** Sem isso, configurar a
   tecla "1" faria o Nimbus trocar de serviço toda vez que você digitasse 1 em
   **qualquer** programa — porque o Nimbus lê o teclado do computador inteiro,
   não só o dele. Não há como saber que a tecla "não era para ele".
2. **A mesma combinação não fica em duas ações.** As duas dispariam no mesmo
   toque, na ordem em que estivessem guardadas — imprevisível. Então a nova
   **tira** a combinação de quem a tinha, como um jogo faz ao reconfigurar o
   controle.

Também ficam de fora, como tecla de atalho: os próprios modificadores e os
botões **esquerdo, direito e do meio** do mouse (virariam atalho e tomariam o
clique do PC). Os botões **extras** do mouse (4 e 5, os do polegar) valem.

## A pegadinha do "foi apertada"

O bit 1 da resposta do `GetAsyncKeyState` quer dizer *"esta tecla foi apertada
desde a última vez que alguém perguntou"* — e o Windows **apaga esse bit ao
responder**. Ou seja: a primeira pergunta **consome** o toque.

Isso quebraria dois atalhos que dividem a mesma tecla (Alt+1 e Ctrl+1): o
primeiro da lista comeria o toque do segundo. Por isso o `Conferir` pergunta
**uma vez por tecla**, guarda um "retrato" e só depois compara os atalhos com
ele.

## Onde os atalhos ficam guardados

`%LOCALAPPDATA%\Nimbus\atalhos.txt` — a mesma pasta do atualizador de listas, e
pelo mesmo motivo: a pasta do programa pode estar em "Arquivos de Programas",
onde escrever exige permissão de administrador.

O arquivo é texto e dá para ler (e consertar) no Bloco de Notas:

```
youtube = Alt+1
music = Alt+2
whatsapp = Ctrl+Shift+W
```

Duas coisas sobre ele:

- **o arquivo manda.** Se ele existe, ele diz a configuração inteira —
  inclusive "esta ação está sem atalho" (a linha não está lá). Sem isso, um
  atalho de fábrica que o dono apagou voltaria a cada vez que o Nimbus abrisse;
- **linha torta é ignorada em silêncio**, e o resto continua valendo. O programa
  nunca deixa de abrir por causa deste arquivo.

## Fallbacks

| Situação | O que acontece |
|---|---|
| Arquivo não existe (primeira vez no PC) | vale o de fábrica: Alt+1 até Alt+5 nos serviços |
| Arquivo com linha estragada | aquela linha é ignorada, as outras valem |
| Não deu para salvar (disco, permissão) | o painel Atalhos mostra o motivo em destaque, e tenta de novo no quadro seguinte |
| Ação que não existe mais no arquivo | ignorada (pode ser de uma versão antiga) |
