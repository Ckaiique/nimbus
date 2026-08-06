# internal/monitor — uso do computador (LÓGICA)

Mede **CPU, GPU, memória RAM** e os **processos que mais consomem CPU**.

| Arquivo          | O que faz                                                     |
|------------------|---------------------------------------------------------------|
| `monitor.go`     | Mede tudo a cada 2 segundos, **em segundo plano**, e guarda o resultado pronto. A janela só lê o valor (função `Atual()`) — assim ela nunca trava esperando medição. |
| `gpu_windows.go` | Mede a GPU usando os contadores de desempenho do Windows (PDH) — o mesmo sistema do Gerenciador de Tarefas. Se não conseguir, devolve -1 e a janela mostra "GPU --". |

**Por que a biblioteca gopsutil?** Medir CPU por processo no Windows na mão
exige muito código de sistema propenso a erro. A gopsutil é a biblioteca padrão
da comunidade Go para isso — dependência justificada.
