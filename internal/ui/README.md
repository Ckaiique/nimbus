# internal/ui — os painéis do overlay (VISUAL)

Tudo que **aparece na tela** fica aqui, mais a mecânica da janela do overlay
(que é assunto de janela, não de regra de negócio). Para mexer no som chamamos
`internal/audio`; para ler as medições, `internal/monitor`; para o YouTube,
`internal/player`.

| Arquivo      | O que faz |
|--------------|-----------|
| `overlay.go` | A mecânica do overlay (as 7 regras — veja o `CLAUDE.md` da raiz) e os painéis: Música, Sistema, Config e Player. |
| `tema.go`    | O tema: você define 5 cores base (destaque, fundo, cartão, texto, borda) e as ~20 cores da interface são **calculadas** a partir delas. Traz 7 presets prontos. |
| `depurar.go` | Saída de depuração, ligada por variável de ambiente (`NIMBUS_DEBUG=1`, `NIMBUS_NOGHOST=1`). Não afeta o uso normal. |

## Por que os painéis são janelas do ImGui

Cada painel é uma janela **do próprio Dear ImGui**, desenhada dentro da janela
invisível que cobre todas as telas. Assim mover e redimensionar são recursos
nativos dele — não precisamos calcular arrasto na mão (as tentativas anteriores
de fazer isso travavam a janela num monitor só).

## Cuidado com as coordenadas

O giu liga o modo **viewports** do ImGui, e nesse modo **todas as coordenadas do
ImGui são as da TELA** (o viewport principal começa no canto da tela virtual,
ex.: -1920,0). Por isso:

- a posição do mouse é injetada **sem** converter (`GetCursorPos` direto);
- os painéis são posicionados em coordenadas de tela;
- **só** o WebView2 do player precisa de coordenadas de cliente, porque ele é
  janela-filha — aí subtraímos o canto da tela virtual.

Misturar os dois espaços faz o ImGui nunca perceber o cursor sobre os painéis.
