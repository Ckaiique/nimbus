// Cria a janela que hospeda o navegador embutido.
//
// POR QUE O NAVEGADOR TEM A PRÓPRIA JANELA (e não fica dentro da nossa):
//
// Antes, o WebView2 era encaixado DENTRO da janela do overlay. O problema: o
// motor do Edge usa composição própria (DirectComposition) e, ao entrar na
// janela, muda o jeito como o Windows compõe ela toda. Quando o navegador saía
// (fechar a aba OU recolher o painel), o Windows recompunha e a transparência
// por pixel se perdia — a TELA INTEIRA ficava preta, e reabrir o navegador
// consertava. Tentamos reafirmar a transparência todo quadro e forçar a
// recomposição; não resolveu, porque a causa é a convivência dos dois na mesma
// janela.
//
// Com o navegador na SUA janela, a nossa nunca hospeda a composição do Edge:
// a transparência dos painéis nunca é afetada, aconteça o que acontecer com o
// vídeo.
//
// A janela do vídeo é:
//   - sem moldura (WS_POPUP) — quem desenha a borda é o painel do ImGui;
//   - "dona" da janela do overlay, então acompanha ela na ordem das janelas;
//   - fora da barra de tarefas e do Alt+Tab (WS_EX_TOOLWINDOW);
//   - sempre por cima, como o overlay.
//
// Ela PODE receber foco (ao contrário do overlay) — é isso que faz o teclado
// funcionar para digitar login e busca dentro do site.
package player

import (
	"syscall"
	"unsafe"
)

var (
	user32            = syscall.NewLazyDLL("user32.dll")
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procRegisterClass = user32.NewProc("RegisterClassExW")
	procCreateWindow  = user32.NewProc("CreateWindowExW")
	procDefWindowProc = user32.NewProc("DefWindowProcW")
	procShowWindow    = user32.NewProc("ShowWindow")
	procSetWindowPos  = user32.NewProc("SetWindowPos")
	procDestroyWindow = user32.NewProc("DestroyWindow")
	procSetForeground = user32.NewProc("SetForegroundWindow")
	procSetLayered    = user32.NewProc("SetLayeredWindowAttributes")
	procGetModule     = kernel32.NewProc("GetModuleHandleW")
)

const (
	nomeClasseVideo = "NimbusJanelaVideo"

	estiloPopup        = 0x80000000 // WS_POPUP (sem moldura)
	exJanelaFerramenta = 0x00000080 // WS_EX_TOOLWINDOW
	exSemprePorCima    = 0x00000008 // WS_EX_TOPMOST
	exEmCamadas        = 0x00080000 // WS_EX_LAYERED (permite opacidade)

	mostrarSemAtivar = 4 // SW_SHOWNOACTIVATE
	esconder         = 0 // SW_HIDE

	swpSemAtivar = 0x0010 // SWP_NOACTIVATE

	usarAlfa = 0x2 // LWA_ALPHA

	msgDestruir = 0x0002 // WM_DESTROY
)

// paraOTopo é o HWND_TOPMOST (-1): usado para manter a janela do vídeo ACIMA do
// overlay. Necessário porque o overlay reafirma o topo a cada quadro e, sem
// isto, passaria na frente do vídeo e engoliria os cliques dele.
var paraOTopo = func() uintptr { n := -1; return uintptr(n) }()

// classeJanela é o "molde" da janela (WNDCLASSEXW).
type classeJanela struct {
	tamanho     uint32
	estilo      uint32
	processo    uintptr
	extraClasse int32
	extraJanela int32
	instancia   uintptr
	icone       uintptr
	cursor      uintptr
	fundo       uintptr
	menu        *uint16
	nome        *uint16
	iconePeq    uintptr
}

var classeRegistrada bool

// registrarClasseVideo registra o molde da janela, uma vez só.
func registrarClasseVideo() bool {
	if classeRegistrada {
		return true
	}

	instancia, _, _ := procGetModule.Call(0)
	nome, err := syscall.UTF16PtrFromString(nomeClasseVideo)
	if err != nil {
		return false
	}

	// Cursor padrão (setinha) e SEM pincel de fundo: quem pinta é o navegador.
	procLoadCursor := user32.NewProc("LoadCursorW")
	const cursorSeta = 32512 // IDC_ARROW
	cursor, _, _ := procLoadCursor.Call(0, cursorSeta)

	classe := classeJanela{
		tamanho:   uint32(unsafe.Sizeof(classeJanela{})),
		processo:  syscall.NewCallback(tratarMensagemVideo),
		instancia: instancia,
		cursor:    cursor,
		nome:      nome,
	}
	if r, _, _ := procRegisterClass.Call(uintptr(unsafe.Pointer(&classe))); r == 0 {
		return false
	}
	classeRegistrada = true
	return true
}

// tratarMensagemVideo é o tratador de mensagens da janela do vídeo. Não precisa
// fazer nada de especial: o navegador cuida do conteúdo.
func tratarMensagemVideo(janela uintptr, tipo uint32, wParam, lParam uintptr) uintptr {
	if tipo == msgDestruir {
		return 0
	}
	r, _, _ := procDefWindowProc.Call(janela, uintptr(tipo), wParam, lParam)
	return r
}

// criarJanelaVideo cria a janela que vai hospedar o navegador. Nasce ESCONDIDA:
// só aparece quando a interface souber onde ela deve ficar.
func criarJanelaVideo(dona uintptr) (uintptr, bool) {
	if !registrarClasseVideo() {
		return 0, false
	}

	nome, _ := syscall.UTF16PtrFromString(nomeClasseVideo)
	titulo, _ := syscall.UTF16PtrFromString("Nimbus Video")

	janela, _, _ := procCreateWindow.Call(
		exJanelaFerramenta|exSemprePorCima|exEmCamadas,
		uintptr(unsafe.Pointer(nome)),
		uintptr(unsafe.Pointer(titulo)),
		estiloPopup,
		0, 0, 640, 400, // posição/tamanho provisórios; a interface acerta depois
		dona, 0, 0, 0,
	)
	return janela, janela != 0
}

// posicionarJanelaVideo move e redimensiona a janela do vídeo, em coordenadas
// de TELA, sem ativá-la (não rouba o foco de quem está na frente).
//
// Também a mantém NO TOPO a cada chamada. Isso é necessário: o overlay reafirma
// o topo dele todo quadro e, sem isto, passaria na frente do vídeo — e aí os
// cliques na página iam para o overlay em vez de para o site.
func posicionarJanelaVideo(janela uintptr, x, y, larg, alt int32) {
	procSetWindowPos.Call(janela, paraOTopo,
		uintptr(x), uintptr(y), uintptr(larg), uintptr(alt),
		swpSemAtivar)
}

// definirOpacidadeJanela deixa a janela do vídeo translúcida.
//
// Este é o jeito CERTO agora que o vídeo tem janela própria: uma opacidade
// uniforme aplicada pelo Windows na janela inteira.
//
// Antes eu injetava CSS na página para isso (apagando o fundo dela e baixando a
// opacidade). Funcionava quando o navegador morava dentro da janela do overlay,
// que tinha composição transparente. Numa janela normal, apagar o fundo da
// página não tem com o que compor — e o site aparecia MUITO ESCURO.
func definirOpacidadeJanela(janela uintptr, alfa float32) {
	if alfa < 0.15 {
		alfa = 0.15 // nunca deixa o vídeo desaparecer de vez
	}
	if alfa > 1 {
		alfa = 1
	}
	valor := uintptr(alfa*255 + 0.5)
	procSetLayered.Call(janela, 0, valor, usarAlfa)
}
