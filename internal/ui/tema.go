// Tema visual do overlay — portado do projeto DLL do Kaique
// (dll/Project/Menu/Theme.cpp), que ficou muito bonito.
//
// A ideia: em vez de escolher 30 cores na mão, definimos apenas 5 cores base
// (destaque, fundo, cartão, texto, borda) e o resto é CALCULADO a partir
// delas (clareando, escurecendo e misturando com branco). Assim o tema fica
// harmônico automaticamente e trocar de cor é trocar 5 valores.
package ui

import (
	"image/color"

	"github.com/AllenDang/cimgui-go/imgui"
)

// Preset é um tema pronto: as 5 cores base com um nome.
type Preset struct {
	Nome                          string
	Destaque, Fundo, Cartao, Texto, Borda color.RGBA
}

// Os mesmos presets do projeto DLL.
var Presets = []Preset{
	{"Crimson", rgb(220, 40, 40), rgb(8, 8, 10), rgb(12, 12, 14), rgb(90, 85, 85), rgb(30, 28, 28)},
	{"Midnight", rgb(80, 130, 230), rgb(9, 10, 14), rgb(14, 16, 22), rgb(110, 116, 130), rgb(30, 34, 44)},
	{"Ocean", rgb(40, 190, 200), rgb(7, 12, 14), rgb(11, 18, 21), rgb(96, 116, 120), rgb(26, 40, 44)},
	{"Forest", rgb(80, 200, 110), rgb(8, 11, 9), rgb(12, 17, 13), rgb(96, 112, 100), rgb(28, 40, 30)},
	{"Amethyst", rgb(170, 90, 230), rgb(11, 9, 14), rgb(16, 13, 21), rgb(112, 102, 124), rgb(36, 30, 46)},
	{"Spotify", rgb(30, 215, 96), rgb(12, 12, 12), rgb(18, 18, 18), rgb(140, 140, 140), rgb(40, 40, 40)},
	{"Mono", rgb(200, 200, 205), rgb(10, 10, 11), rgb(15, 15, 16), rgb(100, 100, 104), rgb(34, 34, 36)},
}

// Paleta é o tema inteiro, já calculado a partir das 5 cores base.
type Paleta struct {
	Destaque, DestaqueForte color.RGBA
	Fundo, BarraTitulo      color.RGBA
	Cartao, Borda           color.RGBA
	Texto, TextoFraco       color.RGBA
	Caixa, CaixaHover, CaixaAtiva color.RGBA
	Botao, BotaoHover       color.RGBA
}

var (
	// PresetAtual é o índice do tema escolhido (aba Config).
	PresetAtual int32 = 5 // começa no Spotify

	// Opacidade geral da interface, 0.30 a 1.00 (aba Config).
	Alfa float32 = 0.92

	pal Paleta // paleta calculada do quadro atual
)

func rgb(r, g, b uint8) color.RGBA { return color.RGBA{r, g, b, 255} }

// escalar clareia (f > 1) ou escurece (f < 1) uma cor.
func escalar(c color.RGBA, f float32) color.RGBA {
	return color.RGBA{limite(float32(c.R) * f), limite(float32(c.G) * f), limite(float32(c.B) * f), c.A}
}

// misturar caminha de c até d na proporção t (0 a 1).
func misturar(c, d color.RGBA, t float32) color.RGBA {
	return color.RGBA{
		limite(float32(c.R) + (float32(d.R)-float32(c.R))*t),
		limite(float32(c.G) + (float32(d.G)-float32(c.G))*t),
		limite(float32(c.B) + (float32(d.B)-float32(c.B))*t),
		c.A,
	}
}

func limite(v float32) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// recalcularPaleta deriva o tema inteiro das 5 cores base do preset atual.
func recalcularPaleta() {
	if PresetAtual < 0 || int(PresetAtual) >= len(Presets) {
		PresetAtual = 0
	}
	p := Presets[PresetAtual]
	branco := color.RGBA{255, 255, 255, 255}

	pal = Paleta{
		Destaque:      p.Destaque,
		DestaqueForte: escalar(p.Destaque, 1.20),

		Fundo:       p.Fundo,
		BarraTitulo: escalar(p.Fundo, 1.30),
		Cartao:      p.Cartao,
		Borda:       p.Borda,

		Texto:      misturar(p.Texto, branco, 0.80),
		TextoFraco: escalar(p.Texto, 1.00),

		Caixa:      escalar(p.Cartao, 1.55),
		CaixaHover: escalar(p.Cartao, 2.40),
		CaixaAtiva: escalar(p.Cartao, 3.20),

		Botao:      escalar(p.Cartao, 2.20),
		BotaoHover: escalar(p.Cartao, 3.40),
	}
}

// aplicarTemaPersistente grava o tema no estilo PERMANENTE do ImGui.
//
// POR QUE PERMANENTE, e não "empilhar" (Push/Pop) a cada quadro:
//
// Quando você junta duas janelinhas numa só (arrastando uma sobre a outra, o
// recurso de "encaixe"), o ImGui cria por conta própria uma janela hospedeira
// com a barra de abas. Ela é criada **dentro do NewFrame**, ou seja, ANTES de
// qualquer cor que a gente empilhasse durante o desenho — e por isso saía com
// a aparência crua do ImGui, com bordas e barra de título padrão.
//
// Gravando no estilo permanente (e chamando isto ANTES do NewFrame), TODA
// janela nasce já com o nosso tema, inclusive as que o ImGui cria sozinho.
//
// ⚠️ Junto com isso, o tema padrão do giu precisa ser desligado
// (`janela.SetStyle(g.Style())` em Rodar), senão ele empilha o azul dele por
// cima do nosso a cada quadro.
func aplicarTemaPersistente() {
	recalcularPaleta()

	estilo := imgui.CurrentStyle()

	// O vetor de cores é uma CÓPIA: mexemos nela e devolvemos inteira.
	cores := estilo.Colors()

	paraAplicar := []struct {
		id  imgui.Col
		cor color.RGBA
	}{
		{imgui.ColWindowBg, pal.Fundo},
		{imgui.ColChildBg, pal.Cartao},
		{imgui.ColPopupBg, escalar(pal.Cartao, 1.10)},
		{imgui.ColBorder, pal.Borda},
		{imgui.ColTitleBg, pal.BarraTitulo},
		{imgui.ColTitleBgActive, pal.BarraTitulo},
		{imgui.ColTitleBgCollapsed, pal.BarraTitulo},
		{imgui.ColText, pal.Texto},
		{imgui.ColTextDisabled, escalar(pal.TextoFraco, 0.55)},
		{imgui.ColFrameBg, pal.Caixa},
		{imgui.ColFrameBgHovered, pal.CaixaHover},
		{imgui.ColFrameBgActive, pal.CaixaAtiva},
		{imgui.ColSliderGrab, pal.Destaque},
		{imgui.ColSliderGrabActive, pal.DestaqueForte},
		{imgui.ColButton, pal.Botao},
		{imgui.ColButtonHovered, pal.BotaoHover},
		{imgui.ColButtonActive, pal.Destaque},
		{imgui.ColHeader, escalar(pal.Cartao, 2.60)},
		{imgui.ColHeaderHovered, escalar(pal.Cartao, 4.20)},
		{imgui.ColHeaderActive, pal.Destaque},
		{imgui.ColCheckMark, pal.Destaque},
		{imgui.ColPlotHistogram, pal.Destaque},
		{imgui.ColSeparator, pal.Borda},
		{imgui.ColScrollbarBg, escalar(pal.Fundo, 1.10)},
		{imgui.ColScrollbarGrab, escalar(pal.Cartao, 3.20)},
		{imgui.ColScrollbarGrabHovered, pal.Destaque},
		{imgui.ColResizeGrip, escalar(pal.Cartao, 2.60)},
		{imgui.ColResizeGripHovered, comAlfa(pal.Destaque, 0.50)},
		{imgui.ColResizeGripActive, pal.Destaque},
		{imgui.ColTab, pal.Fundo},
		{imgui.ColTabHovered, pal.BotaoHover},
		{imgui.ColTabSelected, pal.Botao},
		// Cor do realce ao arrastar uma janelinha sobre a outra (encaixe).
		{imgui.ColDockingPreview, comAlfa(pal.Destaque, 0.55)},

		// ⚠️ AS TRÊS CORES ABAIXO PRECISAM SER TOTALMENTE TRANSPARENTES.
		//
		// O ImGui usa elas para pintar a TELA INTEIRA em certas situações:
		//
		//	DockingEmptyBg    -> a área vazia de um encaixe que cobre a tela
		//	ModalWindowDimBg  -> escurece tudo quando abre uma janela modal
		//	NavWindowingDimBg -> escurece tudo ao trocar de janela pelo teclado
		//
		// Num programa comum isso é bonito. Num OVERLAY é desastre: a tela
		// inteira fica com um véu escuro e só a nossa interface aparece — foi
		// exatamente o bug de "a tela toda ficando meio preta". Aqui NUNCA
		// pintamos a tela toda.
		{imgui.ColDockingEmptyBg, transparente()},
		{imgui.ColModalWindowDimBg, transparente()},
		{imgui.ColNavWindowingDimBg, transparente()},
	}
	for _, c := range paraAplicar {
		if int(c.id) < len(cores) { // segurança: índice dentro do vetor
			cores[c.id] = cor4(c.cor)
		}
	}
	estilo.SetColors(&cores)

	// Medidas: cantos bem arredondados, como no projeto DLL.
	estilo.SetAlpha(Alfa)
	estilo.SetWindowRounding(10)
	estilo.SetChildRounding(8)
	estilo.SetPopupRounding(8)
	estilo.SetFrameRounding(6)
	estilo.SetTabRounding(6)
	estilo.SetGrabRounding(12)
	estilo.SetGrabMinSize(14)
	estilo.SetScrollbarSize(8)
	estilo.SetWindowBorderSize(1)
	estilo.SetWindowPadding(imgui.Vec2{X: 12, Y: 10})
	estilo.SetFramePadding(imgui.Vec2{X: 9, Y: 6})
	estilo.SetItemSpacing(imgui.Vec2{X: 8, Y: 7})
}

// transparente é o "nada": usada onde o ImGui pintaria a tela inteira.
func transparente() color.RGBA { return color.RGBA{0, 0, 0, 0} }

// comAlfa devolve a cor com outra transparência (0 a 1).
func comAlfa(c color.RGBA, a float32) color.RGBA {
	return color.RGBA{c.R, c.G, c.B, limite(float32(c.A) * a)}
}

// cor4 converte a cor do Go para o formato do ImGui (0.0 a 1.0).
func cor4(c color.RGBA) imgui.Vec4 {
	return imgui.Vec4{
		X: float32(c.R) / 255, Y: float32(c.G) / 255,
		Z: float32(c.B) / 255, W: float32(c.A) / 255,
	}
}
