# internal/instancia — um Nimbus por vez

## Para que serve

Impede que o Nimbus seja aberto **duas vezes ao mesmo tempo**.

## Por que isso importa

O Nimbus não aparece na barra de tarefas nem no Alt+Tab (ele mora na bandeja,
ao lado do relógio). Então é fácil clicar no atalho achando que ele estava
fechado — e o segundo Nimbus não convive bem com o primeiro:

- **dois ícones** iguais na bandeja (e o "Sair" fecha só um deles);
- **dois overlays** transparentes empilhados, cada um reafirmando "eu fico no
  topo" a cada quadro — eles brigam e a tela pisca;
- **duas janelas de vídeo** do player, uma tapando a outra;
- a tecla **Insert** responde nos dois: um esconde, o outro mostra.

## Como funciona

Pedimos ao Windows uma "plaquinha" com nome único (um *mutex* nomeado). Quem
abrir primeiro fica com ela. O segundo pede a mesma plaquinha e o Windows
responde "isso já existe" — é assim que ele descobre que já tem um Nimbus
rodando, sem precisar procurar janela nem vasculhar processos.

O Windows **apaga a plaquinha sozinho** quando o programa fecha, mesmo se ele
travar ou for encerrado pelo Gerenciador de Tarefas. Ou seja: não existe risco
de ficar uma trava fantasma que impeça o Nimbus de abrir depois.

## Arquivos

| Arquivo | O que faz |
|---|---|
| `instancia.go` | `Unica()` (tenta reservar) e `Liberar()` (devolve na saída) |

## Detalhes que já foram armadilha

- **O modo player (`--player`) fica FORA da trava.** Ele é um processo filho
  aberto pelo próprio Nimbus; se entrasse na conta, o pai bloquearia o filho.
  Por isso a checagem no `main.go` vem **depois** do desvio do `--player`.
- **O aviso é uma caixinha do Windows, não texto no terminal.** O Nimbus é
  compilado com `-H=windowsgui` e não tem janela preta — um `fmt.Println` não
  seria visto por ninguém.
- **Fallback:** se por algum motivo estranho o Windows não conseguir criar a
  plaquinha, o Nimbus **abre assim mesmo**. Um Nimbus a mais é menos ruim do
  que um Nimbus que se recusa a iniciar.
