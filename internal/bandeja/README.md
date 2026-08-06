# internal/bandeja — o ícone ao lado do relógio (LÓGICA)

Coloca o Nimbus na **bandeja do sistema**: aqueles ícones pequenos ao lado do
relógio, às vezes escondidos atrás da setinha `^`.

| Arquivo      | O que faz |
|--------------|-----------|
| `bandeja.go` | Cria o ícone, trata o clique (liga/desliga os painéis) e o menu do botão direito (Mostrar/Esconder e Sair). |

## Por que existe uma janela invisível aqui

O Windows só entrega eventos de um ícone da bandeja **para uma janela**. Como a
janela do overlay é do ImGui/GLFW (e não controlamos as mensagens dela),
criamos aqui uma janela **invisível e sem tela** — do tipo "só mensagens" —
numa **thread separada**, com o próprio laço de mensagens. Assim ela não
atrapalha em nada o desenho do overlay.

## Como conversa com a interface (sem quebrar nada)

Mexer na interface de outra thread **não é seguro**. Então esta thread nunca
chama o ImGui: ela só levanta "bandeirinhas" (`Pedidos()`), que a interface
confere a cada quadro. No caminho de volta, a interface avisa se os painéis
estão aparecendo (`DefinirVisivel`) para o menu mostrar o texto certo.

## O ícone

Vem de `assets/nimbus.ico`, **lido do disco** (padrão do projeto: assets em
arquivo, não embutidos — dá para trocar o ícone sem recompilar). Se o arquivo
não estiver lá, usa o ícone padrão do Windows: nunca fica sem ícone.

> ⚠️ O `.ico` precisa estar no formato **clássico** (BMP dentro do ico). O
> Windows aceita PNG dentro de `.ico` em algumas APIs, mas o `LoadImage` — que
> é o usado aqui — **não** decodifica PNG e devolveria "sem ícone".
