// Pacote bandeja: o ícone do Nimbus na bandeja do sistema — aqueles ícones
// pequenos ao lado do relógio (às vezes escondidos atrás da setinha "^").
//
// Como funciona: o Windows só entrega eventos de um ícone da bandeja para uma
// JANELA. Então criamos aqui uma janela **invisível e sem tela** (do tipo
// "só mensagens"), numa thread separada com o próprio laço de mensagens.
// Assim ela não atrapalha em nada a janela do overlay, que é desenhada pelo
// ImGui/GLFW na thread principal.
//
// Comunicação com a interface: esta thread NUNCA chama o ImGui (não é seguro
// mexer na interface de outra thread). Ela só levanta "bandeirinhas" que a
// interface confere a cada quadro — veja Pedidos().
package bandeja

import (
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClass     = user32.NewProc("RegisterClassExW")
	procCreateWindow      = user32.NewProc("CreateWindowExW")
	procDefWindowProc     = user32.NewProc("DefWindowProcW")
	procGetMessage        = user32.NewProc("GetMessageW")
	procTranslateMessage  = user32.NewProc("TranslateMessage")
	procDispatchMessage   = user32.NewProc("DispatchMessageW")
	procPostQuitMessage   = user32.NewProc("PostQuitMessage")
	procDestroyWindow     = user32.NewProc("DestroyWindow")
	procLoadImage         = user32.NewProc("LoadImageW")
	procLoadIcon          = user32.NewProc("LoadIconW")
	procCreatePopupMenu   = user32.NewProc("CreatePopupMenu")
	procAppendMenu        = user32.NewProc("AppendMenuW")
	procDestroyMenu       = user32.NewProc("DestroyMenu")
	procTrackPopupMenu    = user32.NewProc("TrackPopupMenu")
	procSetForegroundWnd  = user32.NewProc("SetForegroundWindow")
	procGetCursorPos      = user32.NewProc("GetCursorPos")
	procShellNotifyIcon   = shell32.NewProc("Shell_NotifyIconW")
	procGetModuleHandle   = kernel32.NewProc("GetModuleHandleW")
)

const (
	// Mensagem que escolhemos para o Windows nos avisar dos cliques no ícone.
	msgIcone = 0x8000 + 1 // WM_APP + 1

	// Mensagens de mouse que nos interessam.
	msgBotaoEsqSolto  = 0x0202 // WM_LBUTTONUP
	msgBotaoDirSolto  = 0x0205 // WM_RBUTTONUP
	msgCliqueDuploEsq = 0x0203 // WM_LBUTTONDBLCLK
	msgDestruir       = 0x0002 // WM_DESTROY

	// Shell_NotifyIcon: o que fazer com o ícone.
	adicionarIcone = 0x0
	removerIcone   = 0x2

	// Quais campos da estrutura estão preenchidos.
	temMensagem = 0x01 // NIF_MESSAGE
	temIcone    = 0x02 // NIF_ICON
	temDica     = 0x04 // NIF_TIP

	janelaSoMensagens = ^uintptr(2) // HWND_MESSAGE (o valor -3 do Windows)

	// Menu do botão direito.
	menuTexto      = 0x0000 // MF_STRING
	menuSeparador  = 0x0800 // MF_SEPARATOR
	menuRetornaID  = 0x0100 // TPM_RETURNCMD
	menuBotaoDir   = 0x0002 // TPM_RIGHTBUTTON

	idMostrarEsconder = 1
	idSair            = 2
)

// classeJanela é o "molde" da janela invisível.
type classeJanela struct {
	tamanho    uint32
	estilo     uint32
	processo   uintptr
	extraClass int32
	extraJanela int32
	instancia  uintptr
	icone      uintptr
	cursor     uintptr
	fundo      uintptr
	menu       *uint16
	nome       *uint16
	iconePeq   uintptr
}

// dadosIcone é a estrutura que descreve o ícone da bandeja (NOTIFYICONDATAW).
type dadosIcone struct {
	tamanho          uint32
	janela           uintptr
	id               uint32
	campos           uint32
	mensagemRetorno  uint32
	icone            uintptr
	dica             [128]uint16
	estado           uint32
	mascaraEstado    uint32
	info             [256]uint16
	versao           uint32
	tituloInfo       [64]uint16
	sinalizadoresInfo uint32
	guid             [16]byte
	iconeBalao       uintptr
}

type mensagem struct {
	janela   uintptr
	tipo     uint32
	wParam   uintptr
	lParam   uintptr
	tempo    uint32
	x, y     int32
	privado  uint32
}

// Bandeirinhas que a interface confere a cada quadro (ver Pedidos()).
var (
	pedidoAlternar int32
	pedidoSair     int32

	// Estado atual do overlay, para o menu dizer "Mostrar" ou "Esconder".
	overlayVisivel int32 = 1

	janelaBandeja uintptr
	dados         dadosIcone
)

// Pedidos devolve (e limpa) os pedidos feitos pelo ícone da bandeja desde a
// última vez. A interface chama isto a cada quadro.
func Pedidos() (alternar bool, sair bool) {
	return atomic.SwapInt32(&pedidoAlternar, 0) > 0,
		atomic.SwapInt32(&pedidoSair, 0) > 0
}

// DefinirVisivel avisa a bandeja se o overlay está aparecendo, para o texto do
// menu ficar certo. A interface chama isto a cada quadro.
func DefinirVisivel(visivel bool) {
	v := int32(0)
	if visivel {
		v = 1
	}
	atomic.StoreInt32(&overlayVisivel, v)
}

// Iniciar cria o ícone na bandeja. Roda em segundo plano; se algo falhar, o
// programa segue normalmente (só fica sem o ícone).
func Iniciar(dica string) {
	go func() {
		// O laço de mensagens do Windows é por THREAD, então prendemos esta
		// goroutine a uma thread só dela.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		if !criarJanela() {
			return
		}
		if !criarIcone(dica) {
			return
		}
		defer removerIconeBandeja()

		// Fica aqui esperando os cliques no ícone até o programa encerrar.
		var msg mensagem
		for {
			r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if r == 0 || int32(r) == -1 {
				return
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
}

// Encerrar remove o ícone da bandeja (chamado ao sair do programa, para não
// deixar um ícone "fantasma" ao lado do relógio).
func Encerrar() {
	removerIconeBandeja()
	if janelaBandeja != 0 {
		procDestroyWindow.Call(janelaBandeja)
	}
}

func removerIconeBandeja() {
	if dados.janela != 0 {
		procShellNotifyIcon.Call(removerIcone, uintptr(unsafe.Pointer(&dados)))
	}
}

// criarJanela registra a classe e cria a janela invisível "só mensagens".
func criarJanela() bool {
	instancia, _, _ := procGetModuleHandle.Call(0)
	nome, err := syscall.UTF16PtrFromString("NimbusBandeja")
	if err != nil {
		return false
	}

	classe := classeJanela{
		tamanho:   uint32(unsafe.Sizeof(classeJanela{})),
		processo:  syscall.NewCallback(tratarMensagem),
		instancia: instancia,
		nome:      nome,
	}
	if r, _, _ := procRegisterClass.Call(uintptr(unsafe.Pointer(&classe))); r == 0 {
		return false
	}

	vazio, _ := syscall.UTF16PtrFromString("")
	janelaBandeja, _, _ = procCreateWindow.Call(
		0, uintptr(unsafe.Pointer(nome)), uintptr(unsafe.Pointer(vazio)),
		0, 0, 0, 0, 0,
		janelaSoMensagens, 0, instancia, 0,
	)
	return janelaBandeja != 0
}

// criarIcone carrega o ícone do disco e o coloca na bandeja.
func criarIcone(dica string) bool {
	dados = dadosIcone{
		tamanho:         uint32(unsafe.Sizeof(dadosIcone{})),
		janela:          janelaBandeja,
		id:              1,
		campos:          temMensagem | temIcone | temDica,
		mensagemRetorno: msgIcone,
		icone:           carregarIcone(),
	}
	if texto, err := syscall.UTF16FromString(dica); err == nil {
		copy(dados.dica[:], texto)
	}

	r, _, _ := procShellNotifyIcon.Call(adicionarIcone, uintptr(unsafe.Pointer(&dados)))
	return r != 0
}

// carregarIcone lê o assets/nimbus.ico do disco (padrão do projeto: assets
// ficam em arquivo, não embutidos). Se não achar, usa o ícone padrão do
// Windows — assim nunca fica sem ícone nenhum.
func carregarIcone() uintptr {
	const (
		tipoIcone     = 1    // IMAGE_ICON
		carregarDeArquivo = 0x10 // LR_LOADFROMFILE
		tamanhoPadrao = 0x40 // LR_DEFAULTSIZE
	)

	for _, caminho := range caminhosDoIcone() {
		if _, err := os.Stat(caminho); err != nil {
			continue
		}
		p, err := syscall.UTF16PtrFromString(caminho)
		if err != nil {
			continue
		}
		icone, _, _ := procLoadImage.Call(0, uintptr(unsafe.Pointer(p)),
			tipoIcone, 0, 0, carregarDeArquivo|tamanhoPadrao)
		if icone != 0 {
			return icone
		}
	}

	const iconeAplicacaoPadrao = 32512 // IDI_APPLICATION
	icone, _, _ := procLoadIcon.Call(0, iconeAplicacaoPadrao)
	return icone
}

// caminhosDoIcone lista onde procurar o ícone: ao lado do .exe (que fica em
// build/, então subimos um nível) e na pasta atual.
func caminhosDoIcone() []string {
	lista := []string{
		filepath.Join("assets", "nimbus.ico"),
	}
	if exe, err := os.Executable(); err == nil {
		pasta := filepath.Dir(exe)
		lista = append(lista,
			filepath.Join(pasta, "assets", "nimbus.ico"),
			filepath.Join(pasta, "..", "assets", "nimbus.ico"),
			filepath.Join(pasta, "nimbus.ico"),
		)
	}
	return lista
}

// tratarMensagem recebe os eventos da janela invisível.
func tratarMensagem(janela uintptr, tipo uint32, wParam, lParam uintptr) uintptr {
	switch tipo {
	case msgIcone:
		// O tipo do clique vem na parte baixa do lParam.
		switch uint32(lParam) & 0xFFFF {
		case msgBotaoEsqSolto, msgCliqueDuploEsq:
			atomic.AddInt32(&pedidoAlternar, 1)
		case msgBotaoDirSolto:
			abrirMenu(janela)
		}
		return 0

	case msgDestruir:
		procPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := procDefWindowProc.Call(janela, uintptr(tipo), wParam, lParam)
	return r
}

// abrirMenu mostra o menuzinho do botão direito (Mostrar/Esconder e Sair).
func abrirMenu(janela uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	rotulo := "Esconder painéis"
	if atomic.LoadInt32(&overlayVisivel) == 0 {
		rotulo = "Mostrar painéis"
	}
	adicionarItem(menu, menuTexto, idMostrarEsconder, rotulo)
	adicionarItem(menu, menuSeparador, 0, "")
	adicionarItem(menu, menuTexto, idSair, "Sair do Nimbus")

	// O Windows exige que a janela do menu esteja "na frente", senão o menu
	// fica preso na tela quando o usuário clica fora dele.
	procSetForegroundWnd.Call(janela)

	var p struct{ x, y int32 }
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))

	// Com "retorna ID", a escolha do usuário volta aqui direto.
	escolha, _, _ := procTrackPopupMenu.Call(menu,
		menuRetornaID|menuBotaoDir,
		uintptr(p.x), uintptr(p.y), 0, janela, 0)

	switch escolha {
	case idMostrarEsconder:
		atomic.AddInt32(&pedidoAlternar, 1)
	case idSair:
		atomic.StoreInt32(&pedidoSair, 1)
	}
}

func adicionarItem(menu uintptr, tipo uint32, id int, texto string) {
	t, err := syscall.UTF16PtrFromString(texto)
	if err != nil {
		return
	}
	procAppendMenu.Call(menu, uintptr(tipo), uintptr(id), uintptr(unsafe.Pointer(t)))
}
