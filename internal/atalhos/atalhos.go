// Pacote atalhos: as combinações de teclas que trocam de serviço sem tirar a
// mão do que você está fazendo (Alt+1 abre o YouTube, Alt+5 o WhatsApp...).
//
// ─── O que este pacote é, e o que ele NÃO é ───────────────────────────────
//
// Ele é a LÓGICA: guarda qual combinação faz o quê, confere o teclado a cada
// quadro, grava uma combinação nova que a pessoa apertou e salva tudo em disco.
//
// Ele NÃO sabe o que cada ação faz — não conhece player, áudio nem janela. Quem
// registra a lista de ações e executa a que disparou é a interface
// (`internal/ui`). É a regra da casa: aqui mora a decisão, não o efeito.
//
// ─── Por que "atalho global" precisa ser assim ────────────────────────────
//
// O overlay do Nimbus NUNCA fica com o foco do teclado (é o que faz ele não
// roubar a janela que você está usando). Sem foco, ele não recebe evento de
// tecla nenhum. Então a leitura é por PERGUNTA, não por aviso: a cada quadro
// perguntamos ao Windows "esta tecla está apertada?" (`GetAsyncKeyState`), do
// mesmo jeito que a tecla Insert já era lida.
//
// Não usamos `RegisterHotKey` (a função oficial do Windows para isto) porque ela
// exige uma janela com laço de mensagens só dela e RESERVA a combinação para o
// Nimbus no PC inteiro — se outro programa já usa Alt+1, o registro falha
// silenciosamente e o atalho simplesmente não funciona, sem explicação. Do jeito
// atual, o atalho funciona junto com o resto e a gente controla o que acontece.
package atalhos

import (
	"fmt"
	"syscall"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
)

// Acao é uma coisa que o Nimbus sabe fazer e que pode ganhar um atalho.
//
// A interface é quem monta esta lista (veja acoesDeAtalho, em overlay.go),
// justamente para este pacote não precisar conhecer os serviços.
type Acao struct {
	Nome   string // o nome curto, que vai para o arquivo: "youtube"
	Rotulo string // o nome que aparece na tela: "YouTube"

	// A combinação de fábrica. Zero em Tecla = nasce sem atalho.
	PadraoMods  Mods
	PadraoTecla uint16
}

// Atalho é uma combinação já amarrada a uma ação.
type Atalho struct {
	Acao  string
	Mods  Mods
	Tecla uint16
}

// Texto escreve o atalho como a pessoa lê: "Ctrl+Alt+1". Sem tecla, devolve
// vazio — a interface mostra "(nenhum)" nesse caso.
func (a Atalho) Texto() string {
	if a.Tecla == 0 {
		return ""
	}
	nome, conhecida := nomeDaTecla[a.Tecla]
	if !conhecida {
		// Não deveria acontecer (só gravamos teclas da tabela), mas mostrar o
		// número é melhor do que mostrar vazio e a pessoa achar que sumiu.
		nome = fmt.Sprintf("tecla %d", a.Tecla)
	}
	if m := a.Mods.Texto(); m != "" {
		return m + "+" + nome
	}
	return nome
}

var (
	acoes   []Acao            // a ordem em que aparecem na tela
	amarras map[string]Atalho // ação -> combinação
)

// Registrar recebe a lista de ações e aplica as combinações de fábrica.
// Chamar uma vez, ao iniciar, ANTES de Carregar.
func Registrar(lista []Acao) {
	acoes = lista
	amarras = make(map[string]Atalho, len(lista))
	for _, a := range lista {
		if a.PadraoTecla == 0 {
			continue
		}
		amarras[a.Nome] = Atalho{Acao: a.Nome, Mods: a.PadraoMods, Tecla: a.PadraoTecla}
	}
}

// Acoes devolve a lista registrada, na ordem de exibição.
func Acoes() []Acao { return acoes }

// Do devolve o atalho de uma ação (a segunda resposta é falsa se não tem).
func Do(acao string) (Atalho, bool) {
	a, tem := amarras[acao]
	return a, tem
}

// Definir amarra uma combinação a uma ação, conferindo as duas regras que
// impedem o atalho de estragar o uso normal do PC.
func Definir(acao string, m Mods, tecla uint16) error {
	if tecla == 0 {
		return fmt.Errorf("sem tecla")
	}
	if _, conhecida := nomeDaTecla[tecla]; !conhecida {
		return fmt.Errorf("esta tecla nao serve para atalho")
	}

	// REGRA 1: todo atalho precisa de pelo menos um modificador.
	//
	// Sem isto, configurar "1" faria o Nimbus trocar de serviço toda vez que
	// você digitasse o número 1 em QUALQUER programa — no Word, no jogo, no
	// campo de busca. Como o Nimbus lê o teclado do PC inteiro (ele não tem
	// foco), não há como saber que a tecla "não era para ele".
	if m == 0 {
		return fmt.Errorf("segure Ctrl, Alt, Shift ou Win junto")
	}

	// REGRA 2: a mesma combinação não pode ficar em duas ações.
	//
	// Ficaria imprevisível: as duas dispariam no mesmo toque, na ordem em que
	// estivessem guardadas. Então a nova combinação TIRA a antiga de quem a
	// tinha — igual ao que um jogo faz ao reconfigurar controle.
	for outra, a := range amarras {
		if outra != acao && a.Mods == m && a.Tecla == tecla {
			delete(amarras, outra)
		}
	}

	amarras[acao] = Atalho{Acao: acao, Mods: m, Tecla: tecla}
	precisaSalvar = true
	return nil
}

// Limpar tira o atalho de uma ação (ela continua funcionando pelo mouse).
func Limpar(acao string) {
	if _, tinha := amarras[acao]; !tinha {
		return
	}
	delete(amarras, acao)
	precisaSalvar = true
}

// precisaSalvar avisa que alguma coisa mudou e o arquivo está atrasado.
//
// Existe para o gravar em disco ter UM caminho só: a interface não precisa
// lembrar de salvar depois de cada botão (e um dia esquecer justamente num
// deles) — ela olha esta bandeirinha a cada quadro e salva quando ela subir.
var precisaSalvar bool

// PrecisaSalvar diz se há mudança ainda não gravada em disco.
func PrecisaSalvar() bool { return precisaSalvar }

// MarcarSalvo baixa a bandeirinha. Chamar depois de salvar com sucesso — se o
// salvamento falhar, ela fica em pé e a interface tenta de novo no quadro
// seguinte (por exemplo, se o disco estava momentaneamente ocupado).
func MarcarSalvo() { precisaSalvar = false }

// ─────────────────────────── a leitura do teclado ─────────────────────────

// Conferir olha o teclado e devolve o nome da ação que a pessoa acabou de
// disparar (vazio se nenhuma). Chamar UMA vez por quadro.
//
// ⚠️ A PEGADINHA que dá nome a este comentário: o bit 1 da resposta do
// `GetAsyncKeyState` quer dizer "esta tecla foi apertada desde a última vez que
// alguém perguntou" — e o Windows APAGA esse bit quando responde. Ou seja: a
// primeira pergunta consome o toque, e uma segunda pergunta sobre a MESMA tecla,
// no mesmo quadro, responderia "não foi apertada".
//
// Isso quebraria dois atalhos que dividem a tecla (Alt+1 e Ctrl+1): o primeiro
// da lista comeria o toque do outro. Por isso perguntamos UMA vez por tecla, ao
// juntar tudo num "retrato" (o mapa `toques`), e só depois comparamos os
// atalhos com esse retrato.
//
// A MESMA pegadinha vale entre pacotes: a interface lê a tecla Insert (o
// liga/desliga dos painéis) por conta própria — se nós perguntássemos de novo,
// a resposta seria "não" e um atalho gravado com Insert nunca dispararia. Por
// isso ela EMPRESTA o que já leu pelo `toquesJaLidos` (tecla -> foi apertada),
// e essas teclas entram no retrato sem nova pergunta. Pode ser nil.
func Conferir(toquesJaLidos map[uint16]bool) string {
	// Enquanto está gravando, ninguém dispara: as teclas estão sendo apertadas
	// para ESCOLHER o atalho, não para usá-lo.
	if gravandoPara != "" {
		conferirGravacao(toquesJaLidos)
		return ""
	}

	toques := map[uint16]bool{}
	for tecla, tocada := range toquesJaLidos {
		toques[tecla] = tocada
	}
	for _, a := range amarras {
		if _, jaPerguntei := toques[a.Tecla]; jaPerguntei {
			continue
		}
		toques[a.Tecla] = foiApertada(a.Tecla)
	}

	// A ordem de exibição decide quem ganha, para o resultado ser sempre o
	// mesmo (percorrer um mapa em Go dá ordem aleatória a cada volta).
	for _, acao := range acoes {
		a, tem := amarras[acao.Nome]
		if !tem || !toques[a.Tecla] {
			continue
		}
		if modificadoresSegurados(a.Mods) {
			return acao.Nome
		}
	}
	return ""
}

// foiApertada pergunta ao Windows se a tecla foi apertada desde a última
// pergunta (o bit 1). Consome o toque — veja o aviso em Conferir.
func foiApertada(tecla uint16) bool {
	estado, _, _ := procGetAsyncKeyState.Call(uintptr(tecla))
	return estado&1 != 0
}

// estaSegurada pergunta se a tecla está apertada AGORA (o bit alto). Não
// consome nada, então pode ser chamada quantas vezes for preciso.
func estaSegurada(tecla uint16) bool {
	estado, _, _ := procGetAsyncKeyState.Call(uintptr(tecla))
	return estado&0x8000 != 0
}

// modificadoresSegurados confere se as teclas que o atalho pede estão sendo
// seguradas — e SÓ elas.
//
// O "e só elas" é importante: sem isso, um atalho "Alt+1" também dispararia com
// "Ctrl+Alt+1", que pode ser o atalho de outra coisa (no teclado brasileiro,
// aliás, o AltGr manda Ctrl+Alt junto). Exigir exatidão deixa cada combinação
// com um significado só.
func modificadoresSegurados(m Mods) bool {
	return segurado(ModCtrl) == (m&ModCtrl != 0) &&
		segurado(ModAlt) == (m&ModAlt != 0) &&
		segurado(ModShift) == (m&ModShift != 0) &&
		segurado(ModWin) == (m&ModWin != 0)
}

// segurado diz se aquele modificador está apertado, de qualquer um dos dois
// lados do teclado.
func segurado(m Mods) bool {
	switch m {
	case ModCtrl:
		return estaSegurada(vkCtrl)
	case ModAlt:
		return estaSegurada(vkAlt)
	case ModShift:
		return estaSegurada(vkShift)
	case ModWin:
		// A tecla Windows não tem código "qualquer lado", então olhamos os dois.
		return estaSegurada(vkWinEsq) || estaSegurada(vkWinDir)
	}
	return false
}

// ModsSeguradosAgora devolve os modificadores apertados neste instante. A
// interface usa para mostrar o que a pessoa está segurando durante a gravação.
func ModsSeguradosAgora() Mods {
	var m Mods
	if segurado(ModCtrl) {
		m |= ModCtrl
	}
	if segurado(ModAlt) {
		m |= ModAlt
	}
	if segurado(ModShift) {
		m |= ModShift
	}
	if segurado(ModWin) {
		m |= ModWin
	}
	return m
}

// ─────────────────────────── gravar uma combinação ────────────────────────

var (
	gravandoPara string // a ação que está esperando uma combinação
	erroAoGravar string // o motivo da última recusa, para a tela explicar
)

// Gravar entra no modo "estou esperando você apertar a combinação".
func Gravar(acao string) {
	gravandoPara = acao
	erroAoGravar = ""
	// Limpa o bit de "foi apertada" das teclas conhecidas, senão um toque
	// anterior (o próprio clique de teclado que abriu a gravação) seria lido
	// como se fosse a combinação escolhida.
	for tecla := range nomeDaTecla {
		foiApertada(tecla)
	}
}

// Gravando devolve a ação que está esperando combinação (vazio se nenhuma).
func Gravando() string { return gravandoPara }

// ErroAoGravar devolve o motivo da última combinação recusada.
func ErroAoGravar() string { return erroAoGravar }

// CancelarGravacao desiste sem mudar nada.
func CancelarGravacao() {
	gravandoPara = ""
	erroAoGravar = ""
}

// conferirGravacao procura a tecla que a pessoa apertou e fecha o atalho.
//
// Roda dentro do Conferir, uma vez por quadro. Percorrer a tabela inteira de
// teclas custa umas 90 perguntas ao Windows — insignificante, e só acontece
// enquanto a gravação está aberta.
//
// O `toquesJaLidos` é o mesmo empréstimo do Conferir: teclas que a interface
// já leu neste quadro (o Insert). Sem ele, gravar uma combinação com Insert
// não funcionaria — o toque já teria sido gasto lá.
func conferirGravacao(toquesJaLidos map[uint16]bool) {
	tocada := func(tecla uint16) bool {
		if v, jaLida := toquesJaLidos[tecla]; jaLida {
			return v
		}
		return foiApertada(tecla)
	}

	// Esc desiste (é o que qualquer programa faz, então ninguém precisa
	// aprender). Vem antes de tudo para nunca virar atalho por acidente.
	const vkEsc = 0x1B
	if tocada(vkEsc) {
		CancelarGravacao()
		return
	}

	m := ModsSeguradosAgora()
	for tecla := range nomeDaTecla {
		if tecla == vkEsc || !tocada(tecla) {
			continue
		}
		if err := Definir(gravandoPara, m, tecla); err != nil {
			// Não fecha a gravação: a pessoa erra, lê o motivo na tela e tenta
			// de novo sem ter de clicar em "Gravar" outra vez.
			erroAoGravar = err.Error()
			return
		}
		gravandoPara = ""
		erroAoGravar = ""
		return
	}
}
