// Pacote player: o mini-player de YouTube / YouTube Music.
//
// Como funciona: o ImGui (a janelinha do overlay) não sabe mostrar sites ou
// vídeos — ele só desenha controles. Então o player é uma SEGUNDA janelinha,
// que usa o WebView2 (o motor do navegador Edge, que já vem instalado no
// Windows) para carregar o site do YouTube. Ela também fica sempre por cima,
// como um picture-in-picture.
//
// Cada player roda como um PROCESSO separado (o mesmo .exe, chamado com
// "--player ..."). Por quê: o navegador precisa do seu próprio "loop de
// janela" do Windows, e misturar isso com o loop do ImGui trava os dois.
// Separado, cada um vive sua vida — dá até para fechar o overlay e deixar a
// música tocando.
package player

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

// Sites disponíveis. Para adicionar outro (ex.: Twitch), inclua aqui e
// acrescente o botão na lista "servicos" (internal/ui/overlay.go).
//
// ⚠️ Netflix e Disney+ usam proteção de conteúdo (DRM). O navegador embutido
// (WebView2) não traz o componente que decifra esse conteúdo, então o vídeo
// pode se recusar a tocar mesmo com o site abrindo normalmente. Nesse caso use
// o botão DIREITO no ícone: abre no navegador de verdade, que tem o DRM.
var enderecos = map[string]string{
	"youtube": "https://www.youtube.com",
	"music":   "https://music.youtube.com",
	"netflix": "https://www.netflix.com",
	"disney":  "https://www.disneyplus.com",
}

// Abrir dispara um novo processo do próprio programa em modo player.
// (É o que os botões "YouTube" e "YT Music" do overlay chamam.)
func Abrir(qual string) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	// Start (e não Run): dispara e NÃO espera — o overlay continua livre.
	exec.Command(exe, "--player", qual).Start()
}

// Rodar abre a janela do player e fica nela até o usuário fechar.
// É chamada pelo main.go quando o programa inicia com "--player".
func Rodar(qual string) {
	endereco, existe := enderecos[qual]
	if !existe {
		endereco = enderecos["youtube"]
	}

	titulo := "YouTube - mini player"
	if qual == "music" {
		titulo = "YouTube Music - mini player"
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  titulo,
			Width:  560,
			Height: 360,
		},
	})
	if w == nil {
		// Fallback: sem o WebView2 no PC (muito raro no Windows 11), não tem
		// como mostrar o site — abre no navegador padrão para não deixar o
		// usuário sem nada.
		exec.Command("rundll32", "url.dll,FileProtocolHandler", endereco).Start()
		return
	}
	defer w.Destroy()

	deixarSemprePorCima(w.Window())
	w.Navigate(endereco)
	w.Run() // fica aqui até fecharem a janela
}

// deixarSemprePorCima marca a janela como "topmost" no Windows —
// mesmo truque do overlay: ela flutua por cima de tudo.
func deixarSemprePorCima(idJanela unsafe.Pointer) {
	user32 := syscall.NewLazyDLL("user32.dll")
	setWindowPos := user32.NewProc("SetWindowPos")

	const (
		semprePorCima = ^uintptr(0) // HWND_TOPMOST (o valor -1 do Windows)
		naoMover      = 0x0002      // SWP_NOMOVE  (mantém a posição atual)
		naoRedimensionar = 0x0001   // SWP_NOSIZE (mantém o tamanho atual)
	)
	setWindowPos.Call(uintptr(idJanela), semprePorCima, 0, 0, 0, 0, naoMover|naoRedimensionar)
}
