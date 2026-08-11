// Depuração temporária: com a variável de ambiente NIMBUS_DEBUG=1,
// escreve no terminal o que o ImGui está enxergando do mouse. Serve para
// descobrir por que ele reivindica (ou não) o cursor.
package ui

import (
	"fmt"
	"os"
	"strconv"

	"github.com/AllenDang/cimgui-go/imgui"

	"nimbus/internal/player"
)

var contaQuadros int

// semFantasma: com NIMBUS_NOGHOST=1 o clique-atravessa fica desligado.
// Serve para testar se é ele que atrapalha o hover do ImGui.
var semFantasma = os.Getenv("NIMBUS_NOGHOST") == "1"

// Auxiliares de teste (não afetam o uso normal):
//
//	NIMBUS_DEBUG_PLAYER=youtube|music -> abre o player já ao iniciar
//	NIMBUS_DEBUG_ALFA=0.5             -> começa com essa opacidade
//	NIMBUS_DEBUG_ESCONDER=1           -> esconde os painéis sozinho depois de
//	                                     ~5 segundos (testa se o vídeo some junto)
var (
	playerAutomatico = os.Getenv("NIMBUS_DEBUG_PLAYER")
	playerJaAberto   bool

	esconderSozinho    = os.Getenv("NIMBUS_DEBUG_ESCONDER") == "1"
	quadrosAteEsconder int
	jaEscondeu         bool

	pedirRecolher      = os.Getenv("NIMBUS_DEBUG_RECOLHER") == "1"
	quadrosAteRecolher int

	// NIMBUS_DEBUG_MULTI=1     -> começa com "manter cada serviço carregado"
	// NIMBUS_DEBUG_TROCAR=music -> troca para esse serviço depois de ~6s
	// NIMBUS_DEBUG_CONFIG=1 -> abre a aba Config já ao iniciar (útil para
	// tirar as imagens do README sem precisar clicar).
	abrirConfig   = os.Getenv("NIMBUS_DEBUG_CONFIG") == "1"
	jaAbriuConfig bool

	// NIMBUS_DEBUG_SEM_CONTROLES=1 -> começa com a opção "Mostrar controles no
	// painel" DESLIGADA (o vídeo ocupando o painel inteiro). Existe para
	// conferir esse modo sem precisar achar e clicar na caixinha da Config.
	semControlesNoInicio = os.Getenv("NIMBUS_DEBUG_SEM_CONTROLES") == "1"
	jaTirouOsControles   bool
	multiNoInicio    = os.Getenv("NIMBUS_DEBUG_MULTI") == "1"
	trocarPara       = os.Getenv("NIMBUS_DEBUG_TROCAR")
	quadrosAteTrocar int
	jaTrocou         bool

	quadrosAteFechar int
	jaFechou         bool
)

// recolherSozinho manda recolher a janelinha do player depois de uns segundos
// (NIMBUS_DEBUG_RECOLHER=1). Serve para testar se o vídeo se esconde junto.
func recolherSozinho() bool {
	if !pedirRecolher {
		return false
	}
	quadrosAteRecolher++
	// Só no quadro exato: depois disso deixamos o usuário controlar.
	return quadrosAteRecolher == 200
}

// aplicarAuxiliaresDeTeste roda no começo de cada quadro.
func aplicarAuxiliaresDeTeste() {
	if v := os.Getenv("NIMBUS_DEBUG_ALFA"); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			Alfa = float32(f)
			os.Unsetenv("NIMBUS_DEBUG_ALFA") // aplica uma vez só
		}
	}
	// Uma vez só: se ficasse a cada quadro, não daria para fechar a aba.
	if abrirConfig && !jaAbriuConfig {
		jaAbriuConfig = true
		configAberto = true
	}

	// Também uma vez só, pelo mesmo motivo: assim dá para marcar a caixinha de
	// volta e ver os controles reaparecerem.
	if semControlesNoInicio && !jaTirouOsControles {
		jaTirouOsControles = true
		mostrarControles = false
	}

	if multiNoInicio && !manterCarregado {
		manterCarregado = true
		player.DefinirModoMultiplo(true)
	}

	if playerAutomatico != "" && !playerJaAberto && telaPronta {
		playerJaAberto = true
		abrirPlayer(playerAutomatico)
	}

	// NIMBUS_DEBUG_FECHAR=1 -> fecha o player sozinho depois de ~8s (para
	// testar o que acontece com a tela ao fechar).
	if os.Getenv("NIMBUS_DEBUG_FECHAR") == "1" && !jaFechou && playerJaAberto {
		quadrosAteFechar++
		if quadrosAteFechar > 480 {
			jaFechou = true
			fecharPlayer()
		}
	}

	if trocarPara != "" && !jaTrocou && playerJaAberto {
		quadrosAteTrocar++
		if quadrosAteTrocar > 360 { // ~6 segundos
			jaTrocou = true
			abrirPlayer(trocarPara)
		}
	}

	if esconderSozinho && !jaEscondeu && telaPronta {
		quadrosAteEsconder++
		if quadrosAteEsconder > 300 { // ~5 segundos a 60 quadros/s
			jaEscondeu = true
			menuAberto = false
		}
	}
}

// preenchido de dentro das janelinhas, para sabermos onde elas estão
var (
	dbgPos, dbgTam imgui.Vec2
	dbgHover       bool

	dbgPlayerPos, dbgPlayerTam imgui.Vec2
)

// espiarPlayer é chamada DENTRO da janelinha do player.
func espiarPlayer() {
	dbgPlayerPos = imgui.WindowPos()
	dbgPlayerTam = imgui.WindowSize()
}

// espiarJanela é chamada DENTRO do layout da janelinha Música.
func espiarJanela() {
	dbgPos = imgui.WindowPos()
	dbgTam = imgui.WindowSize()
	dbgHover = imgui.IsWindowHoveredV(imgui.HoveredFlagsChildWindows)
}

func depurar() {
	if os.Getenv("NIMBUS_DEBUG") != "1" {
		return
	}
	contaQuadros++
	// Registra QUADRO A QUADRO (só os primeiros 400), para ver oscilação.
	if contaQuadros > 400 {
		return
	}

	io := imgui.CurrentIO()
	pos := io.MousePos()

	tela := io.DisplaySize()
	if contaQuadros == 2 {
		// Confere se o tema entrou no estilo PERMANENTE (é o que faz as
		// janelas de encaixe nascerem com a nossa aparência).
		e := imgui.CurrentStyle()
		c := e.Colors()
		fmt.Fprintf(os.Stderr,
			"[tema] fundoDaJanela=(%.2f,%.2f,%.2f) esperado=(%.2f,%.2f,%.2f) | cantoDaJanela=%.0f alfa=%.2f\n",
			c[imgui.ColWindowBg].X, c[imgui.ColWindowBg].Y, c[imgui.ColWindowBg].Z,
			cor4(pal.Fundo).X, cor4(pal.Fundo).Y, cor4(pal.Fundo).Z,
			e.WindowRounding(), e.Alpha())

		// As três cores que pintariam a TELA INTEIRA: têm de estar com alfa 0,
		// senão aparece um véu escuro sobre tudo (veja tema.go).
		fmt.Fprintf(os.Stderr,
			"[tema] alfa das cores de tela cheia: encaixeVazio=%.2f modal=%.2f trocaDeJanela=%.2f (todas devem ser 0)\n",
			c[imgui.ColDockingEmptyBg].W,
			c[imgui.ColModalWindowDimBg].W,
			c[imgui.ColNavWindowingDimBg].W)
	}
	if contaQuadros == 1 {
		vp := imgui.MainViewport()
		vpPos := vp.Pos()
		fmt.Fprintf(os.Stderr,
			"[info] configFlags=%d (viewports=%v docking=%v) viewportPos=(%.0f,%.0f) viewportTam=(%.0f,%.0f)\n",
			io.ConfigFlags(),
			io.ConfigFlags()&imgui.ConfigFlagsViewportsEnable != 0,
			io.ConfigFlags()&imgui.ConfigFlagsDockingEnable != 0,
			vpPos.X, vpPos.Y, vp.Size().X, vp.Size().Y)
	}
	fmt.Fprintf(os.Stderr,
		"[q%03d] display=(%.0f,%.0f) mouse=(%.0f,%.0f) | musica=(%.0f,%.0f) | PLAYER pos=(%.0f,%.0f) tam=(%.0f,%.0f) pedido=(%.0f,%.0f)\n",
		contaQuadros, tela.X, tela.Y,
		pos.X, pos.Y,
		dbgPos.X, dbgPos.Y,
		dbgPlayerPos.X, dbgPlayerPos.Y, dbgPlayerTam.X, dbgPlayerTam.Y,
		basePX+420, basePY+80,
	)
}
