// Depuração TEMPORÁRIA da ordem das janelas (pode ser removida sem dó).
//
// Com NIMBUS_DEBUG_ORDEM=<caminho de arquivo>, escreve ali, uma vez por
// segundo, qual "alvo na ordem" o overlay está usando no SetWindowPos:
// -1 = topo absoluto (HWND_TOPMOST); -2 = sai do topo; outro número = o
// identificador da janela do vídeo (o overlay entra logo abaixo dela).
package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
)

var (
	arquivoOrdem  = os.Getenv("NIMBUS_DEBUG_ORDEM")
	ultimoRegistro time.Time
)

// Com NIMBUS_DEBUG_PAINEIS=<caminho de arquivo>, grava ali (1x por segundo)
// o retângulo de cada painel registrado no quadro, em coordenadas de TELA —
// para achar os painéis de fora do programa sem depender de chute.
var (
	arquivoPaineis = os.Getenv("NIMBUS_DEBUG_PAINEIS")

	// A cada segundo abre uma "janela de gravação" de 100ms: todos os painéis
	// registrados nesse intervalo entram no arquivo (uma trava simples de
	// 1 segundo deixaria passar só o PRIMEIRO painel de cada quadro).
	inicioJanelaLog time.Time
)

// depurarPainel grava o nome e o retângulo do painel que está sendo desenhado.
//
// O NOME é o que faz o arquivo servir de verdade: o script que tira as imagens
// do README (`docs/gerar-imagens.ps1`) lê daqui as coordenadas de cada painel em
// vez de ter números escritos na mão. Assim ele não recorta o lugar errado
// quando a interface muda — e, o que importa mais, se um painel NÃO está na tela
// (escondido pelo Insert, por exemplo) ele simplesmente não aparece no arquivo, e
// o script para em vez de fotografar o que estiver atrás — que é o trabalho da
// pessoa. Já aconteceu.
func depurarPainel(pos, tam imgui.Vec2) {
	if arquivoPaineis == "" {
		return
	}
	if time.Since(inicioJanelaLog) > time.Second {
		inicioJanelaLog = time.Now()
	}
	if time.Since(inicioJanelaLog) > 100*time.Millisecond {
		return
	}
	f, err := os.OpenFile(arquivoPaineis, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "painel %s tela=(%.0f,%.0f) %.0fx%.0f\n",
		nomeDoPainelAtual(), pos.X+float32(telaX), pos.Y+float32(telaY), tam.X, tam.Y)
}

// nomeDoPainelAtual devolve o apelido do painel que está sendo desenhado — o
// pedaço depois de "###" no título ("###musica" -> "musica").
//
// Usamos esse pedaço, e não o título visível, porque ele é o nome ESTÁVEL: o
// título do painel Música muda para "Musica (demo - sem som)" quando não há som,
// e o script que lê este arquivo não deve depender disso.
func nomeDoPainelAtual() string {
	janelaAtual := imgui.InternalCurrentWindowRead()
	if janelaAtual == nil || janelaAtual.CData == nil {
		return "?"
	}
	nome := janelaAtual.Name()
	if _, apelido, achou := strings.Cut(nome, "###"); achou {
		return apelido
	}
	// Sem "###" é algum popup nosso (o menuzinho da engrenagem, por exemplo).
	return nome
}

func depurarOrdem(alvo uintptr) {
	if arquivoOrdem == "" || time.Since(ultimoRegistro) < time.Second {
		return
	}
	ultimoRegistro = time.Now()
	f, err := os.OpenFile(arquivoOrdem, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "alvo=%d (int32=%d)\n", alvo, int32(alvo))
}
