// Player EMBUTIDO: os sites (YouTube, Netflix...) mostrados junto com o overlay.
//
// O ImGui não sabe mostrar sites, então usamos o WebView2 (motor do Edge). Ele
// mora numa JANELA PRÓPRIA (veja janela_video.go), que a interface mantém
// exatamente sobre a janelinha do player — visualmente parece uma coisa só.
//
// Dois níveis de visibilidade, e não devem ser confundidos:
//
//	nav.Show()/Hide()        -> o navegador DENTRO da janela dele
//	ShowWindow(inst.janela)  -> a JANELA do vídeo na tela
//
// O navegador fica visível para sempre; quem esconde/mostra é a JANELA.
//
// ─── DOIS MODOS (escolha do usuário, na aba Config) ───────────────────────
//
// MODO ECONÔMICO (padrão): um navegador só, reaproveitado. Trocar de serviço é
// como digitar outro endereço na mesma aba — o site anterior é descarregado e o
// som dele para. Gasta pouca memória.
//
// MODO "MANTER CARREGADO": um navegador por serviço, criado **na primeira vez
// que você usa aquele serviço** (nunca antes: é sob demanda). Trocar de serviço
// só esconde um e mostra o outro, então o que estava tocando CONTINUA tocando
// em segundo plano. Custa memória: cada navegador abre seus próprios processos.
//
// ─── ESTADOS, que não devem ser confundidos ───────────────────────────────
//
//	site (na instância) -> qual serviço está carregado naquele navegador
//	videoVisivel        -> o usuário QUER ver o vídeo (escolha dele)
//	naTela (na instância) -> a área daquele navegador está aparecendo AGORA
//
// "naTela" existe separado porque a área do vídeo tem de sumir quando a
// janelinha é recolhida ou escondida, sem parar o som e sem mudar a escolha do
// usuário.
package player

import (
	"github.com/jchv/go-webview2/pkg/edge"
)

// instancia é um navegador embutido, com a janela própria que o hospeda.
type instancia struct {
	nav    *edge.Chromium
	janela uintptr // a janela do vídeo (veja janela_video.go)
	site   string // qual serviço está carregado (vazio = página em branco)
	naTela bool   // a janela dele está aparecendo agora

	// Último retângulo aplicado. Serve para NÃO repetir a chamada quando nada
	// mudou: mexer na janela a cada quadro fazia o vídeo PISCAR.
	x, y, larg, alt int32
}

var (
	// instancias guarda os navegadores. A chave é o serviço no modo "manter
	// carregado", ou a palavra "unico" no modo econômico.
	instancias = map[string]*instancia{}

	servicoAtual string // serviço em foco agora
	videoVisivel bool   // o usuário quer ver o vídeo?

	// ModoMultiplo: um navegador por serviço (a opção da aba Config).
	ModoMultiplo bool
)

const chaveUnica = "unico"

// chave diz em qual "gaveta" fica o navegador daquele serviço.
func chave(qual string) string {
	if ModoMultiplo {
		return qual
	}
	return chaveUnica
}

// atual devolve a instância em foco, ou nil se não houver nenhuma.
func atual() *instancia {
	if servicoAtual == "" {
		return nil
	}
	return instancias[chave(servicoAtual)]
}

// MostrarEmbutido prepara o serviço pedido: cria o navegador (se for a
// primeira vez), carrega o site e o deixa como o serviço em foco.
//
// IMPORTANTE — ele NÃO aparece aqui, e isso é de propósito. Quem mostra é a
// MostrarNaTela, chamada durante o desenho, quando já sabemos onde a janelinha
// do player está. Antes, esta função mostrava o navegador numa posição fixa
// escrita no código e o desenho o reposicionava logo depois: dava uma
// "piscada" e o vídeo aparecia num lugar e saltava para outro.
//
// ⚠️ Só pode ser chamada ENTRE quadros do ImGui: criar um navegador processa
// mensagens do Windows por dentro, e isso no meio de um quadro derruba o
// programa. Quem cuida disso é o abrirPlayerAgora(), na interface.
//
// Devolve false se o WebView2 não estiver disponível no PC — aí quem chamou
// cai no plano B (abrir no navegador de verdade).
func MostrarEmbutido(idJanela uintptr, qual string) bool {
	endereco, existe := enderecos[qual]
	if !existe {
		return false
	}

	inst := instancias[chave(qual)]
	if inst == nil {
		// SOB DEMANDA: a janela e o navegador deste serviço só nascem agora, no
		// primeiro uso. Nada é criado ao abrir o programa.
		//
		// A janela é PRÓPRIA (não é a do overlay) — veja janela_video.go para o
		// motivo: o navegador dentro da nossa janela derrubava a transparência
		// dela e a tela ficava preta.
		janela, ok := criarJanelaVideo(idJanela)
		if !ok {
			return false
		}

		nav := edge.NewChromium()
		if !nav.Embed(janela) {
			procDestroyWindow.Call(janela)
			return false // sem WebView2 no PC
		}
		inst = &instancia{nav: nav, janela: janela}
		instancias[chave(qual)] = inst

		// Fundo ESCURO E OPACO enquanto a página não carregou.
		//
		// Opaco de propósito: agora o navegador está numa janela normal, que não
		// sabe compor transparência. Com fundo transparente, o site aparecia
		// MUITO ESCURO. Escuro e opaco evita também o clarão branco padrão.
		if controle := nav.GetController(); controle != nil {
			if controle2 := controle.GetICoreWebView2Controller2(); controle2 != nil {
				controle2.PutDefaultBackgroundColor(
					edge.COREWEBVIEW2_COLOR{A: 255, R: 18, G: 18, B: 18})
			}
		}

		// O navegador fica visível DENTRO da janela dele, para sempre.
		//
		// ⚠️ Não confundir dois níveis de visibilidade:
		//
		//	nav.Show()/Hide()        -> o navegador dentro da janela dele
		//	ShowWindow(inst.janela)  -> a JANELA do vídeo na tela
		//
		// Quem esconde/mostra o vídeo é a JANELA (é ela que a interface
		// acompanha). O navegador em si nunca mais é escondido — quando eu
		// esqueci este Show(), a janela aparecia mas VAZIA: nada era mostrado.
		nav.Show()
	}

	// Carrega o site se ainda não for o que está nesta instância. No modo
	// econômico é aqui que a troca de serviço acontece (mesma "aba").
	if inst.site != qual {
		inst.nav.Navigate(endereco)
		inst.site = qual
		opacidadeAtual = -1 // página nova: o CSS de opacidade tem de voltar
	}

	// Esconde as OUTRAS: no modo "manter carregado" elas continuam tocando.
	for k, outra := range instancias {
		if k != chave(qual) && outra.naTela {
			procShowWindow.Call(outra.janela, esconder)
			outra.naTela = false
		}
	}

	servicoAtual = qual
	videoVisivel = true
	return true
}

// MostrarNaTela coloca a área do vídeo no lugar e garante que ela apareça.
// A interface chama isto a cada quadro, de DENTRO da janelinha do player.
//
// A ORDEM aqui importa: primeiro posiciona, só depois mostra. Ao contrário, o
// navegador apareceria por um instante no lugar antigo — a "piscada" que
// acontecia ao trocar de serviço.
//
// Nunca cria navegador (isso é só na MostrarEmbutido): pode ser chamada com
// segurança no meio de um quadro.
func MostrarNaTela(x, y, larg, alt int32) {
	inst := atual()
	if inst == nil || inst.site == "" || !videoVisivel {
		return
	}
	Reposicionar(x, y, larg, alt)
	if !inst.naTela {
		procShowWindow.Call(inst.janela, mostrarSemAtivar)
		inst.naTela = true
	}
}

// EsconderNaTela some com a área do vídeo SEM parar o som e SEM mudar a
// escolha do usuário. É o que a interface chama quando a janelinha do player
// não foi desenhada no quadro — recolhida, escondida (Insert) ou fechada.
func EsconderNaTela() {
	inst := atual()
	if inst == nil || !inst.naTela {
		return
	}
	procShowWindow.Call(inst.janela, esconder)
	inst.naTela = false

	// Esquece o último retângulo: se a janelinha do player for movida enquanto
	// o vídeo está escondido, ao reaparecer ele tem de ir para o lugar novo.
	inst.x, inst.y = -1, -1
}

// Reposicionar coloca a janela do vídeo no lugar, em coordenadas de TELA.
//
// Duas coisas acontecem aqui:
//  1. a JANELA do vídeo é movida/redimensionada;
//  2. o navegador é esticado para ocupar toda a área dela (0,0 até larg,alt) —
//     dentro da janela dele, o navegador começa no canto.
func Reposicionar(x, y, larg, alt int32) {
	inst := atual()
	if inst == nil || inst.janela == 0 {
		return
	}

	// ⚠️ SÓ mexe se algo mudou de verdade.
	//
	// A interface chama isto a cada quadro (para o vídeo acompanhar a
	// janelinha). Chamar o SetWindowPos 30 ou 60 vezes por segundo com os
	// MESMOS valores fazia o vídeo PISCAR sem parar — cada chamada mexe na
	// ordem das janelas e força repintura.
	if inst.x == x && inst.y == y && inst.larg == larg && inst.alt == alt {
		return
	}

	mudouTamanho := inst.larg != larg || inst.alt != alt
	inst.x, inst.y, inst.larg, inst.alt = x, y, larg, alt

	posicionarJanelaVideo(inst.janela, x, y, larg, alt)

	// O navegador só precisa ser reajustado quando o TAMANHO muda (mover a
	// janela não muda a área dele).
	if !mudouTamanho {
		return
	}

	controle := inst.nav.GetController()
	if controle == nil {
		return
	}
	// Pegamos o retângulo atual e trocamos os cantos (a biblioteca não deixa
	// criar um retângulo do zero por fora, mas deixa editar o existente).
	r, err := controle.GetBounds()
	if err != nil || r == nil {
		return
	}
	r.Left, r.Top, r.Right, r.Bottom = 0, 0, larg, alt
	controle.PutBounds(*r)
}

// OcultarVideo esconde a área de vídeo mas NÃO para o som — é o modo
// "só quero escutar": a música continua tocando por trás.
func OcultarVideo() {
	videoVisivel = false
	EsconderNaTela()
}

// FecharEmbutido para o serviço em foco: manda o navegador para uma página
// vazia (o que interrompe o som) e esconde a área.
//
// No modo "manter carregado", os OUTROS serviços continuam como estavam —
// para parar todos, use PararTodos().
func FecharEmbutido() {
	inst := atual()
	if inst == nil {
		return
	}
	inst.nav.Navigate("about:blank")
	procShowWindow.Call(inst.janela, esconder)
	inst.site = ""
	inst.naTela = false

	videoVisivel = false
	servicoAtual = ""
	opacidadeAtual = -1
}

// PararTodos interrompe todos os serviços, inclusive os de segundo plano.
func PararTodos() {
	for _, inst := range instancias {
		if inst.site == "" && !inst.naTela {
			continue
		}
		inst.nav.Navigate("about:blank")
		procShowWindow.Call(inst.janela, esconder)
		inst.site = ""
		inst.naTela = false
	}
	videoVisivel = false
	servicoAtual = ""
	opacidadeAtual = -1
}

// DescarregarSegundoPlano para os serviços que NÃO estão em foco.
//
// O que isso libera de verdade: a página é esvaziada, então o som para e a
// memória do conteúdo é devolvida. O navegador em si continua ocioso (a
// biblioteca não expõe como destruí-lo), consumindo pouco.
func DescarregarSegundoPlano() {
	for k, inst := range instancias {
		if k == chave(servicoAtual) || inst.site == "" {
			continue
		}
		inst.nav.Navigate("about:blank")
		procShowWindow.Call(inst.janela, esconder)
		inst.site = ""
		inst.naTela = false
	}
}

// QuantosCarregados conta quantos serviços estão carregados (para a Config
// mostrar quanto está em uso).
func QuantosCarregados() int {
	n := 0
	for _, inst := range instancias {
		if inst.site != "" {
			n++
		}
	}
	return n
}

// DefinirModoMultiplo troca entre "um navegador por serviço" e "um só".
//
// Ao DESLIGAR, para os serviços de segundo plano: senão ficaria som tocando
// sem nenhum jeito de chegar até ele pela interface.
func DefinirModoMultiplo(ligado bool) {
	if ligado == ModoMultiplo {
		return
	}
	ModoMultiplo = ligado
	DescarregarSegundoPlano()

	// A "gaveta" do serviço em foco mudou de nome (de "unico" para o nome do
	// serviço, ou o contrário). Se não houver navegador na gaveta nova, o
	// próximo clique no serviço cria — sob demanda, como deve ser.
	if inst := atual(); inst == nil {
		servicoAtual = ""
		videoVisivel = false
	}
}

// EstaNaTela diz se a área do vídeo está aparecendo AGORA. A interface usa isso
// para perceber o instante em que o vídeo sai da tela — é nesse momento que o
// Windows recompõe a janela e a transparência pode se perder.
func EstaNaTela() bool {
	inst := atual()
	return inst != nil && inst.naTela
}

// JanelaVisivel devolve a janela do vídeo que está aparecendo AGORA (0 se
// nenhuma). O overlay usa isso para se colocar logo ABAIXO dela na ordem das
// janelas — veja o comentário da regra 6 no overlay.go.
func JanelaVisivel() uintptr {
	inst := atual()
	if inst == nil || !inst.naTela {
		return 0
	}
	return inst.janela
}

// Estado conta como o player está: carregado? vídeo visível? qual serviço?
func Estado() (carregado bool, visivel bool, qual string) {
	inst := atual()
	if inst == nil || inst.site == "" {
		return false, false, ""
	}
	return true, videoVisivel, inst.site
}

// Focar entrega o foco do teclado ao navegador, para dar para digitar
// (fazer login, buscar um vídeo...). Sem isso o campo aceita o clique mas não
// escreve nada.
func Focar() {
	inst := atual()
	if inst == nil || inst.site == "" {
		return
	}
	// Primeiro a JANELA do vídeo vai para a frente (é ela que recebe o teclado
	// do Windows), depois o navegador coloca o cursor no campo.
	procSetForeground.Call(inst.janela)
	inst.nav.Focus()
}

// JanelaMoveu avisa o navegador que a janela-mãe mudou de lugar na tela
// (ele precisa saber para posicionar menus e caixas de diálogo direito).
func JanelaMoveu() {
	for _, inst := range instancias {
		inst.nav.NotifyParentWindowPositionChanged()
	}
}

// ───────────────────────── opacidade do vídeo ─────────────────────────────

// opacidadeAtual guarda o último valor aplicado, para não ficar mandando o
// mesmo comando ao navegador sem necessidade.
var opacidadeAtual float32 = -1

// DefinirOpacidade deixa o VÍDEO translúcido junto com o resto da interface.
//
// Por que precisa de código separado: o slider de opacidade mexe no
// "StyleVarAlpha" do ImGui, que vale só para o que o ImGui desenha. O vídeo é
// desenhado pelo motor do Edge, numa janela-filha por cima — ele não sabe nada
// da nossa opacidade. Então pedimos a transparência a ELE, em duas partes:
//
//  1. o fundo do WebView2 vira transparente (senão o próprio navegador pinta
//     um fundo opaco atrás da página e não haveria o que ver através);
//  2. injetamos um CSS que deixa a página toda com a opacidade escolhida.
//
// Precisa ser reaplicado de vez em quando: ao trocar de página (abrir um
// vídeo, por exemplo) o site monta o HTML de novo e o nosso CSS iria embora.
func DefinirOpacidade(alfa float32) {
	inst := atual()
	if inst == nil || inst.janela == 0 {
		return
	}
	if alfa == opacidadeAtual {
		return // nada mudou
	}
	opacidadeAtual = alfa

	// Opacidade uniforme na JANELA do vídeo — o Windows faz a mistura.
	// (Antes isto era CSS injetado na página; veja definirOpacidadeJanela.)
	definirOpacidadeJanela(inst.janela, alfa)
}
