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
