// Pacote instancia: garante que só exista UM Nimbus rodando por vez.
//
// Por que isso existe: o Nimbus não aparece na barra de tarefas (fica só na
// bandeja, ao lado do relógio). Então é muito fácil clicar duas vezes no
// atalho sem perceber que ele já estava aberto — e aí dois programas iguais
// disputam as mesmas coisas do Windows: dois ícones na bandeja, dois overlays
// sobrepostos brigando por "quem fica no topo", duas janelas de vídeo e a
// tecla Insert respondendo em dobro. Dá bug na hora.
//
// Como travamos: pedimos ao Windows um "mutex" (uma plaquinha com nome único,
// visível para todos os programas). Quem cria primeiro fica com ela; o segundo
// que pedir o MESMO nome recebe o aviso "isso já existe" — e é assim que ele
// descobre que já tem um Nimbus rodando, sem precisar procurar janela nem
// vasculhar a lista de processos.
//
// Vantagem desse jeito sobre "procurar a janela pelo título": o Windows
// apaga a plaquinha sozinho quando o programa fecha, INCLUSIVE se ele travar
// ou for morto pelo Gerenciador de Tarefas. Não fica trava fantasma.
package instancia

import (
	"syscall"
	"unsafe"
)

// nomeDaTrava é o nome da "plaquinha" no Windows. Precisa ser único no PC
// inteiro — por isso é bem específico. Não vale mudar sem motivo: se mudar,
// uma versão antiga e uma nova do Nimbus deixariam de se enxergar.
const nomeDaTrava = "Nimbus.Overlay.KST.InstanciaUnica"

// tituloAviso/textoAviso: o que a pessoa lê quando tenta abrir de novo.
const (
	tituloAviso = "Nimbus já está aberto"
	textoAviso  = "O Nimbus já está rodando neste computador.\n\n" +
		"Ele não aparece na barra de tarefas: fica na bandeja do sistema " +
		"(os ícones pequenos ao lado do relógio, às vezes escondidos atrás " +
		"da setinha \"^\").\n\n" +
		"Clique nesse ícone — ou aperte a tecla Insert — para mostrar ou " +
		"esconder os painéis."
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")

	procCreateMutex  = kernel32.NewProc("CreateMutexW")
	procReleaseMutex = kernel32.NewProc("ReleaseMutex")
	procCloseHandle  = kernel32.NewProc("CloseHandle")
	procMessageBox   = user32.NewProc("MessageBoxW")
)

// erroJaExiste é o ERROR_ALREADY_EXISTS do Windows: "essa plaquinha já era de
// alguém". É exatamente o sinal de que outro Nimbus está aberto.
const erroJaExiste = 183

// Botões/estilo da caixinha de aviso (constantes do MessageBox do Windows).
const (
	soBotaoOK    = 0x00000000 // MB_OK
	iconeInfo    = 0x00000040 // MB_ICONINFORMATION
	ficarNoTopo  = 0x00040000 // MB_TOPMOST (senão nasce atrás do overlay)
	trazerPraFre = 0x00010000 // MB_SETFOREGROUND
)

// trava guarda o "punho" (handle) da plaquinha enquanto o programa vive.
// Precisa ficar guardado numa variável do pacote: se o Go achasse que ninguém
// mais usa, nada mudaria (o Windows é quem manda), mas fechá-la cedo demais
// liberaria a trava com o programa ainda rodando.
var trava uintptr

// Unica tenta reservar o Nimbus para este processo.
//
// Devolve true  -> pode seguir, este é o único Nimbus;
// devolve false -> já tem outro aberto (e a pessoa já foi avisada na tela);
//                  quem chamou deve simplesmente encerrar.
//
// Fallback (regra do projeto: nada pode travar o programa por completo): se o
// Windows não conseguir criar a plaquinha por qualquer motivo estranho, a
// gente deixa abrir. É melhor um Nimbus a mais do que um Nimbus que se recusa
// a iniciar.
func Unica() bool {
	punho, jaExistia := reservar(nomeDaTrava)
	if punho == 0 {
		return true // não deu para criar a trava: deixa abrir (fallback)
	}

	if jaExistia {
		// Já existe outro Nimbus. Soltamos o punho (não é nosso) e avisamos.
		procCloseHandle.Call(punho)
		avisar()
		return false
	}

	trava = punho
	return true
}

// reservar é o miolo da trava, separado para poder ser testado sem abrir
// caixinha na tela.
//
// Devolve o punho (0 = não deu) e se a plaquinha JÁ EXISTIA — ou seja, se tem
// outro Nimbus rodando.
func reservar(nomeDaPlaquinha string) (punho uintptr, jaExistia bool) {
	nome, err := syscall.UTF16PtrFromString(nomeDaPlaquinha)
	if err != nil {
		return 0, false // nome inválido é impossível aqui, mas não trava a abertura
	}

	// CreateMutexW(sem segurança, não quero ser o dono exclusivo, nome).
	// Ela SEMPRE devolve um punho válido quando o mutex já existe — quem
	// conta a novidade é o "último erro" do Windows.
	h, _, ultimoErro := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(nome)))
	if h == 0 {
		return 0, false
	}
	return h, numeroDoErro(ultimoErro) == erroJaExiste
}

// Liberar devolve a plaquinha ao Windows na saída normal do programa.
// Não é obrigatório (o Windows limpa sozinho quando o processo morre), mas
// deixa o encerramento explícito — e evita qualquer atraso do sistema entre
// fechar e poder abrir de novo.
func Liberar() {
	if trava == 0 {
		return
	}
	procReleaseMutex.Call(trava)
	procCloseHandle.Call(trava)
	trava = 0
}

// avisar mostra a caixinha de "já está aberto".
//
// Tem de ser uma caixa do Windows (MessageBox) e não um texto no terminal:
// o Nimbus é compilado com "-H=windowsgui", ou seja, não tem janela preta de
// terminal — uma mensagem impressa não seria vista por ninguém.
func avisar() {
	titulo, err1 := syscall.UTF16PtrFromString(tituloAviso)
	texto, err2 := syscall.UTF16PtrFromString(textoAviso)
	if err1 != nil || err2 != nil {
		return // sem aviso na tela, mas o programa mesmo assim não abre duas vezes
	}
	procMessageBox.Call(
		0, // sem janela-mãe
		uintptr(unsafe.Pointer(texto)),
		uintptr(unsafe.Pointer(titulo)),
		soBotaoOK|iconeInfo|ficarNoTopo|trazerPraFre,
	)
}

// numeroDoErro extrai o código numérico do erro que as chamadas do Windows
// devolvem. Detalhe importante: essas funções do syscall devolvem um erro
// **sempre** (é o "último erro" do Windows), inclusive quando deu tudo certo
// — nesse caso o código é 0. Por isso comparamos o NÚMERO, nunca "err != nil".
func numeroDoErro(err error) uintptr {
	if errno, ok := err.(syscall.Errno); ok {
		return uintptr(errno)
	}
	return 0
}
