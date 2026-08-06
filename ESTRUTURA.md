# Estrutura do projeto — Nimbus

> **Este arquivo é a fonte única da verdade sobre a estrutura.**
> Se criar, mover ou apagar arquivos/pastas, atualize aqui primeiro.

## O que é o projeto

O **Nimbus** é um overlay em arquitetura de **"janela fantasma"** (a mesma dos
overlays do Discord e do MSI Afterburner): uma janela invisível cobre **todos os
monitores**, e dentro dela o Dear ImGui desenha **painéis independentes** —
**Música**, **Sistema**, **Config** e **Player** — que você move e redimensiona
livremente, inclusive de uma tela para outra.

Nos espaços vazios entre os painéis o clique **atravessa** para o que estiver
atrás, como se o Nimbus não existisse.

Com ele dá para:

- controlar o **volume geral do Windows** (slider, botões − / + e mudo);
- controlar **música/vídeo**: faixa anterior, play/pause e próxima faixa
  (funciona com qualquer player — simula as teclas de mídia do teclado);
- assistir **YouTube ou YouTube Music DENTRO do overlay** (motor do Edge /
  WebView2, que já vem no Windows), com modo "só escutar";
- ver o **uso do computador**: CPU, GPU, memória RAM e os 3 processos que mais
  usam CPU, atualizado a cada 2 segundos;
- escolher entre **7 temas de cores** e ajustar a opacidade da interface.

Ele **não aparece na barra de tarefas** nem no Alt+Tab: fica discreto na
**bandeja do sistema** (os ícones pequenos ao lado do relógio). Clique no ícone
— ou a tecla **Insert** — esconde e mostra os painéis.

## Árvore de pastas

```
nimbus/
├── ESTRUTURA.md        ← este arquivo (mapa do projeto)
├── CLAUDE.md           ← instruções para a IA + as 7 regras do overlay
├── README.md           ← apresentação do projeto (é o que aparece no GitHub)
├── LICENSE             ← licença MIT (KST)
├── .gitignore          ← o que não vai para o Git (.exe, imgui.ini...)
├── compilar.bat        ← duplo clique: compila e abre o programa
├── go.mod / go.sum     ← lista de bibliotecas que o Go usa (não editar na mão)
├── rsrc_windows_amd64.syso ← o ícone embutido no .exe (gerado; ver assets/README.md)
├── main.go             ← porta de entrada: liga áudio + monitor + interface
│
├── assets/             ← arquivos visuais lidos do DISCO (não embutidos)
│   ├── README.md
│   ├── nimbus.ico      ← o ícone (halo com degradê ciano → violeta)
│   ├── nimbus.png      ← o mesmo ícone em PNG
│   └── servicos/       ← logos dos botões do player
│       ├── youtube.png
│       ├── music.png
│       ├── netflix.png
│       └── disney.png
│
├── internal/           ← LÓGICA e VISUAL, separados em pastas
│   ├── README.md
│   ├── audio/          ← som do Windows
│   │   ├── README.md
│   │   ├── volume.go   ← pegar/definir volume e mudo (com modo demonstração)
│   │   └── midia.go    ← próxima faixa / anterior / play-pause (teclas de mídia)
│   │
│   ├── bandeja/        ← ícone ao lado do relógio (bandeja do sistema)
│   │   ├── README.md
│   │   └── bandeja.go  ← ícone, clique e menu (Mostrar/Esconder, Sair)
│   │
│   ├── monitor/        ← medições do computador
│   │   ├── README.md
│   │   ├── monitor.go  ← CPU, RAM e processos mais pesados (roda em 2º plano)
│   │   └── gpu_windows.go ← uso da GPU via contadores do Windows (PDH)
│   │
│   ├── player/         ← YouTube dentro do overlay (WebView2 / motor do Edge)
│   │   ├── README.md
│   │   ├── embutido.go ← renderiza o site DENTRO da janela do overlay
│   │   └── player.go   ← plano B: janelinha separada (PC sem WebView2)
│   │
│   └── ui/             ← VISUAL (os painéis em ImGui)
│       ├── README.md
│       ├── overlay.go  ← a mecânica do overlay + os painéis
│       ├── tema.go     ← paleta derivada de 5 cores base + os 7 presets
│       ├── imagens.go  ← carrega as logos dos serviços como textura do ImGui
│       └── depurar.go  ← saída de depuração (ligada por variável de ambiente)
│
└── build/              ← programas compilados (.exe) ficam aqui
    └── README.md
```

## Regras de "onde cada coisa vai"

- **Falar com o Windows (som, teclas, medições, bandeja)** → `internal/audio/`,
  `internal/monitor/` ou `internal/bandeja/`.
- **Tudo que aparece na tela** (painéis, cores) → `internal/ui/`.
- **Imagens e ícones** → `assets/` (lidos do disco, para trocar sem recompilar).
- O `main.go` só **liga as partes** — não tem lógica própria.
- `.exe` compilado → sempre em `build/` (nunca solto na raiz).
- A `ui` conversa com a lógica **só pelas funções públicas** dos pacotes
  `audio`, `monitor` e `player` (nunca chama o Windows diretamente) — exceto a
  mecânica do próprio overlay, que é assunto de janela e vive no `overlay.go`.

## Bibliotecas de fora (e por que cada uma)

| Biblioteca | Para quê |
|---|---|
| `AllenDang/giu` | O Dear ImGui em Go (os painéis). Precisa de GCC no PC. |
| `moutend/go-wca` + `go-ole` | Falar com o Core Audio do Windows (volume/mudo). |
| `shirou/gopsutil` | CPU, RAM e lista de processos — fazer isso na mão no Windows é muito código de sistema propenso a erro. |
| `jchv/go-webview2` | O player: usa o WebView2 (motor do Edge, já instalado no Windows) para mostrar o YouTube. |

## Fallbacks (a tela nunca fica "quebrada")

- **Sem conexão com o som** → modo demonstração: o painel abre, o slider mexe,
  e o título avisa "demo".
- **GPU sem medição** (driver antigo etc.) → mostra "GPU --" e o resto segue normal.
- **Primeiros 2 segundos** → processos aparecem como "medindo..." até a primeira
  medição ficar pronta.
- **PC sem WebView2** (muito raro no Windows 11) → o YouTube abre numa janela
  separada; em último caso, no navegador padrão.
- **`assets/nimbus.ico` ausente** → a bandeja usa o ícone padrão do Windows;
  nunca fica sem ícone.
