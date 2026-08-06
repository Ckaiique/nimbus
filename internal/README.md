# internal — o miolo do Nimbus

Aqui ficam o "cérebro" e o visual, em pastas separadas (regra da casa: lógica e
visual nunca se misturam).

| Pasta       | Responsabilidade |
|-------------|------------------|
| `audio/`    | Volume do Windows e teclas de mídia (anterior / play-pause / próxima) |
| `monitor/`  | Medição de CPU, GPU, RAM e processos mais pesados |
| `player/`   | YouTube e YouTube Music via WebView2 (motor do Edge) |
| `bandeja/`  | Ícone na bandeja do sistema (ao lado do relógio) e seu menu |
| `ui/`       | Os painéis em ImGui e a mecânica do overlay — o único lugar com código visual |

A regra da seta: `ui` → chama → `audio`, `monitor`, `player` e `bandeja`.
**Nunca o contrário** — nenhuma dessas pastas sabe que a interface existe. A
`bandeja` roda em outra thread e só troca "bandeirinhas" com a `ui`, nunca
chama o ImGui direto (mexer na interface de outra thread quebraria o programa).
