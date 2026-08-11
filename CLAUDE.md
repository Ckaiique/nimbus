# CLAUDE.md — Nimbus

## O que é

**Nimbus**: overlay em **Go + giu (Dear ImGui)** com painéis flutuantes que
controlam volume, mídia, monitoramento do PC e um player de YouTube embutido.
Arquitetura de "janela fantasma": uma janela invisível cobre todos os monitores
e o clique atravessa nos espaços vazios. Autor do projeto: **KST**.

O dono do projeto (Kaique) **não é programador**: documentação e comentários em
**PT-BR, linguagem simples**, sempre explicando o **porquê**.

## Regras deste projeto

- Siga o `ESTRUTURA.md` (fonte única da estrutura). Mudou algo? Atualize lá primeiro.
- **Lógica** em `internal/audio/`, `internal/monitor/` e `internal/player/` —
  **visual** em `internal/ui/`. Nunca misture: a `ui` só usa as funções públicas
  desses pacotes (a exceção é a mecânica da própria janela do overlay).
- Sempre manter os **fallbacks** funcionando (áudio em modo demonstração, "GPU --",
  player em janela separada). A tela nunca pode ficar em branco/quebrada.
- Binário compilado vai em `build/nimbus.exe`.
- O título da janela vive na constante `tituloJanela` (`internal/ui/overlay.go`) e
  é usado no `FindWindow` — nunca escreva esse nome solto pelo código.

## Bibliotecas usadas (e por quê)

- `github.com/AllenDang/giu` — o ImGui em Go. Precisa de **cgo** (compilador C /
  GCC instalado no PC) porque o ImGui é feito em C++.
- `github.com/moutend/go-wca` + `github.com/go-ole/go-ole` — falam com o
  **Windows Core Audio** (o sistema oficial de som do Windows) via COM.
- `github.com/shirou/gopsutil/v4` — CPU, memória e lista de processos.
- `github.com/jchv/go-webview2` — WebView2 (motor do Edge) para o player.

## Como compilar e rodar

- Jeito fácil: duplo clique no **`compilar.bat`**.
- Na mão:
  ```
  go build -ldflags "-H=windowsgui" -o build/nimbus.exe .
  build\nimbus.exe
  ```
  O `-H=windowsgui` esconde a janela preta do terminal (é um app de janela, não
  de linha de comando).

## Overlay: as 7 regras que fazem funcionar (NÃO MEXER SEM LER)

A arquitetura veio do projeto DLL do Kaique (`dll/Project/Hooks/Hooks.cpp`), que
já era testado e funcionava. Uma janela invisível cobre todas as telas e o ImGui
desenha janelinhas dentro dela. Errar **uma** destas regras quebra tudo:

1. **Estilos fixos** da janela: `WS_EX_LAYERED` (habilita a composição alpha),
   `WS_EX_TOOLWINDOW` (fora do Alt+Tab) e `WS_EX_NOACTIVATE` (não rouba foco).
2. **Transparência**: `SetLayeredWindowAttributes(alfa 255)` +
   `DwmExtendFrameIntoClientArea(margens -1)` + fundo com alfa 0.
3. **Clique atravessa**: liga/desliga só o bit `WS_EX_TRANSPARENT`.
4. **Posição do mouse injetada na mão** (`GetCursorPos`) no gancho
   `BeforeRenderHook` — que roda depois de ler os eventos e antes do `NewFrame`.
   Sem isso, em modo fantasma a janela não recebe evento de mouse nenhum e o
   ImGui nunca descobre que o cursor chegou (fica fantasma para sempre).
5. **Botões do mouse NUNCA injetados na mão.** O ImGui calcula
   `WantCaptureMouse = (janela sob o cursor || ALGUM BOTÃO PRESSIONADO)` — então
   injetar botão por `GetAsyncKeyState` (que dispara em clique em qualquer lugar
   da tela) faz o overlay engolir o mouse do PC inteiro.
6. **Posição, tamanho e topo reafirmados todo quadro** (`SetWindowPos`).
7. **`NoMouseCursorChange`**: o overlay não mexe no cursor do sistema.

### Ficar fora da barra de tarefas

Não basta `WS_EX_TOOLWINDOW`: o GLFW liga **`WS_EX_APPWINDOW`**, que faz o
oposto (força a janela na barra) e **vence** o TOOLWINDOW. Então esse bit é
**desligado** junto com a reafirmação dos estilos, a cada quadro. Além disso a
janela nasce com `MasterWindowFlagsHidden` e só é exibida com
`ShowWindow(SW_SHOWNOACTIVATE)` **depois** dos estilos aplicados — assim o
Windows nunca chega a criar o botão dela na barra de tarefas.

### Player: abrir SEMPRE entre quadros (não dentro de um)

`player.MostrarEmbutido` chama o `Embed` do WebView2, que **processa mensagens do
Windows por dentro** (ele espera o navegador inicializar). Se isso for chamado no
meio de um quadro do ImGui, uma dessas mensagens pode disparar um quadro NOVO —
e o ImGui fecha o programa com *"Forgot to call Render()"*.

Por isso quem clica no botão só **anota** o pedido (`abrirPlayer`), e a abertura
real acontece em `abrirPlayerAgora()`, chamado pelo `antesDoQuadro` — que roda
ENTRE quadros. **Nunca** volte a chamar `MostrarEmbutido` de dentro do desenho.

### Teclado no player: o NOACTIVATE precisa sair

`WS_EX_NOACTIVATE` impede a janela de receber o foco — é o que faz o overlay não
roubar a janela que você está usando. Só que o Windows entrega o **teclado
apenas para a janela ativa**. Resultado: com esse estilo, o campo de texto do
YouTube aceitava o clique mas **não deixava digitar** (não dava para fazer login
nem buscar).

Por isso o `estilosFixos()` decide o NOACTIVATE conforme `precisaTeclado()`:

- **player visível** → NOACTIVATE **sai** (clicar dá foco, o teclado funciona);
- **sem player** → NOACTIVATE **volta** (o overlay não rouba foco de ninguém).

E ao abrir o player chamamos `SetForegroundWindow` + `player.Focar()`
(`controller.MoveFocus`), para já dar para digitar sem precisar clicar duas vezes.

Se algum dia aparecer um campo de texto nos painéis do ImGui, é só incluí-lo no
`precisaTeclado()`.

### 🚫 Nunca pintar a tela inteira (três cores proibidas)

O ImGui tem três cores que ele usa para pintar **a tela toda**:

| Cor | Quando o ImGui usa |
|---|---|
| `ColDockingEmptyBg` | área vazia de um encaixe que cobre a tela |
| `ColModalWindowDimBg` | escurece tudo quando abre uma janela modal |
| `ColNavWindowingDimBg` | escurece tudo ao trocar de janela pelo teclado |

Num programa comum isso fica bonito. **Num overlay é desastre:** a tela inteira
ganha um véu escuro e só a nossa interface aparece. Foi exatamente o bug de "a
tela toda ficando meio preta" — eu tinha deixado o `ColDockingEmptyBg` com o
fundo **opaco** do tema, e as outras duas com o padrão do ImGui (cinza
semitransparente).

**As três ficam com alfa 0**, em `tema.go`. Se aparecer uma quarta cor desse
tipo numa versão futura do ImGui, ela entra na mesma lista.

Conferir com `NIMBUS_DEBUG=1`: a segunda linha `[tema]` imprime o alfa das três,
e todas têm de ser `0.00`.

### Tema: gravado no estilo PERMANENTE, não empilhado por quadro

O ImGui do giu vem com **encaixe (docking) ligado**: arrastando uma janelinha
sobre a outra, elas viram uma só com barra de abas. Só que essa janela
hospedeira é criada **dentro do `NewFrame`** — ou seja, ANTES de qualquer cor
que a gente empilhasse (`PushStyleColor`) durante o desenho. Resultado: a janela
juntada saía com a aparência crua do ImGui, com bordas e barra de título padrão.

Por isso o tema agora é **gravado no estilo permanente** (`imgui.CurrentStyle()`
com `SetColors`, `SetWindowRounding`...), chamado em `antesDoQuadro()`, que roda
antes do `NewFrame`. Assim toda janela nasce com o nosso tema, inclusive as que
o ImGui cria por conta própria.

⚠️ **Duas coisas que andam juntas** — mexer numa sem a outra quebra o visual:

1. `aplicarTemaPersistente()` tem de ser chamado **antes do NewFrame**;
2. o tema padrão do giu tem de ficar **desligado** (`janela.SetStyle(g.Style())`
   em `Rodar`), senão ele empilha o azul-acinzentado dele por cima do nosso a
   cada quadro.

Para conferir se o tema entrou: `NIMBUS_DEBUG=1` imprime uma linha `[tema]` com
a cor de fundo lida de volta do estilo.

### 🎯 A janela é 1 PIXEL MAIOR que a tela (nunca mexa nisso)

Esta única linha em `telaVirtual()` — somar 1 ao tamanho — é a correção do bug
mais difícil do projeto.

**O sintoma:** a tela inteira ficava preta (só os painéis apareciam) ao fechar a
aba do player ou recolher o painel. Reabrir consertava.

**A pista que resolveu, dada pelo usuário:** só acontecia com **um monitor só**.
Com um segundo monitor ligado, nunca acontecia.

**A causa:** o Windows tem uma otimização em que uma janela sempre-por-cima com
**exatamente** o tamanho do monitor é tratada como "tela cheia" — e nesse caminho
ele **desliga a composição**, o que mata a transparência por pixel. Com um
monitor, a nossa janela batia exatamente com ele. Com dois monitores, a área
virtual é maior que qualquer monitor, a igualdade não acontecia e o bug também
não.

**A correção:** 1 pixel a mais em cada sentido quebra a igualdade. O pixel extra
fica fora da tela e ninguém vê.

⚠️ Se alguém "arrumar" isso para o tamanho exato da tela, o bug volta — e volta
só para quem usa um monitor, o que faz parecer aleatório.

### 🏗️ O navegador tem JANELA PRÓPRIA (não fica dentro da nossa)

Esta é a decisão de arquitetura mais importante do projeto, e veio de um bug que
resistiu a três tentativas de correção.

**O sintoma:** com o player aberto, **fechar a aba ou recolher o painel deixava a
tela inteira preta** — só os painéis do ImGui apareciam. Reabrir o navegador
consertava. Os cliques continuavam atravessando normalmente, o que provou que a
janela não estava "sólida": só a transparência do desenho tinha caído.

**A causa:** o motor do Edge usa composição própria (DirectComposition). Ao ser
encaixado DENTRO da nossa janela, ele muda como o Windows compõe ela toda —
enquanto está visível, sustenta a composição; quando sai, o Windows recompõe e o
"vidro" do DWM se perde. Aí um pixel de alfa 0 passa a ser desenhado como preto.

**O que NÃO resolveu** (tentado, na ordem):

1. reafirmar a transparência a cada quadro;
2. acrescentar cor-chave como reserva (não casa: com alfa 0 o Windows zera a cor
   antes de comparar);
3. forçar a recomposição (`DwmExtendFrameIntoClientArea` 0 → -1 +
   `SWP_FRAMECHANGED`) no instante em que o vídeo sai da tela.

**O que resolveu:** dar ao navegador uma **janela própria** (`janela_video.go`),
sem moldura, "dona" da janela do overlay, fora do Alt+Tab e sempre por cima. A
interface a posiciona sobre o painel do player a cada quadro. Assim a nossa
janela **nunca** hospeda a composição do Edge e a transparência dela nunca é
afetada.

Consequências boas que vieram de brinde:

- o overlay pode manter `WS_EX_NOACTIVATE` **sempre** (nunca rouba foco): quem
  recebe o teclado é a janela do vídeo, que pode ser ativada;
- caixas de diálogo do navegador desabilitam a janela DELE, não a nossa.

⚠️ Ao mexer no player, cinco detalhes que já quebraram (ou quase):

1. `Reposicionar` recebe coordenadas de **TELA** (a janela do vídeo é de nível
   superior), enquanto os painéis do ImGui usam coordenadas contadas do canto da
   janela-mãe. A conversão (`+telaX/+telaY`) está em `janelaPlayer`.
2. `Reposicionar` **só mexe na janela quando o retângulo MUDOU**. A interface a
   chama a cada quadro (para o vídeo acompanhar a janelinha), e chamar o
   `SetWindowPos` 30–60 vezes por segundo com os mesmos valores fazia o vídeo
   **piscar sem parar** — cada chamada mexe na ordem das janelas e força
   repintura. Quando muda, a chamada também repõe a janela no topo (sem isso o
   overlay passaria na frente e engoliria os cliques da página).
   Ao esconder, o retângulo guardado é zerado — senão, se a janelinha for movida
   enquanto o vídeo está escondido, ele reapareceria no lugar antigo.
3. Há **dois níveis de visibilidade**: `nav.Show()/Hide()` (o navegador dentro da
   janela dele) e `ShowWindow(inst.janela)` (a janela na tela). O navegador fica
   visível para sempre; quem esconde é a janela. Esquecer o `nav.Show()` deixava
   a janela aparecendo **vazia**.
4. Quando o player está **juntado** com o painel Música (mesmo retângulo, é
   assim que o encaixe é detectado — `playerJuntoDaMusica`), quem posiciona o
   vídeo é o PRÓPRIO painel Música (`videoDentroDaMusica`), na área entre o
   slider e o rodapé. Nessa conta, o retângulo do painel Música é EXCLUÍDO da
   regra "painel cobre o vídeo" — sem isso o vídeo sumiria só de o mouse
   estar no slider (o vídeo mora DENTRO do painel).
5. Com o vídeo na tela, o overlay se recoloca todo quadro **logo ABAIXO da
   janela do vídeo** (`player.JanelaVisivel()`), e **NUNCA** em `HWND_TOPMOST`.
   Motivo: `HWND_TOPMOST` repetido sobe a janela acima das OUTRAS janelas
   "sempre no topo" também — como o vídeo agora só se recoloca quando o
   retângulo muda (detalhe 2), o overlay passava na frente do vídeo parado: o
   painel pintava um fundo escuro por cima ("tela meio preta") e os cliques na
   página iam para o overlay. Antes, quando os DOIS reafirmavam o topo todo
   quadro, a disputa era a causa da **piscada** constante. Entrar na ordem logo
   atrás de uma janela topmost mantém o overlay topmost também — acima de todo
   o resto, só abaixo do vídeo. (Para conferir a ordem ao vivo:
   `NIMBUS_DEBUG_ORDEM`.)

### Transparência: DUAS proteções, reafirmadas a cada quadro

O sintoma que motivou isto: **ao fechar o player, a tela inteira ficava preta** —
"como se a cor da janela invisível virasse preta", nas palavras do usuário. É
exatamente o que acontece: o navegador do Edge usa composição própria
(DirectComposition) e, enquanto está na janela, mantém a composição viva. Quando
sai, o Windows recompõe e o **"vidro" do DWM se perde** — aí um pixel de alfa 0
passa a ser desenhado como **preto opaco**. Reabrir o navegador restaurava.

`aplicarTransparencia()` responde com duas proteções ao mesmo tempo, e roda
**todo quadro**:

| Proteção | O que garante |
|---|---|
| `DwmExtendFrameIntoClientArea` (margens -1) + alfa 0 no fundo | transparência **por pixel** (é o que dá a translucidez dos painéis) |
| `LWA_COLORKEY` com a cor-chave + fundo pintado com ela | a cor-chave é invisível **mesmo sem o vidro** — rede de segurança contra a tela preta |

A cor-chave é um azul imperceptível (R0 G0 B1). Duas consequências a respeitar:

- o fundo da janela **tem de ser pintado exatamente** com ela — por isso existe
  o tipo `fundoInvisivel`, que devolve a cor **sem** multiplicar pelo alfa (o
  padrão do Go multiplicaria, e com alfa 0 o azul=1 viraria zero);
- nenhuma cor do tema pode ser igual a ela (nenhuma é).

Pior caso, se o vidro cair: os painéis perdem a translucidez (ficam opacos), mas
**nada fica preto** e o overlay continua utilizável.

Conferir com `GetLayeredWindowAttributes`: a cor-chave deve ser `0x010000`, o
alfa `255` e os sinalizadores `0x3` (COLORKEY|ALPHA).

### Caixa de diálogo: o overlay TEM de sair da frente

Sintoma real que aconteceu: ao trocar de serviço, apareceu uma caixa de diálogo
**atrás** do overlay. Como ela é modal, **desabilitou** a nossa janela — e o
Nimbus travou: não movia, não trocava de serviço, e não dava para ler nem
responder a caixa. Impasse total.

Por que: caixa modal desabilita a janela que a abriu, e o nosso overlay é
sempre-por-cima, então a caixa nasce embaixo dele.

`conferirDialogo()` usa **um sinal só**: `IsWindowEnabled(janela) == false`. É
isso que um modal faz com a janela que o abriu, e era o sintoma do travamento.

⚠️ **NÃO acrescente `GW_ENABLEDPOPUP` aqui.** Já tentei: ele responde a QUALQUER
janelinha filha nossa, e o navegador do Edge cria essas o tempo todo (dicas,
menus, prévias ao passar o mouse). O código pensava "tem diálogo", escondia o
vídeo, a janelinha sumia, mostrava de novo — e o vídeo ficava **piscando** com o
mouse em cima dele.

Enquanto durar, três coisas acontecem juntas — e as três são necessárias:

1. o overlay **sai do topo** (`HWND_NOTOPMOST`), para a caixa aparecer;
2. o clique **atravessa** sempre (é o mesmo cuidado do `g_FileDialogOpen` do
   projeto DLL: o overlay não pode engolir os cliques destinados à caixa);
3. o **vídeo se esconde**, senão cobriria a caixa (é janela do sistema).

Ao responder a caixa, tudo volta sozinho no quadro seguinte.

Como testar sem provocar um diálogo de verdade: `EnableWindow(hwnd, false)` de
fora simula exatamente esse estado. Conferido: TOPMOST cai para False e volta
para True ao reabilitar.

### Dois modos de player (opção na aba Config)

- **Econômico (padrão):** UM navegador, reaproveitado. Trocar de serviço é como
  digitar outro endereço na mesma aba: o anterior é descarregado e o som dele
  para. Gasta pouca memória.
- **"Manter cada serviço carregado":** um navegador POR serviço, criado **na
  primeira vez que aquele serviço é usado** (sob demanda — nada nasce ao abrir o
  programa). Trocar só esconde um e mostra o outro, então o que estava tocando
  continua em segundo plano. Cada navegador abre seus próprios processos, daí o
  custo de memória.

No código: o mapa `instancias` em `internal/player/embutido.go`, com a chave
sendo o nome do serviço (modo múltiplo) ou a palavra `"unico"` (modo econômico) —
é a função `chave()` que decide. Assim os dois modos usam o mesmo caminho de
código.

⚠️ **`MostrarEmbutido` só pode ser chamada ENTRE quadros** (é ela que cria
navegador, e criar processa mensagens do Windows). A `MostrarNaTela`, usada
durante o desenho, **nunca** cria nada.

⚠️ **`MostrarEmbutido` NÃO mostra o navegador** — e isso é essencial. Ela só
cria/carrega e marca o serviço em foco; quem mostra é a `MostrarNaTela`, no
desenho, **depois** de posicionar. Antes ela mostrava numa posição fixa escrita
no código e o desenho corrigia em seguida: dava uma "piscada" e o vídeo saltava
de lugar ao trocar de serviço. Duas regras que vêm daí:

- navegador novo nasce com `Hide()` e com **fundo transparente**
  (`PutDefaultBackgroundColor` alfa 0) — senão pinta um retângulo branco
  enquanto a página carrega;
- em `MostrarNaTela`, **posicionar vem antes de mostrar**. Nunca inverta.

Conferido amostrando a janela-filha a cada 150ms: a primeira vez que ela fica
visível já é na posição final (424,112), igual à posição estável.

Como "descarregar" funciona: a biblioteca não expõe como destruir um WebView2,
então `DescarregarSegundoPlano()` manda a página para `about:blank`. Isso para o
som e devolve a memória do conteúdo; o navegador fica ocioso consumindo pouco.

Para testar sem clicar: `NIMBUS_DEBUG_MULTI=1` liga o modo múltiplo e
`NIMBUS_DEBUG_TROCAR=music` troca de serviço sozinho depois de ~6s. Conferido
contando as janelas-filhas `Chrome_WidgetWin_1`: modo múltiplo = 2 navegadores
com 1 visível; econômico = 1 navegador.

### Bloqueador de anúncios (`internal/adblock` + `internal/player/anuncios.go`)

A decisão ("isto é anúncio?") mora em `internal/adblock`, como **função pura**
(`DeveBloquear(url) bool`) com listas **embutidas no código** — nada é baixado:
o Nimbus tem de funcionar offline. Quem age é `player/anuncios.go`, ligando o
filtro do WebView2 e injetando o script de limpeza em cada navegador criado.

Três coisas que **não** podem ser mexidas sem entender:

1. **Casamento por rótulo de domínio, nunca `strings.Contains`.**
   `googlesyndication.com` tem de pegar `pagead2.googlesyndication.com`, mas
   **não pode** pegar `naogoogle-analytics.com.br` — que é o site de outra
   pessoa e só por acaso tem aquelas letras. Tem teste para os dois.
2. **A lista de protegidos é uma trava de segurança.** Bloquear
   `googlevideo.com`, `nflxvideo.net` ou `dssott.com` por engano não tira um
   anúncio: **apaga o vídeo inteiro**. Quando as duas listas casam, vence a
   mais específica — é o que permite proteger `google.com` e ainda assim barrar
   `adservice.google.com`.
3. **No script da página, a trava do `ad-showing`.** O pulo do anúncio do
   YouTube adianta o vídeo até o fim; se ele rodar quando NÃO estiver passando
   anúncio, adianta o filme que a pessoa está assistindo. Só age quando o
   próprio player do YouTube diz que está em anúncio.

Honestidade obrigatória em qualquer texto sobre isso: o **anúncio de vídeo do
YouTube não dá para bloquear por endereço** (vem do mesmo servidor do vídeo) —
o que fazemos é **pular**, e o YouTube muda a técnica com frequência, então
pode parar de funcionar sem aviso. Isto é para uso pessoal, não é um uBlock
Origin.

### O vídeo tem de acompanhar a janelinha (esconder junto)

O vídeo é uma **janela-filha do Windows** e fica SEMPRE por cima do que o ImGui
desenha. Ele não sabe nada do estado da janelinha do player — então, ao recolher
a janelinha (a setinha na barra de título), esconder os painéis (Insert) ou
fechar, o navegador ficava **plantado na tela sozinho**, por cima de tudo.

A amarração usa um sinal simples: o ImGui **pula o conteúdo** de uma janela
recolhida ou fechada. Então o `g.Custom` de dentro da janelinha do player marca
`playerDesenhadoNoQuadro = true`; no fim do `desenhar()`, se a marca continuar
falsa, chamamos `player.EsconderNaTela()`. Isso cobre todos os casos de uma vez
(recolhida, escondida, fechada) sem precisar testar cada um.

No pacote player há dois estados diferentes, e não devem ser confundidos:

- `embVisivel` — o usuário **quer** ver o vídeo (a escolha dele);
- `naTela` — a área do vídeo está aparecendo **agora**.

`EsconderNaTela` mexe só no segundo: o som continua e a escolha do usuário é
preservada, então ao mostrar os painéis de novo o vídeo volta sozinho.

**O vídeo sai da frente quando um painel o cobre.** Como o vídeo é janela de
verdade do sistema, ele fica SEMPRE por cima do que o ImGui desenha — então, ao
arrastar um painel para cima da área do vídeo, os botões dele ficavam
inalcançáveis (o clique ia para o navegador).

A regra é `painelTapaOVideo()`, e ela exige **duas** condições ao mesmo tempo:
o cursor está dentro de um painel **E** esse painel se sobrepõe à área do vídeo.

Essa regra já errou duas vezes, então cuidado ao mexer:

1. **1ª tentativa** — consultava a ordem das janelas do ImGui
   (`IsWindowHovered`). Dava resultado errado sobre a área do vídeo: bastava
   apontar o mouse para ele e o vídeo desaparecia.
2. **2ª tentativa** — escondia se o cursor estivesse sobre *qualquer* painel,
   sem checar sobreposição. Apontar para o painel Música (que fica AO LADO)
   escondia o vídeo à toa.

Por isso ela virou uma **função pura** (sem estado do ImGui) com teste:
`go test ./internal/ui/` cobre os dois casos que já quebraram, mais o do painel
arrastado para cima do vídeo.

### Opacidade do vídeo: na JANELA, nunca em CSS

Hoje: `definirOpacidadeJanela()` aplica opacidade uniforme na janela do vídeo
(`SetLayeredWindowAttributes` com `LWA_ALPHA`). Simples e o Windows faz a mistura.

⚠️ **Não volte a injetar CSS na página.** Antes eu injetava
`html,body{background:transparent}` + `html{opacity:X}`, o que funcionava quando o
navegador morava dentro da janela do overlay (que tem composição transparente).
Com o navegador em janela própria, apagar o fundo da página não tem com o que
compor: o site aparecia **muito escuro**, como se houvesse um véu sobre ele.

Duas coisas que andam junto com isso:

- o fundo padrão do WebView2 é **opaco escuro** (18,18,18), não transparente —
  evita o clarão branco no carregamento sem escurecer o site;
- a janela do vídeo tem `WS_EX_LAYERED` desde a criação (é o que permite a
  opacidade).

### Opacidade: o vídeo precisa ser pedido à parte

O slider de opacidade mexe no `StyleVarAlpha` do ImGui, que vale **só para o que
o ImGui desenha**. O vídeo é uma janela-filha do motor do Edge, por cima — ele
não sabe nada da nossa opacidade.

Então `player.DefinirOpacidade` pede a transparência ao próprio navegador, em
duas partes: (1) `PutDefaultBackgroundColor` com alfa 0, para o navegador não
pintar um fundo opaco atrás da página; (2) um CSS injetado por `Eval` que deixa
a página com a opacidade escolhida. O CSS é **reaplicado a cada ~2 segundos**
porque, ao trocar de página, o site monta o HTML de novo e o nosso CSS iria
embora.

### Bandeja do sistema (`internal/bandeja`)

Roda em **outra thread**, com uma janela invisível "só mensagens" e o próprio
laço de mensagens (o Windows só entrega eventos de ícone de bandeja para uma
janela). Ela **nunca** chama o ImGui — só troca "bandeirinhas" com a interface
(`Pedidos()` / `DefinirVisivel()`), porque mexer na interface de outra thread
quebra o programa.

O ícone vem de `assets/nimbus.ico`, **lido do disco**, e precisa estar no
formato **clássico** (BMP dentro do ico): o `LoadImage` do Windows não decodifica
PNG dentro de `.ico`. O mesmo ícone está embutido no `.exe` pelo
`rsrc_windows_amd64.syso` (veja `assets/README.md` para gerar de novo).

### Duas diferenças em relação ao projeto DLL (por causa do GLFW)

- **Não usar a trava (`static cur`) do `SetClickThrough`.** No projeto DLL só ele
  mexe nos estilos; aqui o **GLFW reescreve os estilos** da janela e apaga o
  `WS_EX_TRANSPARENT`. Com a trava, o código acha que já aplicou e nunca
  reaplica → o overlay fica clicável para sempre. Comparar com o estilo **real**
  a cada quadro e só chamar o Windows quando estiver diferente.
- **Coordenadas são da TELA, não do cliente** (nada de `ScreenToClient`): o giu
  liga o modo **viewports** do ImGui, e nesse modo o viewport principal começa
  no canto da tela virtual (ex.: -1920,0). Misturar os dois espaços faz o hover
  nunca casar. Só o WebView2 do player precisa de coordenadas de cliente (ele é
  janela-filha) — aí converte-se subtraindo o canto da tela virtual.

Para depurar (variáveis de ambiente, não afetam o uso normal):

| Variável | O que faz |
|---|---|
| `NIMBUS_DEBUG=1` | imprime o que o ImGui vê do mouse, quadro a quadro |
| `NIMBUS_NOGHOST=1` | desliga o clique-atravessa (janela sempre clicável) |
| `NIMBUS_DEBUG_PLAYER=youtube` | abre o player já ao iniciar |
| `NIMBUS_DEBUG_ALFA=0.45` | começa com essa opacidade |
| `NIMBUS_DEBUG_ORDEM=<arquivo>` | grava ali, 1x por segundo, o alvo do overlay na ordem das janelas (-1 = topo absoluto; outro número = entra abaixo da janela do vídeo) |
| `NIMBUS_DEBUG_PAINEIS=<arquivo>` | grava ali, 1x por segundo, o retângulo de cada painel em coordenadas de TELA (para achar os painéis de fora do programa) |

⚠️ **Ao testar por captura de tela:** confira antes se a sessão do Windows está
**desbloqueada**. Com o PC bloqueado, a captura mostra a tela de bloqueio e os
números não têm relação nenhuma com o programa (já caí nessa).

## Serviços do player (`servicos` em `internal/ui/overlay.go`)

Os botões (YouTube, YT Music, Netflix, Disney+) mostram a **logo de verdade**,
lida de `assets/servicos/<nome>.png` (o nome do arquivo tem de ser igual ao campo
`Qual` do serviço). O carregamento está em `internal/ui/imagens.go`.

Como funciona: o ImGui não desenha PNG direto — precisa de uma textura na placa
de vídeo, e o envio é **assíncrono** (só acontece entre quadros). Então pedimos o
envio ao iniciar e, enquanto não fica pronto — ou se o arquivo faltar —, o botão
desenha a marca **em vetor** (o desenho de reserva, que nunca deixa o botão vazio).

⚠️ **Desenho manual não obedece ao `StyleVarAlpha`.** O alfa do ImGui vale só
para os widgets prontos dele. Por isso `cor32()` multiplica pelo `Alfa`, e a
imagem é desenhada com uma "tinta" branca de alfa `Alfa` — sem isso os ícones
ficariam opacos enquanto o resto do painel ficava translúcido.

Para acrescentar um serviço: inclua o endereço no mapa `enderecos`
(`internal/player/player.go`), um item na lista `servicos` e a logo em
`assets/servicos/`. As formas de reserva são `playRetangulo`, `playCirculo` e
`letra`.

**Clique esquerdo** abre dentro do Nimbus; **clique direito** abre no navegador
de verdade. O direito existe porque **Netflix e Disney+ usam DRM** (proteção de
conteúdo) e o WebView2 não traz o componente que decifra isso — o site abre, mas
o vídeo pode se recusar a tocar. No navegador normal funciona.

### Um Nimbus por vez (`internal/instancia`)

Como o Nimbus não aparece na barra de tarefas nem no Alt+Tab, é fácil abrir o
atalho achando que ele estava fechado. Dois Nimbus juntos dão bug de verdade:
dois ícones na bandeja, dois overlays reafirmando "eu fico no topo" a cada
quadro (tela piscando), duas janelas de vídeo e a tecla Insert respondendo em
dobro (um esconde, o outro mostra).

A trava é um **mutex nomeado** do Windows (`Unica()` no `main.go`): quem abre
primeiro fica com a plaquinha; o segundo recebe `ERROR_ALREADY_EXISTS`, mostra
uma caixinha explicando onde está o ícone da bandeja e encerra.

Três detalhes que não podem mudar:

1. a checagem vem **depois** do desvio do `--player` — o modo player é um
   processo FILHO do próprio Nimbus e seria bloqueado pelo pai;
2. o aviso é `MessageBox`, não `Println`: com `-H=windowsgui` não há terminal;
3. se o Windows não criar a plaquinha, o Nimbus **abre assim mesmo** (fallback).

## Pegadinhas conhecidas

- **`CoInitializeEx` "com erro" que não é erro:** a biblioteca go-ole devolve
  erro para qualquer resposta diferente de zero, mas `S_FALSE` (0x1, "o COM já
  estava aberto nesta thread") e `RPC_E_CHANGED_MODE` (0x80010106) significam
  que está tudo bem. Tratar essas duas como falha fazia o Nimbus cair no modo
  demonstração ("demo - sem som") de vez em quando, dependendo da thread em que
  o Go rodasse o `main`. Veja `codigoDoErro` em `internal/audio/volume.go`.

- A **primeira compilação demora vários minutos** (o Go compila todo o ImGui em
  C++ uma vez). Das próximas vezes é rápido, porque fica em cache.
- Precisa de **GCC** no PATH (instalado via winget: `BrechtSanders.WinLibs.POSIX.UCRT`).
- O COM do Windows (áudio) é inicializado na thread principal — as chamadas de
  volume acontecem dentro do loop da interface, que roda nessa mesma thread.
  **Não** mover as chamadas de áudio para outra goroutine sem repensar isso.
