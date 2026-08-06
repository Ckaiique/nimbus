# internal/player — YouTube dentro do overlay (LÓGICA)

Mostra o YouTube ou o YouTube Music **dentro da própria janela do overlay**
(a janelinha expande e o site aparece na parte de baixo).

| Arquivo       | O que faz                                                      |
|---------------|-----------------------------------------------------------------|
| `embutido.go` | O truque principal: "encaixa" o **WebView2** (motor do Edge, que já vem no Windows) como filho da janela do overlay, num retângulo reservado embaixo dos controles. O ImGui nem fica sabendo — o Windows desenha o site ali. Esconder NÃO descarrega: a música continua tocando. |
| `janela_video.go` | Cria a janela PRÓPRIA que hospeda o navegador (dentro da nossa, a composição do Edge derrubava a transparência do overlay). |
| `anuncios.go` | Liga o **bloqueador de anúncios** em cada navegador criado: manda o navegador avisar de tudo que a página pede, devolve uma resposta vazia quando o endereço é de anúncio e injeta o script de limpeza. Quem DECIDE o que é anúncio é o `internal/adblock`. |
| `player.go`   | Plano B: se o encaixe falhar (PC sem WebView2), abre o site numa janelinha separada, também sempre por cima; em último caso, no navegador padrão. |

**Por que o ImGui não faz isso sozinho?** ImGui só desenha controles (botões,
sliders) — ele não tem motor de navegador. O encaixe de janela-filha é um
recurso do próprio Windows, e funciona com qualquer janela.
