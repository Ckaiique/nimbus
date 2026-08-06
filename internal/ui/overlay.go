// Pacote ui: o overlay ImGui, seguindo FIELMENTE a receita que o Kaique já
// validou no projeto DLL dele (dll/Project/Hooks/Hooks.cpp).
//
// São 7 regras que precisam estar TODAS no lugar. Errar uma só quebra tudo:
//
//  1. ESTILOS FIXOS da janela (nunca mudam):
//     WS_EX_LAYERED   -> habilita a composição alpha (fica SEMPRE ligado)
//     WS_EX_TOOLWINDOW-> fora da barra de tarefas e do Alt+Tab
//     WS_EX_NOACTIVATE-> nunca rouba o foco de quem está na frente
//
//  2. TRANSPARÊNCIA: SetLayeredWindowAttributes(alfa 255) +
//     DwmExtendFrameIntoClientArea(margens -1) + fundo pintado com alfa 0.
//
//  3. CLIQUE ATRAVESSA: liga/desliga SÓ o bit WS_EX_TRANSPARENT, com trava
//     para não chamar o Windows quando o estado não mudou.
//
//  4. POSIÇÃO DO MOUSE injetada na mão (GetCursorPos + ScreenToClient) no
//     momento exato ANTES do NewFrame do ImGui. Sem isso, em modo fantasma a
//     janela não recebe evento de mouse nenhum e o ImGui nunca descobre que o
//     cursor chegou (o overlay ficaria fantasma para sempre).
//
//  5. BOTÕES DO MOUSE **nunca** injetados na mão. Eles chegam só pelos
//     eventos reais da janela. Motivo: o ImGui calcula
//     WantCaptureMouse = (janela sob o cursor || ALGUM BOTÃO PRESSIONADO).
//     Injetando botão por GetAsyncKeyState, um clique em QUALQUER lugar da
//     tela fazia o overlay engolir o mouse — era o bug de "não consigo mais
//     clicar em nada".
//
//  6. TOPMOST REAFIRMADO todo quadro: quando outra janela ganha foco ela sobe
//     na ordem e cobriria o overlay.
//
//  7. NoMouseCursorChange: o overlay não mexe no cursor do sistema.
package ui

import (
	"fmt"
	"image/color"
	"math"
	"syscall"
	"unsafe"

	"github.com/AllenDang/cimgui-go/imgui"
	g "github.com/AllenDang/giu"

	"nimbus/internal/audio"
	"nimbus/internal/bandeja"
	"nimbus/internal/monitor"
	"nimbus/internal/player"
)

// ─────────────────────────── chamadas ao Windows ──────────────────────────
var (
	user32               = syscall.NewLazyDLL("user32.dll")
	dwmapi               = syscall.NewLazyDLL("dwmapi.dll")
	procFindWindow       = user32.NewProc("FindWindowW")
	procGetWindowLong    = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLong    = user32.NewProc("SetWindowLongPtrW")
	procSetLayered       = user32.NewProc("SetLayeredWindowAttributes")
	procSetWindowPos     = user32.NewProc("SetWindowPos")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procGetWindowRect    = user32.NewProc("GetWindowRect")
	procShowWindow       = user32.NewProc("ShowWindow")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procScreenToClient   = user32.NewProc("ScreenToClient")
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procIsWindowEnabled     = user32.NewProc("IsWindowEnabled")
	procGetWindow           = user32.NewProc("GetWindow")
	procDwmExtendFrame   = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
)

// Índices e sinalizadores do Windows (nomes em português para ficar claro).
var (
	// GWL_EXSTYLE é um índice NEGATIVO (-20); em Go precisa desta conversão.
	indiceEstiloExt = func() uintptr { n := -20; return uintptr(n) }()
	// HWND_TOPMOST é o "handle" -1.
	semprePorCima = func() uintptr { n := -1; return uintptr(n) }()
	// HWND_NOTOPMOST é o "handle" -2: usado para o overlay SAIR do topo
	// enquanto uma caixa de diálogo precisa aparecer.
	naoPorCima = func() uintptr { n := -2; return uintptr(n) }()
)

// tituloJanela é o nome da janela-mãe no Windows. Usamos ele para ACHAR a
// janela (FindWindow) — se mudar aqui, muda em tudo, então nunca escreva o
// nome solto pelo código.
const tituloJanela = "Nimbus Overlay"

const (
	// estilos estendidos
	exLayered     = 0x00080000 // WS_EX_LAYERED
	exTransparent = 0x00000020 // WS_EX_TRANSPARENT (o clique atravessa)
	exToolWindow  = 0x00000080 // WS_EX_TOOLWINDOW
	exNoActivate  = 0x08000000 // WS_EX_NOACTIVATE
	// WS_EX_APPWINDOW faz o contrário do TOOLWINDOW: FORÇA a janela na barra
	// de tarefas, e vence o TOOLWINDOW. O GLFW liga esse bit, então temos de
	// desligá-lo — senão o Nimbus aparece na barra de tarefas.
	exAppWindow = 0x00040000

	usarAlfa  = 0x2 // LWA_ALPHA
	usarChave = 0x1 // LWA_COLORKEY

	// A COR-CHAVE: tudo pintado EXATAMENTE com ela fica invisível, e isso vale
	// mesmo sem o "vidro" do DWM. É a nossa rede de segurança contra a tela
	// preta (veja aplicarTransparencia).
	//
	// É um azul imperceptível: R=0 G=0 B=1. Nenhuma cor da interface é igual a
	// ela, então só o fundo vazio desaparece.
	//
	// No formato do Windows (COLORREF) a ordem é 0x00BBGGRR, então azul=1 fica
	// 0x00010000.
	corChaveRef = 0x00010000

	// sinalizadores do SetWindowPos
	swpNaoMover        = 0x0002 // SWP_NOMOVE
	swpNaoRedimensiona = 0x0001 // SWP_NOSIZE
	swpNaoAtivar       = 0x0010 // SWP_NOACTIVATE
	swpNaoMexerDono    = 0x0200 // SWP_NOOWNERZORDER

	teclaInsert = 0x2D // VK_INSERT (liga/desliga o overlay)
)

// identificador da nossa janela no Windows (buscado uma vez e guardado)
var idJanela uintptr

func acharIDJanela() uintptr {
	if idJanela == 0 {
		titulo, _ := syscall.UTF16PtrFromString(tituloJanela)
		idJanela, _, _ = procFindWindow.Call(0, uintptr(unsafe.Pointer(titulo)))
	}
	return idJanela
}

// telaVirtual mede o retângulo que engloba TODOS os monitores juntos.
//
// ⚠️ O tamanho devolvido é o da tela MAIS 1 PIXEL em cada lado, e isso é
// essencial — não é arredondamento.
//
// Motivo: o Windows tem uma otimização em que uma janela sempre-por-cima com
// EXATAMENTE o tamanho do monitor é tratada como "tela cheia", e aí ele
// DESLIGA a composição — o que mata a transparência e deixa a tela preta.
//
// Foi o que explicou o bug mais difícil deste projeto: a tela só ficava preta
// quando o usuário usava **um monitor só** (aí a janela batia exatamente com o
// monitor). Com dois monitores, a área virtual é maior que qualquer monitor, a
// otimização não entra e nunca acontecia.
//
// Um pixel a mais quebra a igualdade e desliga a otimização. O pixel extra fica
// fora da tela, então ninguém vê.
func telaVirtual() (x, y, larg, alt int) {
	const smX, smY, smLarg, smAlt = 76, 77, 78, 79 // SM_*VIRTUALSCREEN
	rx, _, _ := procGetSystemMetrics.Call(smX)
	ry, _, _ := procGetSystemMetrics.Call(smY)
	rl, _, _ := procGetSystemMetrics.Call(smLarg)
	ra, _, _ := procGetSystemMetrics.Call(smAlt)
	// int32 primeiro: monitores à esquerda do principal têm X negativo.
	return int(int32(rx)), int(int32(ry)), int(int32(rl)) + 1, int(int32(ra)) + 1
}

// conferirMonitores percebe quando a tela virtual mudou (monitor ligado ou
// desligado, resolução trocada) e atualiza as contas que dependem dela.
//
// Detalhe importante: as coordenadas dos painéis do ImGui são contadas do
// CANTO da janela-mãe. Se um monitor entra à ESQUERDA do principal, o canto
// muda (ex.: de 0,0 para -1920,0) e os mesmos números passam a apontar para
// outro lugar da tela — os painéis "pulariam" para o monitor novo. Por isso,
// quando o canto anda, empurramos os painéis pela mesma distância, para eles
// ficarem visualmente parados onde estavam.
func conferirMonitores() {
	vx, vy, vl, va := telaVirtual()
	if vx == telaX && vy == telaY && vl == telaLarg && va == telaAlt {
		return // nada mudou (o caso de todo quadro normal)
	}

	// Quanto o canto da janela-mãe andou.
	dx := float32(telaX - vx)
	dy := float32(telaY - vy)

	telaX, telaY, telaLarg, telaAlt = vx, vy, vl, va
	basePX, basePY = float32(-vx), float32(-vy)

	if dx == 0 && dy == 0 {
		return // só cresceu/encolheu para a direita ou para baixo: ninguém pula
	}
	for _, nome := range []string{"###musica", "###sistema", "###config", "###player"} {
		j := imgui.InternalFindWindowByName(nome)
		// O embrulho nunca é nil; o nulo fica DENTRO (CData) quando a janela
		// ainda não existe. Chamar Pos() nesse estado derruba o programa.
		if j == nil || j.CData == nil {
			continue
		}
		p := j.Pos()
		imgui.SetWindowPosStr(nome, imgui.Vec2{X: p.X + dx, Y: p.Y + dy})
	}
}

// ─────────────────────────── estado geral ─────────────────────────────────
var (
	janela *g.MasterWindow
	som    *audio.Controle
	nivel  int32 // cópia local do volume, que o slider mexe

	// Onde fica o monitor PRINCIPAL dentro da janela-mãe (as janelinhas
	// nascem nele, não no monitor mais à esquerda).
	basePX, basePY float32

	// Retângulo que a janela-mãe deve ocupar (todas as telas juntas).
	telaX, telaY, telaLarg, telaAlt int

	menuAberto    = true // Insert liga/desliga o overlay inteiro
	sistemaAberto = true
	configAberto  = false

	preparado bool // estilos fixos e transparência já aplicados?

	telaPronta       bool // a janela-mãe já cobre todas as telas?
	quadrosEsperando int
)

// Rodar abre o overlay e fica em loop até o usuário sair.
func Rodar(controle *audio.Controle) {
	som = controle
	nivel = som.Pegar()

	vx, vy, vl, va := telaVirtual()
	telaX, telaY, telaLarg, telaAlt = vx, vy, vl, va

	// SEM o modo "viewports" (desligamos logo abaixo), as coordenadas do ImGui
	// são contadas do canto da JANELA-MÃE, não da tela. Como a janela começa no
	// canto da tela virtual (ex.: -1920,0 com um monitor à esquerda), somamos o
	// deslocamento para os painéis nascerem no monitor PRINCIPAL.
	basePX, basePY = float32(-vx), float32(-vy)

	// A janela-mãe: cobre TODAS as telas, sem moldura, sempre por cima e com
	// framebuffer transparente (a transparência real, por pixel).
	// Nasce ESCONDIDA (flag Hidden) de propósito: assim aplicamos os estilos
	// antes de ela existir na tela, e o Windows nunca chega a criar o botão
	// dela na barra de tarefas. Quem a mostra é o preparar(), no 1º quadro.
	janela = g.NewMasterWindow(
		tituloJanela,
		vl, va,
		g.MasterWindowFlagsFrameless|g.MasterWindowFlagsFloating|
			g.MasterWindowFlagsTransparent|g.MasterWindowFlagsHidden,
	)
	janela.SetPos(vx, vy)
	// Fundo invisível por DOIS caminhos ao mesmo tempo: alfa 0 (vidro do DWM) e
	// a cor-chave (que funciona mesmo sem o vidro). Veja fundoInvisivel.
	janela.SetBgColor(fundoInvisivel{})

	// DESLIGA o tema padrão do giu (um azul-acinzentado que ele empilha a cada
	// quadro). Sem isso ele venceria o nosso tema, que fica gravado no estilo
	// permanente do ImGui — veja aplicarTemaPersistente().
	janela.SetStyle(g.Style())

	// DESLIGA o modo "viewports" do ImGui (o giu liga por padrão).
	//
	// O que ele faz: permite arrastar uma janelinha para FORA da janela-mãe, e
	// aí o ImGui cria uma janela de verdade do sistema para ela. Isso não
	// serve para o Nimbus: a janelinha escapa do nosso controle (sem
	// clique-atravessa, sem ficar por cima) e, se for arrastada para além da
	// borda da tela, não dá mais para pegá-la de volta.
	//
	// Desligado, os painéis ficam sempre DENTRO da janela-mãe e o ImGui os
	// mantém alcançáveis. O encaixe em abas (juntar painéis) continua
	// funcionando — é recurso separado.
	io := imgui.CurrentIO()
	io.SetConfigFlags(io.ConfigFlags() &^ imgui.ConfigFlagsViewportsEnable)

	// Fonte do Windows (Segoe UI) — mesma escolha do projeto DLL.
	g.Context.FontAtlas.SetDefaultFont("segoeui.ttf")
	g.Context.FontAtlas.SetDefaultFontSize(16)

	// Pede o envio das logos dos serviços para a placa de vídeo. É assíncrono:
	// até ficarem prontas (ou se algum arquivo faltar), os botões mostram a
	// marca desenhada em vetor.
	carregarImagensDosServicos()

	// REGRA 4: a posição do mouse tem de ser injetada ANTES do NewFrame do
	// ImGui. O gancho "beforeRender" roda exatamente nesse ponto (depois de
	// ler os eventos do sistema e antes do quadro começar). O giu já usa esse
	// gancho, então embrulhamos o dele em vez de substituir.
	back := g.Context.Backend()
	var anterior func()
	// O getter do gancho não está na interface pública do giu, mas existe no
	// tipo concreto — pegamos por type assertion para não PERDER o gancho que
	// o giu já usa (ele carrega fontes e texturas ali).
	type comGancho interface{ BeforeRenderHook() func() }
	if bc, ok := back.(comGancho); ok {
		anterior = bc.BeforeRenderHook()
	}
	back.SetBeforeRenderHook(func() {
		if anterior != nil {
			anterior()
		}
		antesDoQuadro()
	})

	janela.Run(desenhar)
}

// aplicarTransparencia liga a transparência de verdade da janela.
//
// São duas peças, e as DUAS são necessárias:
//
//	SetLayeredWindowAttributes(alfa 255) -> a janela entra no modo "em camadas"
//	DwmExtendFrameIntoClientArea(-1)     -> o Windows trata a janela como
//	                                        "vidro": o alfa de cada pixel vale
//
// ⚠️ REAFIRMAR A CADA QUADRO é obrigatório, não é exagero.
//
// Motivo real, que aconteceu: aplicando só na inicialização, ao FECHAR o player
// a tela inteira ficava preta. O navegador do Edge usa composição própria
// (DirectComposition) e, enquanto ele está na janela, mantém a composição viva.
// Quando ele sai, o Windows recompõe a janela e o "vidro" se perde — e aí um
// pixel invisível (alfa 0) passa a ser desenhado como PRETO OPACO, cobrindo a
// tela. Reabrir o navegador trazia a transparência de volta.
//
// As duas chamadas são baratas e idempotentes; chamar todo quadro garante que o
// nosso estado sempre vença, como já fazemos com os estilos da janela.
func aplicarTransparencia(id uintptr) {
	// DUAS proteções ao mesmo tempo, de propósito:
	//
	//	LWA_ALPHA    -> alfa geral 255 (a janela não fica "desbotada")
	//	LWA_COLORKEY -> a cor-chave fica invisível MESMO SEM o vidro do DWM
	//
	// A segunda é a rede de segurança: se o vidro se perder (acontece ao fechar
	// o navegador), o alfa 0 do fundo seria desenhado como PRETO — mas como o
	// fundo é pintado exatamente com a cor-chave, ele continua invisível.
	procSetLayered.Call(id, corChaveRef, 255, usarAlfa|usarChave)

	margens := struct{ esq, dir, topo, baixo int32 }{-1, -1, -1, -1}
	procDwmExtendFrame.Call(id, uintptr(unsafe.Pointer(&margens)))
}

// forcarRecomposicao pede ao Windows para REFAZER a composição da janela.
//
// Para que serve: ao fechar o navegador embutido, o Windows recompõe a janela e
// o "vidro" do DWM se perde — a tela inteira fica escura (o alfa 0 do fundo
// passa a ser desenhado). Reabrir o navegador consertava, o que mostra que o
// problema é o estado da composição, não o desenho.
//
// O truque: mexer nas margens do vidro (0 e depois -1) e avisar que a moldura
// mudou. Isso obriga o DWM a montar tudo de novo, restabelecendo a
// transparência por pixel.
func forcarRecomposicao(id uintptr) {
	semVidro := struct{ esq, dir, topo, baixo int32 }{0, 0, 0, 0}
	procDwmExtendFrame.Call(id, uintptr(unsafe.Pointer(&semVidro)))

	comVidro := struct{ esq, dir, topo, baixo int32 }{-1, -1, -1, -1}
	procDwmExtendFrame.Call(id, uintptr(unsafe.Pointer(&comVidro)))

	const swpMolduraMudou = 0x0020 // SWP_FRAMECHANGED
	const swpNaoMexerOrdem = 0x0004 // SWP_NOZORDER
	procSetWindowPos.Call(id, 0, 0, 0, 0, 0,
		swpMolduraMudou|swpNaoMover|swpNaoRedimensiona|swpNaoAtivar|swpNaoMexerOrdem)
}

// fundoInvisivel é a cor com que a janela é limpa a cada quadro.
//
// Precisa ser um tipo nosso porque o padrão do Go entrega as cores já
// multiplicadas pelo alfa — e com alfa 0 tudo viraria zero, perdendo o azul=1
// que a cor-chave precisa encontrar. Aqui devolvemos os valores crus.
type fundoInvisivel struct{}

func (fundoInvisivel) RGBA() (r, g, b, a uint32) {
	if transparenciaSimples {
		// MODO À PROVA DE FALHAS: fundo OPACO na cor-chave.
		//
		// Por que opaco: com alfa 0 o Windows zera a cor antes de comparar com
		// a cor-chave, então a chave nunca casa e o fundo aparece PRETO quando o
		// vidro do DWM se perde. Opaco, a chave casa sempre e o fundo desaparece
		// de qualquer jeito — em troca, os painéis perdem a translucidez.
		return 0, 0, 257, 0xffff
	}
	// Modo normal: alfa 0 (a transparência vem do vidro do DWM, e ela é o que
	// permite os painéis ficarem translúcidos).
	// Valores de 16 bits: 257 equivale a 1 na escala de 0 a 255.
	return 0, 0, 257, 0
}

// transparenciaSimples é a opção "Transparência simples" da aba Config.
var transparenciaSimples bool

// definirTransparenciaSimples troca o modo e aplica na hora.
func definirTransparenciaSimples(ligado bool) {
	transparenciaSimples = ligado
	janela.SetBgColor(fundoInvisivel{})
	if id := acharIDJanela(); id != 0 {
		aplicarTransparencia(id)
		forcarRecomposicao(id)
	}
}

// dialogoAberto: existe uma caixa de diálogo esperando resposta (do navegador
// ou do Windows). Atualizado a cada quadro por conferirDialogo().
var dialogoAberto bool

// conferirDialogo descobre se alguma caixa de diálogo está travando a nossa
// janela — e é por isso que este código existe:
//
// Uma caixa de diálogo "modal" DESABILITA a janela que a abriu. Como o nosso
// overlay é sempre-por-cima, a caixa aparecia ATRÁS dele: não dava para ler nem
// responder, e a janela ficava travada (sem mover, sem trocar de serviço).
//
// O sinal é UM só, e é o confiável: a janela ficar DESABILITADA. É isso que uma
// caixa modal faz com a janela que a abriu, e era o sintoma do travamento.
//
// ⚠️ NÃO usar GW_ENABLEDPOPUP aqui (já tentei e deu errado): ele responde a
// QUALQUER janelinha filha nossa, e o navegador do Edge cria essas o tempo todo
// para dicas, menus e prévias quando você passa o mouse pela página. O código
// pensava "tem diálogo", escondia o vídeo, a janelinha sumia, mostrava de novo —
// e o vídeo ficava PISCANDO enquanto o mouse andava por cima dele.
//
// Quando há mesmo um modal, o overlay sai do topo, para de capturar o mouse e
// esconde o vídeo — ou seja, sai da frente até você responder.
func conferirDialogo(id uintptr) bool {
	habilitada, _, _ := procIsWindowEnabled.Call(id)
	return habilitada == 0
}

// estilosFixos monta os estilos estendidos que a janela deve ter.
//
// O detalhe importante é o WS_EX_NOACTIVATE, que impede a janela de receber o
// foco (é o que faz o overlay não roubar a janela que você está usando). Só que
// o Windows entrega o TECLADO apenas para a janela ativa — então, com esse
// estilo, o vídeo do YouTube aceitava o clique mas não deixava DIGITAR (fazer
// login, buscar...).
//
// Solução: enquanto o player estiver aberto, tiramos o NOACTIVATE — aí clicar
// nele dá o foco normalmente e o teclado funciona. Ao fechar o player, o estilo
// volta e o overlay para de roubar foco de novo.
func estilosFixos(estiloAtual uintptr) uintptr {
	desejado := (estiloAtual | exLayered | exToolWindow) &^ exAppWindow

	if precisaTeclado() {
		desejado &^= exNoActivate
	} else {
		desejado |= exNoActivate
	}
	return desejado
}

// precisaTeclado diz se algo na NOSSA janela precisa receber o que o usuário
// digita — e hoje a resposta é sempre não.
//
// Antes o player ficava dentro da nossa janela, então para digitar (login,
// busca) tínhamos de abrir mão do WS_EX_NOACTIVATE enquanto ele estava aberto.
// Agora o vídeo tem JANELA PRÓPRIA, que recebe foco por conta dela: clicar no
// site funciona normalmente e o nosso overlay pode manter o NOACTIVATE sempre —
// ou seja, nunca rouba o foco de quem está na frente.
//
// Se algum dia aparecer um campo de texto nos painéis do ImGui, é aqui que a
// exceção volta.
func precisaTeclado() bool { return false }

// preparar aplica, uma vez, os estilos fixos e a transparência (regras 1, 2 e 7).
func preparar() {
	id := acharIDJanela()
	if id == 0 {
		return // janela ainda não existe: tenta no próximo quadro
	}

	// REGRA 1: estilos que ficam para sempre (e tira o WS_EX_APPWINDOW,
	// que o GLFW liga e que colocaria o Nimbus na barra de tarefas).
	estilo, _, _ := procGetWindowLong.Call(id, indiceEstiloExt)
	procSetWindowLong.Call(id, indiceEstiloExt, estilosFixos(estilo))

	// REGRA 2: a transparência (veja aplicarTransparencia).
	aplicarTransparencia(id)

	// REGRA 7: o overlay não mexe no cursor do sistema.
	const naoTrocarCursor = 1 << 5 // ImGuiConfigFlags_NoMouseCursorChange
	io := imgui.CurrentIO()
	io.SetConfigFlags(io.ConfigFlags() | naoTrocarCursor)

	// Agora sim: mostra a janela, SEM ativá-la (não rouba o foco de quem
	// está na frente). Com WS_EX_TOOLWINDOW já aplicado acima, ela não
	// aparece na barra de tarefas nem no Alt+Tab.
	const mostrarSemAtivar = 4 // SW_SHOWNOACTIVATE
	procShowWindow.Call(id, mostrarSemAtivar)

	preparado = true
}

// antesDoQuadro roda ANTES de cada quadro do ImGui (regras 4 e 6).
func antesDoQuadro() {
	if !preparado {
		preparar()
	}

	// Abre o player aqui, ENTRE quadros: encaixar o WebView2 processa
	// mensagens do Windows e não pode acontecer no meio de um quadro do ImGui.
	abrirPlayerAgora()

	// O tema é gravado no estilo PERMANENTE, e aqui (antes do NewFrame) é o
	// único lugar onde isso alcança também as janelas que o ImGui cria sozinho
	// — como a janela de abas que aparece ao juntar duas janelinhas numa só.
	aplicarTemaPersistente()
	id := acharIDJanela()
	if id == 0 {
		return
	}

	// A tela virtual pode MUDAR com o programa aberto: ligar ou desligar um
	// monitor, trocar a resolução. Medimos de novo a cada quadro (são 4
	// consultas baratas ao Windows). Se mudou, as variáveis são atualizadas e
	// a regra 6, logo abaixo, percebe que a janela ficou com o tamanho errado
	// e a corrige neste mesmo quadro — é assim que a UI passa a alcançar o
	// monitor novo sem precisar reabrir o programa.
	conferirMonitores()

	// REGRA 6: reafirma posição, tamanho e topo da ordem — igual ao projeto
	// DLL, que recoloca a janela todo quadro.
	//
	// Por que a posição também: o SetPos do giu/GLFW não aplicou o canto
	// negativo (-1920, 0) do meu monitor da esquerda; a janela ficava em
	// (0,0) e deixava um monitor inteiro de fora. Aí a conversão tela ->
	// janela saía errada e o ImGui nunca via o cursor sobre as janelinhas.
	// Conferimos o retângulo real e só corrigimos quando está diferente.
	// Reafirma a transparência: se ela se perder, a tela inteira fica PRETA
	// (acontece ao fechar o player — veja aplicarTransparencia).
	aplicarTransparencia(id)

	// Se tem caixa de diálogo esperando resposta, o overlay SAI do topo para
	// ela poder aparecer (senão ficaria atrás, inacessível).
	dialogoAberto = conferirDialogo(id)
	alvoNaOrdem := semprePorCima
	if dialogoAberto {
		alvoNaOrdem = naoPorCima
	} else if video := player.JanelaVisivel(); video != 0 {
		// ⚠️ Com o vídeo na tela, o overlay se coloca logo ABAIXO dele — e NÃO
		// no topo absoluto. Motivo: a janela do vídeo só se recoloca quando o
		// retângulo dela muda (recolocar todo quadro fazia o vídeo piscar).
		// Se o overlay subisse para HWND_TOPMOST todo quadro, passaria na
		// frente do vídeo parado: o painel pintava um fundo escuro por cima
		// ("tela meio preta") e os cliques nunca chegavam à página.
		// Entrar na ordem logo atrás de uma janela topmost mantém o overlay
		// topmost também — acima de todo o resto, só abaixo do vídeo.
		alvoNaOrdem = video
	}
	depurarOrdem(alvoNaOrdem)

	var r struct{ esq, topo, dir, baixo int32 }
	procGetWindowRect.Call(id, uintptr(unsafe.Pointer(&r)))
	if int(r.esq) != telaX || int(r.topo) != telaY ||
		int(r.dir-r.esq) != telaLarg || int(r.baixo-r.topo) != telaAlt {
		procSetWindowPos.Call(id, alvoNaOrdem,
			uintptr(int32(telaX)), uintptr(int32(telaY)),
			uintptr(telaLarg), uintptr(telaAlt), swpNaoAtivar)
	} else {
		procSetWindowPos.Call(id, alvoNaOrdem, 0, 0, 0, 0,
			swpNaoMover|swpNaoRedimensiona|swpNaoAtivar|swpNaoMexerDono)
	}

	// Insert liga/desliga o overlay (o bit 1 = "foi apertada desde a última
	// verificação", para contar só uma vez por toque).
	if estado, _, _ := procGetAsyncKeyState.Call(teclaInsert); estado&1 != 0 {
		menuAberto = !menuAberto
	}

	// Pedidos vindos do ícone na bandeja do sistema (clique ou menu). Eles
	// chegam de outra thread, por isso só viram "bandeirinhas" que a gente
	// confere aqui — mexer na interface de outra thread não é seguro.
	if alternar, sair := bandeja.Pedidos(); alternar || sair {
		if sair {
			janela.SetShouldClose(true)
		} else {
			menuAberto = !menuAberto
		}
	}
	bandeja.DefinirVisivel(menuAberto)

	// REGRA 4: posição do mouse, convertida da TELA para dentro da janela-mãe.
	//
	// A conversão é subtrair o canto da tela virtual (é o mesmo que o
	// ScreenToClient do projeto DLL, mas sem depender dele). O mouse e as
	// janelinhas TÊM de estar no mesmo espaço de coordenadas — quando não
	// estavam, o ImGui nunca percebia o cursor sobre os painéis.
	var p struct{ x, y int32 }
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	imgui.CurrentIO().AddMousePosEvent(
		float32(int(p.x)-telaX),
		float32(int(p.y)-telaY),
	)
}

// definirFantasma liga/desliga o clique-atravessa (regra 3).
//
// DIFERENÇA IMPORTANTE em relação ao projeto DLL: lá o SetClickThrough guarda
// o último estado numa variável e sai fora se não mudou. Aqui isso NÃO pode
// ser feito, porque a janela é criada pelo GLFW — e o GLFW reescreve os
// estilos dela em vários eventos, apagando o nosso WS_EX_TRANSPARENT. Com a
// trava, o código achava que já estava aplicado e nunca reaplicava: o overlay
// ficava clicável para sempre e engolia todos os cliques da tela.
//
// A solução: comparar com o estilo REAL da janela a cada quadro, e só chamar o
// Windows quando estiver diferente do desejado. Continua barato (uma leitura
// por quadro) e o nosso estado sempre vence.
func definirFantasma(ativo bool) {
	if semFantasma {
		ativo = false // modo de teste: força a janela sempre clicável
	}

	id := acharIDJanela()
	if id == 0 {
		return
	}
	estilo, _, _ := procGetWindowLong.Call(id, indiceEstiloExt)

	// Reafirma TAMBÉM os estilos fixos: pelo mesmo motivo (o GLFW reescreve),
	// eles podem ter sido apagados desde o quadro anterior.
	desejado := estilosFixos(estilo)
	if ativo {
		desejado |= exTransparent
	} else {
		desejado &^= exTransparent
	}
	if desejado != estilo {
		procSetWindowLong.Call(id, indiceEstiloExt, desejado)
	}
}

// ─────────────────────────── loop de desenho ──────────────────────────────
func desenhar() {
	// ESPERA a janela-mãe assumir o tamanho de TODAS as telas antes de criar
	// as janelinhas. Por que: nos primeiros quadros ela ainda mede só um
	// monitor, e o ImGui "empurra" a janelinha para dentro dessa área menor
	// (deixando só uns 19px visíveis). Como a posição inicial vale uma vez
	// só, ela ficava presa no lugar errado — quase toda fora da tela.
	if !telaPronta {
		if imgui.CurrentIO().DisplaySize().X+1 >= float32(telaLarg) {
			telaPronta = true
		} else if quadrosEsperando < 120 {
			quadrosEsperando++
			return // ainda não: não desenha nada neste quadro
		} else {
			telaPronta = true // desiste de esperar (não deixa a tela vazia)
		}
	}

	aplicarAuxiliaresDeTeste()

	// Começa o quadro assumindo que o player NÃO foi desenhado. Quem desenha
	// marca como true; no fim do quadro conferimos (veja o comentário lá).
	playerDesenhadoNoQuadro = false

	// Zera a lista de onde estão os painéis: cada um se anota de novo agora.
	retangulosDosPaineis = retangulosDosPaineis[:0]

	// Se o usuário não estiver clicando (mexendo no slider), mostramos o
	// volume REAL do Windows — ele pode ter mudado por fora (teclado, etc).
	if !g.IsMouseDown(g.MouseButtonLeft) {
		nivel = som.Pegar()
	}

	if menuAberto {
		janelaMusica()
		if sistemaAberto {
			janelaSistema()
		}
		if configAberto {
			janelaConfig()
		}
		janelaPlayer()
	}

	// O vídeo é uma janela-filha do Windows e fica SEMPRE por cima do que o
	// ImGui desenha. Ele não sabe nada do estado da janelinha, então se ela
	// foi recolhida, escondida (Insert) ou fechada, o navegador ficaria
	// plantado na tela sozinho. Aqui é onde amarramos os dois.
	// Com caixa de diálogo esperando resposta, o vídeo também tem de sair da
	// frente — senão ele cobriria a caixa (é janela do sistema, fica por cima).
	if !playerDesenhadoNoQuadro || dialogoAberto {
		player.EsconderNaTela()
	}

	// O vídeo acabou de sair da tela? Então o Windows vai recompor a janela, e é
	// nesse momento que o "vidro" costuma se perder (a tela escurece). Pedimos a
	// recomposição na hora, para ele voltar já no quadro seguinte.
	videoNaTelaAgora := player.EstaNaTela()
	if videoEstavaNaTela && !videoNaTelaAgora {
		forcarRecomposicao(acharIDJanela())
	}
	videoEstavaNaTela = videoNaTelaAgora

	// Mantém o vídeo do YouTube com a mesma opacidade da interface. Precisa
	// ser pedido ao navegador à parte: a opacidade do ImGui vale só para o que
	// o ImGui desenha (veja player.DefinirOpacidade).
	acertarOpacidadeDoVideo()

	// A condição do projeto DLL, ao final do quadro:
	//   capturando = menu aberto E o ImGui quer o mouse
	// Se não estamos capturando, o clique atravessa.
	//
	// O "&& !dialogoAberto" é o mesmo cuidado do g_FileDialogOpen do projeto
	// DLL: enquanto uma caixa de diálogo espera resposta, o overlay não pode
	// engolir cliques — eles têm de chegar à caixa.
	capturando := menuAberto && !dialogoAberto && imgui.CurrentIO().WantCaptureMouse()
	definirFantasma(!capturando)

	depurar()
}

// ─────────────────────────── janela MÚSICA ────────────────────────────────
// A organização segue o desenho do usuário: um MENU com os serviços à
// ESQUERDA (as logos, uma embaixo da outra) e os controles à direita.
func janelaMusica() {
	carregado, visivel, _ := player.Estado()

	titulo := "Musica"
	if som.Demo {
		titulo = "Musica (demo - sem som)"
	}

	// Coluna da ESQUERDA: o menu com os serviços e, no pé, a engrenagem que
	// abre o menuzinho de Sistema / Config / Sair (tudo junto, sincronizado).
	menu := make([]g.Widget, 0, len(servicos)+2)
	for i := range servicos {
		menu = append(menu, botaoServico(servicos[i]))
	}
	menu = append(menu,
		g.Custom(botaoMenuSistema),
		g.Popup("##menuSistema").Layout(
			g.Selectable("Sistema").OnClick(func() { sistemaAberto = !sistemaAberto }),
			g.Selectable("Config").OnClick(func() { configAberto = !configAberto }),
			g.Separator(),
			g.Selectable("Sair").OnClick(sair),
			// Registra o menuzinho como painel: se ele abrir por cima do
			// vídeo, o vídeo sai da frente enquanto o cursor estiver nele
			// (senão as opções ficariam escondidas atrás do vídeo).
			g.Custom(registrarPainel),
		),
	)

	// ÁREA PRINCIPAL (o retângulo "vermelho" do desenho do usuário): fica
	// livre para o vídeo, que é desenhado aqui quando a janelinha do
	// navegador está JUNTADA com este painel (arrastada para virar aba dele).
	// Separada, cada um vive na sua janelinha, como sempre.
	juntado := carregado && visivel && playerJuntoDaMusica()

	principal := []g.Widget{}
	if carregado {
		rotuloVideo := "Ver video"
		if visivel {
			rotuloVideo = "Sem video (so escutar)"
		}
		principal = append(principal,
			g.Row(
				g.Button(rotuloVideo).Size(160, 26).OnClick(alternarVideo),
				g.Style().
					SetColor(g.StyleColorText, pal.Destaque).
					To(g.Button("Parar").Size(78, 26).OnClick(fecharPlayer)),
			),
		)
	}
	if juntado {
		principal = append(principal, g.Custom(videoDentroDaMusica))
	}

	// RODAPÉ: os botões de mídia (anterior / play-pause / próxima),
	// posicionados por coordenada — cada um no seu lugar exato, centralizados
	// na área principal.
	principal = append(principal, g.Custom(rodapeDeMidia))

	g.Window(titulo+"###musica").Pos(basePX+posMusicaX, basePY+posMusicaY).Size(356, 268).Layout(
		g.Row(g.Column(menu...), g.Column(principal...)),
		// O slider de volume fica EM PÉ, colado na borda direita, sempre
		// acompanhando o painel (o traço verde do desenho do usuário) — fora
		// do caminho do vídeo.
		g.Custom(sliderVolumeVertical),
		g.Custom(func() {
			registrarPainel()
			espiarJanela()
		}),
	)
}

// sliderVolumeVertical desenha o volume em pé, rente à borda direita do
// painel Música, da altura toda da área útil. O número não cabe no slider
// fininho, então o valor aparece numa dica ao passar o mouse.
func sliderVolumeVertical() {
	imgui.SetCursorPos(imgui.Vec2{X: imgui.WindowWidth() - 26, Y: 30})
	alt := imgui.WindowHeight() - 30 - 10
	if alt < 40 {
		return // painel baixo demais para o slider
	}
	if imgui.VSliderIntV("##volume", imgui.Vec2{X: 18, Y: alt},
		&nivel, 0, 100, "", 0) {
		som.Definir(nivel)
	}
	if imgui.IsItemHovered() {
		imgui.SetTooltip(fmt.Sprintf("Volume: %d", nivel))
	}
}

// ─────────────────────────── janela SISTEMA ───────────────────────────────
func janelaSistema() {
	medida := monitor.Atual()

	linhas := []g.Widget{
		barraDeUso("CPU", medida.CPU),
		barraDeUso("GPU", medida.GPU),
		barraDeUso("RAM", medida.RAM),
		g.Separator(),
		textoFraco("Processos que mais usam CPU:"),
	}
	if len(medida.Processos) == 0 {
		linhas = append(linhas, textoFraco("  medindo..."))
	}
	for _, p := range medida.Processos {
		linhas = append(linhas,
			textoFraco(fmt.Sprintf("%-24s %5.1f%%", curto(p.Nome, 24), p.CPU)))
	}

	linhas = append(linhas, g.Custom(registrarPainel))

	g.Window("Sistema###sistema").IsOpen(&sistemaAberto).
		Pos(basePX+posSistemaX, basePY+posSistemaY).Size(310, 250).Layout(linhas...)
}

// ─────────────────────────── janela CONFIG ────────────────────────────────
func janelaConfig() {
	nomes := make([]string, len(Presets))
	for i, p := range Presets {
		nomes[i] = p.Nome
	}

	g.Window("Config###config").IsOpen(&configAberto).
		// Altura 510: o conteúdo da Config cresceu (tema, opacidade, dois
		// botões, TRÊS opções e a dica do Insert). Com menos que isso vira
		// barra de rolagem e o último item fica escondido. Se acrescentar
		// item, aumente aqui.
		Pos(basePX+posConfigX, basePY+posConfigY).Size(300, 510).Layout(
		textoFraco("Tema de cores"),
		g.Combo("##tema", nomes[PresetAtual], nomes, &PresetAtual).Size(-1),

		g.Dummy(1, 4),
		textoFraco("Opacidade da interface"),
		g.SliderFloat(&Alfa, 0.30, 1.00).Size(-1).Format("%.2f"),

		g.Dummy(1, 8),
		g.Button("Restaurar padrao").Size(-1, 28).OnClick(func() {
			PresetAtual, Alfa = 5, 0.92 // Spotify, quase opaco
		}),

		g.Dummy(1, 4),
		// Rede de segurança: se um painel for arrastado para um canto ruim da
		// tela (ou ficar recolhido em lugar difícil), este botão traz todos de
		// volta para as posições originais.
		g.Button("Recolocar paineis").Size(-1, 26).OnClick(recolocarPaineis),

		g.Separator(),

		// Escolha entre "um navegador por serviço" e "um só".
		g.Checkbox("Manter cada servico carregado", &manterCarregado).
			OnChange(func() { player.DefinirModoMultiplo(manterCarregado) }),
		textoFraco(explicacaoDoModo()),

		g.Dummy(1, 4),

		// Bloqueador de anúncios dos sites abertos aqui dentro.
		g.Checkbox("Bloquear anuncios", &bloquearAnuncios).
			OnChange(func() { player.DefinirBloqueioDeAnuncios(bloquearAnuncios) }),
		textoFraco("Corta banners e rastreadores. No"),
		textoFraco("YouTube, pula o anuncio sozinho."),

		g.Dummy(1, 4),

		// Saída de emergência para o caso de a tela escurecer.
		g.Checkbox("Transparencia simples", &transparenciaSimples).
			OnChange(func() { definirTransparenciaSimples(transparenciaSimples) }),
		textoFraco("Ligue se a tela escurecer. Painéis"),
		textoFraco("ficam opacos, mas nunca falha."),
		g.Custom(registrarPainel),

		g.Dummy(1, 6),
		textoFraco("Insert = esconder/mostrar os paineis."),
		textoFraco("Arraste pela barra de titulo;"),
		textoFraco("estique pelo canto de baixo."),
	)
}

// ─────────────────────────── janela PLAYER ────────────────────────────────
// Uma janelinha do ImGui que o vídeo do YouTube "acompanha": a cada quadro
// lemos onde ela está e mandamos o navegador embutido para lá.
var (
	playerJanelaAberta = true

	// videoEstavaNaTela guarda se o vídeo estava aparecendo no quadro anterior,
	// para percebermos o momento exato em que ele SAI da tela.
	videoEstavaNaTela bool

	// playerDesenhadoNoQuadro marca se o conteúdo da janelinha do player foi
	// desenhado neste quadro. Serve de sinal para esconder o vídeo quando a
	// janelinha é recolhida, escondida (tecla Insert) ou fechada — veja o
	// final da função desenhar().
	playerDesenhadoNoQuadro bool
)

func janelaPlayer() {
	carregado, visivel, qual := player.Estado()
	if !carregado || !visivel {
		return
	}

	titulo := rotuloDoServico(qual)

	playerJanelaAberta = true

	// Auxiliar de teste: recolhe a janelinha sozinho, para conferir se o
	// vídeo se esconde junto (NIMBUS_DEBUG_RECOLHER=1).
	if recolherSozinho() {
		imgui.SetNextWindowCollapsedV(true, imgui.CondAlways)
	}

	// A janelinha do player é SÓ o navegador: o vídeo ocupa a área útil
	// inteira (quem organiza serviços e botões é o painel Música).
	g.Window(titulo+"###player").IsOpen(&playerJanelaAberta).
		Pos(basePX+posPlayerX, basePY+posPlayerY).Size(560, 380).Layout(
		g.Custom(func() {
			// Este trecho SÓ roda quando a janelinha está de fato aberta e
			// não recolhida (o ImGui pula o conteúdo de janela recolhida).
			// É justamente esse o sinal que usamos: se não passou por aqui,
			// o vídeo tem de se esconder.
			espiarPlayer()

			pos := imgui.WindowPos()
			tam := imgui.WindowSize()

			// O vídeo só sai da frente se o cursor estiver sobre um painel que
			// de fato COBRE parte dele (veja painelTapaOVideo). Assim, apontar
			// para o painel Música — que fica ao lado — não esconde nada.
			if painelTapaOVideo(imgui.CurrentIO().MousePos(), pos, tam, retangulosDosPaineis) {
				return
			}

			playerDesenhadoNoQuadro = true

			// Cola o vídeo na área útil da janelinha (abaixo da barra de
			// título). Agora o vídeo tem JANELA PRÓPRIA, então precisa de
			// coordenadas de TELA: somamos o canto da tela virtual (as
			// coordenadas do ImGui são contadas do canto da janela-mãe).
			player.MostrarNaTela(
				int32(pos.X)+int32(telaX)+4, int32(pos.Y)+int32(telaY)+32,
				int32(tam.X)-8, int32(tam.Y)-38,
			)
		}),
	)
	if !playerJanelaAberta {
		fecharPlayer() // clicou no X da janelinha: para o som e some
	}
}

// playerJuntoDaMusica diz se a janelinha do player está ENCAIXADA com o
// painel Música (arrastada para dentro, virando aba da mesma janela). O
// encaixe do ImGui deixa as duas com o MESMO retângulo — é assim que dá para
// perceber, com a folga de alguns pixels do quaseIgual.
func playerJuntoDaMusica() bool {
	m := imgui.InternalFindWindowByName("###musica")
	p := imgui.InternalFindWindowByName("###player")
	// ⚠️ O embrulho Go NUNCA é nil — quando a janela não existe (primeiros
	// quadros), o nulo fica DENTRO (CData). Testar só o embrulho derrubava o
	// programa ao abrir um serviço.
	if m == nil || m.CData == nil || p == nil || p.CData == nil {
		return false
	}
	mp, pp := m.Pos(), p.Pos()
	mt, pt := m.Size(), p.Size()
	return quaseIgual(mp.X, pp.X) && quaseIgual(mp.Y, pp.Y) &&
		quaseIgual(mt.X, pt.X) && quaseIgual(mt.Y, pt.Y)
}

// videoDentroDaMusica desenha o vídeo DENTRO do painel Música, na área entre
// o slider e o rodapé (a "área principal" do desenho do usuário). Só é usada
// quando o player está juntado com o painel (veja janelaMusica).
func videoDentroDaMusica() {
	pos := imgui.WindowPos()
	tam := imgui.WindowSize()
	cur := imgui.CursorScreenPos() // logo abaixo do slider e da linha de vídeo

	vx := pos.X + 4 + largBotaoServico + 8
	vy := cur.Y + 4
	vl := pos.X + tam.X - 34 - vx // para antes do slider de volume em pé
	va := pos.Y + tam.Y - 52 - vy // até a faixa do rodapé

	// Painel pequeno demais para caber vídeo: fica só o som.
	if vl < 60 || va < 60 {
		return
	}

	// O vídeo sai da frente se o cursor estiver sobre OUTRO painel cobrindo
	// ele. O próprio painel Música (e a aba do player, que tem o mesmo
	// retângulo) fica de fora da conta — senão o vídeo sumiria só de o mouse
	// estar no slider.
	outros := make([][4]float32, 0, len(retangulosDosPaineis))
	for _, r := range retangulosDosPaineis {
		if quaseIgual(r[0], pos.X) && quaseIgual(r[1], pos.Y) &&
			quaseIgual(r[2], tam.X) && quaseIgual(r[3], tam.Y) {
			continue
		}
		outros = append(outros, r)
	}
	if painelTapaOVideo(imgui.CurrentIO().MousePos(),
		imgui.Vec2{X: vx, Y: vy}, imgui.Vec2{X: vl, Y: va}, outros) {
		return
	}

	playerDesenhadoNoQuadro = true
	player.MostrarNaTela(
		int32(vx)+int32(telaX), int32(vy)+int32(telaY),
		int32(vl), int32(va),
	)
}

// manterCarregado é a opção "um navegador por serviço" (aba Config). Começa
// desligada: o padrão é o modo econômico, de um navegador só.
var manterCarregado bool

// bloquearAnuncios é a opção "Bloquear anuncios" (aba Config). Começa LIGADA,
// e o valor inicial tem de ser o MESMO do player.BloquearAnuncios — senão a
// caixinha mostraria uma coisa e o programa faria outra.
var bloquearAnuncios = player.BloquearAnuncios

// explicacaoDoModo escreve, em uma linha, o que a opção faz agora.
func explicacaoDoModo() string {
	if !manterCarregado {
		return "Trocar de servico descarrega o anterior."
	}
	carregados := player.QuantosCarregados()
	if carregados <= 1 {
		return "O que estava tocando continua em 2o plano."
	}
	return fmt.Sprintf("%d servicos carregados (usa mais memoria).", carregados)
}

// recolocarPaineis traz todos os painéis de volta para o lugar original e os
// desdobra (caso estejam recolhidos).
//
// O ImGui identifica a janela pelo que vem DEPOIS de "###", então "###musica"
// acha a janelinha mesmo quando o título visível muda (o painel Música troca
// de título no modo demonstração, por exemplo).
func recolocarPaineis() {
	lugares := map[string][2]float32{
		"###musica":  {posMusicaX, posMusicaY},
		"###sistema": {posSistemaX, posSistemaY},
		"###config":  {posConfigX, posConfigY},
		"###player":  {posPlayerX, posPlayerY},
	}
	for nome, p := range lugares {
		imgui.SetWindowPosStr(nome, imgui.Vec2{X: basePX + p[0], Y: basePY + p[1]})
		imgui.SetWindowCollapsedStr(nome, false)
	}
	// Garante que os painéis estejam à mostra (não adianta recolocar escondido).
	menuAberto = true
	sistemaAberto = true
}

// Posições originais dos painéis, contadas do canto do monitor principal.
const (
	posMusicaX, posMusicaY   = 80, 80
	posSistemaX, posSistemaY = 80, 398
	posConfigX, posConfigY   = 410, 398
	posPlayerX, posPlayerY   = 420, 80
)

// quaseIgual compara dois tamanhos/posições com folga de alguns pixels (o
// encaixe do ImGui pode deixar uma diferença de 1 ou 2).
func quaseIgual(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= 4
}

// retangulosDosPaineis guarda onde cada painel (menos o player) está neste
// quadro. É zerado no começo de cada quadro e preenchido por registrarPainel.
var retangulosDosPaineis [][4]float32

// registrarPainel anota o retângulo do painel atual. Chamar de DENTRO de cada
// painel que NÃO é o player.
func registrarPainel() {
	pos := imgui.WindowPos()
	tam := imgui.WindowSize()
	retangulosDosPaineis = append(retangulosDosPaineis,
		[4]float32{pos.X, pos.Y, tam.X, tam.Y})
	depurarPainel(pos, tam)
}

// painelTapaOVideo diz se o cursor está sobre um painel que REALMENTE cobre
// parte da área do vídeo.
//
// Para que serve: o vídeo é janela de verdade do Windows e fica sempre por cima
// do que o ImGui desenha. Se um painel for arrastado para cima dele, os botões
// daquele painel ficariam inalcançáveis — então o vídeo sai da frente enquanto
// o cursor estiver ali.
//
// As DUAS condições são necessárias, e é aí que estava o erro das versões
// anteriores:
//
//	1. o cursor está dentro do painel  (só isso escondia o vídeo à toa: bastava
//	   apontar para o painel Música, que fica AO LADO, e o vídeo sumia);
//	2. esse painel se sobrepõe à área do vídeo (é o que faz o vídeo estorvar).
//
// Função pura, sem estado do ImGui, justamente para poder ser testada —
// veja overlay_test.go.
func painelTapaOVideo(mouse, videoPos, videoTam imgui.Vec2, paineis [][4]float32) bool {
	for _, p := range paineis {
		px, py, plarg, palt := p[0], p[1], p[2], p[3]

		// Painel ENCAIXADO junto com o player (viraram abas de uma janela só):
		// ele ocupa o mesmo retângulo, e isso não é "cobrir o vídeo" — é ser a
		// mesma janela. Sem esta exceção o vídeo ficava piscando quando o mouse
		// andava sobre ele, porque a regra achava que havia um painel na frente.
		if quaseIgual(px, videoPos.X) && quaseIgual(py, videoPos.Y) &&
			quaseIgual(plarg, videoTam.X) && quaseIgual(palt, videoTam.Y) {
			continue
		}

		cursorNoPainel := mouse.X >= px && mouse.X <= px+plarg &&
			mouse.Y >= py && mouse.Y <= py+palt
		if !cursorNoPainel {
			continue
		}

		// Sobreposição de retângulos: eles se cruzam nos dois sentidos?
		cruzaEmX := px < videoPos.X+videoTam.X && px+plarg > videoPos.X
		cruzaEmY := py < videoPos.Y+videoTam.Y && py+palt > videoPos.Y
		if cruzaEmX && cruzaEmY {
			return true
		}
	}
	return false
}

// ─────────────────────── botões dos serviços ──────────────────────────────

// servico é um site que o player sabe abrir, com o desenho da sua marca.
//
// Os ícones são DESENHADOS aqui em vetor (retângulos, círculos, triângulos e
// letras) em vez de vir de arquivos de imagem. Por quê: não precisamos
// distribuir figuras de marcas de terceiros, o desenho fica nítido em qualquer
// tamanho de tela, e não há arquivo para se perder.
type servico struct {
	Qual   string // o nome curto que o player entende
	Rotulo string // o nome que aparece na dica ao passar o mouse
	Marca  color.RGBA
	Forma  string // "playRetangulo", "playCirculo" ou "letra"
	Letra  string // usado quando Forma == "letra"
}

var servicos = []servico{
	{"youtube", "YouTube", rgb(255, 0, 0), "playRetangulo", ""},
	{"music", "YouTube Music", rgb(255, 0, 0), "playCirculo", ""},
	{"netflix", "Netflix", rgb(229, 9, 20), "letra", "N"},
	{"disney", "Disney+", rgb(90, 175, 255), "letra", "D+"},
}

// Tamanho de cada botão de serviço, em pixels.
const (
	largBotaoServico = 56
	altBotaoServico  = 40
)

// botaoMenuSistema é o botão com a ENGRENAGEM no pé do menu lateral: abre o
// menuzinho com Sistema, Config e Sair. A engrenagem é desenhada à mão (um
// anel com seis dentes) porque a fonte do ImGui não tem esse símbolo.
func botaoMenuSistema() {
	const altura = 32

	canto := imgui.CursorScreenPos()
	imgui.InvisibleButton("menu_sistema", imgui.Vec2{X: largBotaoServico, Y: altura})
	sobre := imgui.IsItemHovered()

	desenho := imgui.WindowDrawList()
	fim := imgui.Vec2{X: canto.X + largBotaoServico, Y: canto.Y + altura}
	fundo := pal.Botao
	if sobre {
		fundo = pal.BotaoHover
	}
	desenho.AddRectFilledV(canto, fim, cor32(fundo), 8, 0)

	cx := canto.X + largBotaoServico/2
	cy := canto.Y + altura/2
	cor := cor32(pal.TextoFraco)
	if sobre {
		cor = cor32(pal.Texto)
	}
	for i := 0; i < 6; i++ {
		ang := float64(i) * math.Pi / 3
		sen, cos := float32(math.Sin(ang)), float32(math.Cos(ang))
		desenho.AddLineV(
			imgui.Vec2{X: cx + cos*4, Y: cy + sen*4},
			imgui.Vec2{X: cx + cos*9, Y: cy + sen*9},
			cor, 3)
	}
	desenho.AddCircleV(imgui.Vec2{X: cx, Y: cy}, 5.5, cor, 0, 2.5)

	if sobre {
		imgui.SetTooltip("Sistema, Config e sair")
	}
	if imgui.IsItemClicked() {
		imgui.OpenPopupStr("##menuSistema")
	}
}

// botaoServico desenha um botão com a marca do site.
//
// Clique com o botão ESQUERDO abre dentro do Nimbus; com o DIREITO abre no
// navegador de verdade — útil para Netflix e Disney+, que podem se recusar a
// tocar vídeo dentro de um navegador embutido (proteção de conteúdo/DRM).
func botaoServico(s servico) g.Widget {
	return g.Custom(func() {
		canto := imgui.CursorScreenPos()
		imgui.InvisibleButton("servico_"+s.Qual, imgui.Vec2{
			X: largBotaoServico, Y: altBotaoServico,
		})
		sobre := imgui.IsItemHovered()

		desenho := imgui.WindowDrawList()
		fim := imgui.Vec2{X: canto.X + largBotaoServico, Y: canto.Y + altBotaoServico}

		// Fundo do botão: escuro, clareando quando o mouse passa em cima.
		fundo := pal.Botao
		if sobre {
			fundo = pal.BotaoHover
		}
		desenho.AddRectFilledV(canto, fim, cor32(fundo), 8, 0)

		// Se este site está carregado no player, ganha um risco de destaque
		// embaixo — dá para ver de relance qual está ativo.
		if carregado, _, atual := player.Estado(); carregado && atual == s.Qual {
			desenho.AddRectFilledV(
				imgui.Vec2{X: canto.X + 10, Y: fim.Y - 4},
				imgui.Vec2{X: fim.X - 10, Y: fim.Y - 2},
				cor32(pal.Destaque), 2, 0)
		}

		desenharMarca(desenho, s, canto)

		if sobre {
			imgui.SetTooltip(s.Rotulo + "\nbotao direito: abrir no navegador")
		}
		if imgui.IsItemClicked() {
			abrirPlayer(s.Qual)
		}
		if sobre && g.IsMouseClicked(g.MouseButtonRight) {
			player.Abrir(s.Qual) // abre fora, no navegador padrão
		}
	})
}

// desenharMarca desenha a logo do serviço no meio do botão.
//
// Usa a IMAGEM de verdade (assets/servicos/<nome>.png) quando ela já está
// pronta na placa de vídeo. Se o arquivo faltar ou ainda estiver carregando,
// cai no desenho em vetor — assim o botão nunca fica vazio.
func desenharMarca(desenho *imgui.DrawList, s servico, canto imgui.Vec2) {
	if logo, pronta := texturasServicos[s.Qual]; pronta && logo != nil {
		desenharLogo(desenho, logo, canto)
		return
	}

	// Centro do botão, um pouco acima do meio para ficar visualmente centrado.
	cx := canto.X + largBotaoServico/2
	cy := canto.Y + altBotaoServico/2

	branco := cor32(rgb(255, 255, 255))

	switch s.Forma {
	case "playRetangulo":
		// Retângulo arredondado + triângulo de "play" (o crachá clássico).
		desenho.AddRectFilledV(
			imgui.Vec2{X: cx - 15, Y: cy - 10},
			imgui.Vec2{X: cx + 15, Y: cy + 10},
			cor32(s.Marca), 7, 0)
		triangulo(desenho, cx, cy, 6, branco)

	case "playCirculo":
		desenho.AddCircleFilled(imgui.Vec2{X: cx, Y: cy}, 12, cor32(s.Marca))
		triangulo(desenho, cx, cy, 6, branco)

	case "letra":
		// A letra é desenhada 2 vezes, com 1 pixel de diferença, para ficar
		// com aparência de negrito (o ImGui só tem uma espessura de fonte).
		tamanho := imgui.CalcTextSize(s.Letra)
		pos := imgui.Vec2{X: cx - tamanho.X/2, Y: cy - tamanho.Y/2}
		desenho.AddTextVec2(pos, cor32(s.Marca), s.Letra)
		desenho.AddTextVec2(imgui.Vec2{X: pos.X + 1, Y: pos.Y}, cor32(s.Marca), s.Letra)
	}
}

// desenharLogo desenha a imagem centralizada no botão, SEM distorcer: calcula
// o maior tamanho que caiba na área útil mantendo a proporção original.
func desenharLogo(desenho *imgui.DrawList, logo *logoServico, canto imgui.Vec2) {
	if logo.Larg <= 0 || logo.Alt <= 0 {
		return
	}

	// Área útil: sobra uma borda para a logo não encostar no canto do botão.
	const margemX, margemY = 6, 5
	espacoX := float32(largBotaoServico - margemX*2)
	espacoY := float32(altBotaoServico - margemY*2)

	// Escala = a menor das duas, para a imagem caber inteira nos dois
	// sentidos sem esticar (mantém a proporção original).
	escala := espacoX / logo.Larg
	if e := espacoY / logo.Alt; e < escala {
		escala = e
	}
	larg := logo.Larg * escala
	alt := logo.Alt * escala

	centroX := canto.X + largBotaoServico/2
	centroY := canto.Y + altBotaoServico/2
	inicio := imgui.Vec2{X: centroX - larg/2, Y: centroY - alt/2}
	fim := imgui.Vec2{X: centroX + larg/2, Y: centroY + alt/2}

	// A "tinta" branca com o alfa da interface: é assim que a imagem também
	// obedece ao slider de opacidade (o alfa do ImGui não vale para imagens
	// desenhadas direto). Cantos arredondados deixam as logos com fundo
	// próprio (Netflix, Disney+) com cara de ícone de aplicativo.
	tinta := imgui.ColorConvertFloat4ToU32(imgui.Vec4{X: 1, Y: 1, Z: 1, W: Alfa})
	desenho.AddImageRounded(logo.Textura.ID(), inicio, fim,
		imgui.Vec2{X: 0, Y: 0}, imgui.Vec2{X: 1, Y: 1}, tinta, 6)
}

// triangulo desenha o "play" apontando para a direita.
func triangulo(desenho *imgui.DrawList, cx, cy, tamanho float32, cor uint32) {
	desenho.AddTriangleFilled(
		imgui.Vec2{X: cx - tamanho*0.6, Y: cy - tamanho},
		imgui.Vec2{X: cx - tamanho*0.6, Y: cy + tamanho},
		imgui.Vec2{X: cx + tamanho, Y: cy},
		cor)
}

// cor32 converte a cor do Go para o número que o ImGui usa ao desenhar.
//
// Aplica a opacidade da interface (Alfa) na conta: o "StyleVarAlpha" do ImGui
// vale só para os widgets prontos dele, e NÃO para o que a gente desenha à mão.
// Sem isso, os ícones e o fundo dos botões ficariam sempre opacos enquanto o
// resto do painel ficava translúcido.
func cor32(c color.RGBA) uint32 {
	v := cor4(c)
	v.W *= Alfa
	return imgui.ColorConvertFloat4ToU32(v)
}

// rotuloDoServico devolve o nome bonito de um serviço (para o título do player).
func rotuloDoServico(qual string) string {
	for _, s := range servicos {
		if s.Qual == qual {
			return s.Rotulo
		}
	}
	return "Player"
}

// ─────────────────────── peças reutilizáveis ──────────────────────────────

func barraDeUso(nome string, valor float64) g.Widget {
	texto := fmt.Sprintf("%s  %.0f%%", nome, valor)
	fracao := float32(valor / 100)
	if valor < 0 {
		texto = nome + "  --"
		fracao = 0
	}
	return g.ProgressBar(fracao).Size(-1, 18).Overlay(texto)
}

// ─── botões de mídia desenhados à mão ──────────────────────────────────────
//
// Os símbolos seguem o padrão universal dos players (o triângulo de "play"
// vem dos gravadores de rolo dos anos 60 e nunca mudou): faixa ANTERIOR é uma
// barrinha + triângulo apontando para trás; PRÓXIMA é o espelho disso; play é
// o triângulo e pause são as duas barras. Desenhamos em vetor porque a fonte
// do ImGui não tem esses símbolos com boa aparência.

// rodapeDeMidia desenha o rodapé do painel Música: ⏮ ▶⏸ ⏭ na faixa de baixo,
// centralizados entre o menu lateral e o slider de volume em pé. Cada botão é
// posicionado por coordenada, para o rodapé ficar sempre no mesmo lugar.
func rodapeDeMidia() {
	esq := float32(4 + largBotaoServico + 8) // onde a área principal começa
	dir := imgui.WindowWidth() - 26          // onde o slider em pé começa

	const larguraDosTres = 44 + 8 + 64 + 8 + 44
	x := esq + (dir-esq-larguraDosTres)/2
	if x < esq {
		x = esq
	}
	y := imgui.WindowHeight() - 46

	imgui.SetCursorPos(imgui.Vec2{X: x, Y: y})
	desenharBotaoFaixa("faixa_anterior", false, audio.FaixaAnterior)

	imgui.SetCursorPos(imgui.Vec2{X: x + 52, Y: y})
	desenharBotaoPlayPause()

	imgui.SetCursorPos(imgui.Vec2{X: x + 124, Y: y})
	desenharBotaoFaixa("proxima_faixa", true, audio.ProximaFaixa)
}

// desenharBotaoFaixa é o botão de faixa anterior (proxima=false) ou próxima
// (true): discreto, sem fundo, que acende quando o mouse passa por cima.
func desenharBotaoFaixa(id string, proxima bool, acao func()) {
	const larg, alt = 44, 36
	canto := imgui.CursorScreenPos()
	imgui.InvisibleButton(id, imgui.Vec2{X: larg, Y: alt})
	sobre := imgui.IsItemHovered()

	desenho := imgui.WindowDrawList()
	if sobre {
		desenho.AddRectFilledV(canto,
			imgui.Vec2{X: canto.X + larg, Y: canto.Y + alt},
			cor32(pal.Botao), 8, 0)
	}

	cx := canto.X + larg/2
	cy := canto.Y + alt/2
	cor := cor32(pal.TextoFraco)
	if sobre {
		cor = cor32(pal.Texto)
	}

	if proxima {
		// triângulo para a frente + barrinha no fim (⏭)
		desenho.AddTriangleFilled(
			imgui.Vec2{X: cx - 8, Y: cy - 6},
			imgui.Vec2{X: cx - 8, Y: cy + 6},
			imgui.Vec2{X: cx + 3, Y: cy}, cor)
		desenho.AddRectFilledV(
			imgui.Vec2{X: cx + 5, Y: cy - 6},
			imgui.Vec2{X: cx + 8, Y: cy + 6}, cor, 1, 0)
	} else {
		// barrinha no começo + triângulo para trás (⏮)
		desenho.AddRectFilledV(
			imgui.Vec2{X: cx - 8, Y: cy - 6},
			imgui.Vec2{X: cx - 5, Y: cy + 6}, cor, 1, 0)
		desenho.AddTriangleFilled(
			imgui.Vec2{X: cx + 8, Y: cy - 6},
			imgui.Vec2{X: cx + 8, Y: cy + 6},
			imgui.Vec2{X: cx - 3, Y: cy}, cor)
	}

	if sobre {
		if proxima {
			imgui.SetTooltip("Proxima faixa")
		} else {
			imgui.SetTooltip("Faixa anterior")
		}
	}
	if imgui.IsItemClicked() {
		acao()
	}
}

// desenharBotaoPlayPause é a "pílula" clara central, como o play do Spotify —
// com o triângulo (play) e as duas barras (pause) desenhados em vetor.
func desenharBotaoPlayPause() {
	const larg, alt = 64, 36
	canto := imgui.CursorScreenPos()
	imgui.InvisibleButton("play_pause", imgui.Vec2{X: larg, Y: alt})
	sobre := imgui.IsItemHovered()

	fundo := pal.Texto
	if sobre {
		fundo = misturar(pal.Texto, rgb(255, 255, 255), 0.5)
	}
	if imgui.IsItemActive() {
		fundo = pal.Destaque
	}
	desenho := imgui.WindowDrawList()
	desenho.AddRectFilledV(canto,
		imgui.Vec2{X: canto.X + larg, Y: canto.Y + alt},
		cor32(fundo), alt/2, 0)

	cx := canto.X + larg/2
	cy := canto.Y + alt/2
	cor := cor32(pal.Fundo)

	// play à esquerda, pause à direita
	desenho.AddTriangleFilled(
		imgui.Vec2{X: cx - 14, Y: cy - 7},
		imgui.Vec2{X: cx - 14, Y: cy + 7},
		imgui.Vec2{X: cx - 2, Y: cy}, cor)
	desenho.AddRectFilledV(
		imgui.Vec2{X: cx + 4, Y: cy - 7},
		imgui.Vec2{X: cx + 7, Y: cy + 7}, cor, 1, 0)
	desenho.AddRectFilledV(
		imgui.Vec2{X: cx + 10, Y: cy - 7},
		imgui.Vec2{X: cx + 13, Y: cy + 7}, cor, 1, 0)

	if imgui.IsItemClicked() {
		audio.PlayPause()
	}
}

func textoFraco(texto string) g.Widget {
	return g.Style().
		SetColor(g.StyleColorText, pal.TextoFraco).
		To(g.Label(texto))
}

func curto(nome string, max int) string {
	if len(nome) <= max {
		return nome
	}
	return nome[:max-3] + "..."
}

// ───────────────────── opacidade do vídeo ─────────────────────────────────

var (
	ultimaOpacidadeEnviada float32 = -1
	quadrosDesdeOpacidade  int
)

// acertarOpacidadeDoVideo mantém o vídeo com a mesma opacidade da interface.
//
// Manda o comando ao navegador em dois casos:
//   - quando o usuário move o slider (o valor mudou);
//   - de dois em dois segundos, porque ao trocar de página (abrir um vídeo,
//     por exemplo) o site monta o HTML de novo e apaga o nosso CSS.
func acertarOpacidadeDoVideo() {
	if carregado, _, _ := player.Estado(); !carregado {
		return
	}

	quadrosDesdeOpacidade++
	mudou := Alfa != ultimaOpacidadeEnviada
	naHora := quadrosDesdeOpacidade >= 120 // ~2 segundos a 60 quadros/s

	if mudou || naHora {
		player.DefinirOpacidade(Alfa)
		ultimaOpacidadeEnviada = Alfa
		quadrosDesdeOpacidade = 0
	}
}

// ───────────────────── player embutido ────────────────────────────────────

// pedidoDePlayer guarda qual site abrir. Quem clica no botão só ANOTA aqui;
// quem abre de verdade é o antesDoQuadro (veja abrirPlayerAgora).
var pedidoDePlayer string

// abrirPlayer só anota o pedido — NÃO abre na hora.
//
// Por que: encaixar o WebView2 (função Embed) processa mensagens do Windows por
// dentro, e isso pode disparar um NOVO quadro do ImGui no meio do quadro atual.
// O ImGui não permite isso e o programa fecha com "Forgot to call Render()".
// Anotando o pedido, a abertura acontece no antesDoQuadro — que roda ENTRE
// quadros, onde processar mensagens é seguro.
func abrirPlayer(qual string) { pedidoDePlayer = qual }

// abrirPlayerAgora executa o pedido anotado. Chamado pelo antesDoQuadro.
func abrirPlayerAgora() {
	qual := pedidoDePlayer
	if qual == "" {
		return
	}
	pedidoDePlayer = ""

	// Não passamos posição: quem posiciona é a janelinha do player, durante o
	// desenho (veja player.MostrarEmbutido). Assim o vídeo já aparece no lugar
	// certo, sem piscar antes num lugar errado.
	if !player.MostrarEmbutido(acharIDJanela(), qual) {
		player.Abrir(qual) // plano B: PC sem WebView2 -> janela separada
		return
	}

	// Traz a janela para a frente e entrega o teclado ao navegador, para já
	// dar para digitar (buscar um vídeo, fazer login...). Precisa ser DEPOIS
	// de abrir: o estilo NOACTIVATE só sai quando o player está visível.
	const mostrarSemAtivar = 4 // SW_SHOWNOACTIVATE
	procShowWindow.Call(acharIDJanela(), mostrarSemAtivar)
	procSetForegroundWindow.Call(acharIDJanela())
	player.Focar()
}

// alternarVideo esconde/mostra o vídeo. Esconder NÃO para o som —
// é o modo "só quero escutar".
func alternarVideo() {
	_, visivel, qual := player.Estado()
	if visivel {
		player.OcultarVideo()
		return
	}
	abrirPlayer(qual)
}

func fecharPlayer() {
	player.FecharEmbutido()
}

func sair() {
	janela.SetShouldClose(true)
}
