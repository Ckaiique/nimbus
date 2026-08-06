// Teste da regra que decide quando o vídeo sai da frente.
//
// Por que testar isto: essa regra já errou duas vezes (uma escondia o vídeo ao
// passar o mouse sobre ele; outra escondia ao apontar para o painel Música, que
// fica AO LADO e não cobre nada). São só contas de retângulo, então dá para
// provar que está certo sem abrir o programa.
//
// Rode com: go test ./internal/ui/
package ui

import (
	"testing"

	"github.com/AllenDang/cimgui-go/imgui"
)

func TestPainelTapaOVideo(t *testing.T) {
	// A área do vídeo em todos os casos: começa em (420,80) e mede 560x380.
	videoPos := imgui.Vec2{X: 420, Y: 80}
	videoTam := imgui.Vec2{X: 560, Y: 380}

	// Os painéis, no formato {x, y, largura, altura}.
	painelAoLado := [4]float32{80, 80, 310, 306}    // à esquerda do vídeo
	painelSobreOVideo := [4]float32{500, 150, 310, 306} // arrastado para cima dele

	casos := []struct {
		nome     string
		mouse    imgui.Vec2
		paineis  [][4]float32
		esperado bool
	}{
		{
			// O caso que estava com bug: o painel Música fica ao lado do vídeo,
			// então apontar para ele NÃO pode esconder nada.
			nome:     "cursor no painel ao lado do video",
			mouse:    imgui.Vec2{X: 200, Y: 200},
			paineis:  [][4]float32{painelAoLado},
			esperado: false,
		},
		{
			// O caso que a regra existe para resolver.
			nome:     "cursor no painel arrastado para cima do video",
			mouse:    imgui.Vec2{X: 600, Y: 300},
			paineis:  [][4]float32{painelSobreOVideo},
			esperado: true,
		},
		{
			// Painel cobre o vídeo, mas o cursor está longe: o vídeo fica.
			nome:     "painel cobre o video mas o cursor esta fora",
			mouse:    imgui.Vec2{X: 1500, Y: 900},
			paineis:  [][4]float32{painelSobreOVideo},
			esperado: false,
		},
		{
			// Cursor sobre o próprio vídeo, sem painel nenhum ali: o vídeo fica.
			// (era o outro bug: o vídeo sumia ao passar o mouse em cima dele)
			nome:     "cursor sobre o video, sem painel na frente",
			mouse:    imgui.Vec2{X: 700, Y: 300},
			paineis:  [][4]float32{painelAoLado},
			esperado: false,
		},
		{
			nome:     "sem painel nenhum",
			mouse:    imgui.Vec2{X: 700, Y: 300},
			paineis:  nil,
			esperado: false,
		},
		{
			// Com vários painéis, basta UM cobrir e ter o cursor dentro.
			nome:     "dois paineis, o cursor esta no que cobre",
			mouse:    imgui.Vec2{X: 600, Y: 300},
			paineis:  [][4]float32{painelAoLado, painelSobreOVideo},
			esperado: true,
		},
		{
			// Painel ENCAIXADO com o player (viraram abas de uma janela só):
			// ocupa o mesmo retângulo, e isso NÃO conta como "cobrir".
			// Sem essa exceção o vídeo piscava ao passar o mouse nele.
			nome:     "painel encaixado junto (mesmo retangulo)",
			mouse:    imgui.Vec2{X: 700, Y: 300},
			paineis:  [][4]float32{{420, 80, 560, 380}},
			esperado: false,
		},
		{
			// Mesma coisa, com a diferença de 1 ou 2 pixels que o encaixe deixa.
			nome:     "painel encaixado com 2px de diferenca",
			mouse:    imgui.Vec2{X: 700, Y: 300},
			paineis:  [][4]float32{{421, 82, 558, 378}},
			esperado: false,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			obtido := painelTapaOVideo(caso.mouse, videoPos, videoTam, caso.paineis)
			if obtido != caso.esperado {
				t.Errorf("esperava %v, obtive %v", caso.esperado, obtido)
			}
		})
	}
}

// TestBarraDeTituloNaoEscondeOVideo cobre o incômodo que o dono relatou: passar
// o mouse na aba do painel Música fazia o vídeo do YouTube sumir.
//
// # Por que acontecia
//
// A regra antiga era grossa demais: se o painel encostasse no vídeo em QUALQUER
// pedacinho, então o cursor em QUALQUER lugar do painel escondia o vídeo
// inteiro. Só que a barra de título fica em cima do painel, longe da área do
// vídeo — não estorva nada, e mesmo assim o vídeo piscava.
//
// A regra nova olha só a parte onde um está mesmo por cima do outro.
func TestBarraDeTituloNaoEscondeOVideo(t *testing.T) {
	// Vídeo à direita, ocupando boa parte da tela.
	videoPos := imgui.Vec2{X: 420, Y: 200}
	videoTam := imgui.Vec2{X: 560, Y: 380}

	// Painel encostando no vídeo pelo canto: a barra de título dele fica ACIMA
	// da área do vídeo; só o corpo entra por cima.
	painel := [4]float32{300, 100, 300, 300} // de (300,100) até (600,400)

	casos := []struct {
		nome     string
		mouse    imgui.Vec2
		esperado bool
		porque   string
	}{
		{
			nome:     "cursor na barra de titulo do painel",
			mouse:    imgui.Vec2{X: 450, Y: 115},
			esperado: false,
			porque:   "a barra fica acima do video; nao tapa nada",
		},
		{
			nome:     "cursor na parte do painel que esta ACIMA do video",
			mouse:    imgui.Vec2{X: 350, Y: 150},
			esperado: false,
			porque:   "esse pedaco do painel nao encosta no video",
		},
		{
			nome:     "cursor na parte do painel que esta A ESQUERDA do video",
			mouse:    imgui.Vec2{X: 330, Y: 300},
			esperado: false,
			porque:   "o video comeca em x=420; aqui o painel esta sozinho",
		},
		{
			nome:     "cursor onde o painel REALMENTE cobre o video",
			mouse:    imgui.Vec2{X: 500, Y: 300},
			esperado: true,
			porque:   "aqui o video estorva de verdade: tem de sair da frente",
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			veio := painelTapaOVideo(c.mouse, videoPos, videoTam, [][4]float32{painel})
			if veio != c.esperado {
				t.Fatalf("esperava %v, veio %v — %s", c.esperado, veio, c.porque)
			}
		})
	}
}

// TestVideoDentroDoPainelNaoSomeSozinho cobre o modo em que o vídeo é arrastado
// para DENTRO do painel Música (viram uma janela só).
//
// Nesse arranjo o painel CONTÉM o vídeo. Com a regra antiga, qualquer canto do
// painel — a aba, os ícones dos serviços, o volume — escondia o vídeo, porque
// "o painel cobre o vídeo" era sempre verdade.
func TestVideoDentroDoPainelNaoSomeSozinho(t *testing.T) {
	// O painel Música inteiro.
	painel := [4]float32{80, 80, 356, 268}
	// A área do vídeo DENTRO dele: sobra a coluna de ícones à esquerda, o
	// volume à direita e o rodapé embaixo.
	videoPos := imgui.Vec2{X: 148, Y: 112}
	videoTam := imgui.Vec2{X: 254, Y: 184}

	fora := []struct {
		nome  string
		mouse imgui.Vec2
	}{
		{"na aba do painel", imgui.Vec2{X: 200, Y: 90}},
		{"nos icones dos servicos", imgui.Vec2{X: 100, Y: 200}},
		{"no volume, na borda direita", imgui.Vec2{X: 425, Y: 200}},
		{"no rodape dos controles", imgui.Vec2{X: 250, Y: 330}},
	}

	for _, c := range fora {
		t.Run(c.nome, func(t *testing.T) {
			if painelTapaOVideo(c.mouse, videoPos, videoTam, [][4]float32{painel}) {
				t.Fatal("o vídeo sumiu com o cursor fora da área dele — é o bug relatado")
			}
		})
	}
}
