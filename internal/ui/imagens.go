// Carrega as logos dos serviços (YouTube, Netflix...) do disco e entrega para
// o ImGui desenhar.
//
// Como funciona: o ImGui não desenha um arquivo PNG direto — ele precisa de uma
// "textura", que é a imagem já enviada para a placa de vídeo. O giu faz esse
// envio, mas de forma ASSÍNCRONA (a placa só aceita entre quadros). Então:
//
//  1. no começo do programa pedimos o envio de cada imagem;
//  2. quando cada uma fica pronta, guardamos a textura no mapa;
//  3. enquanto não está pronta (ou se o arquivo faltar), o botão desenha a
//     marca em vetor — o desenho de reserva, que nunca deixa o botão vazio.
//
// As imagens são lidas do DISCO (padrão do projeto: assets em arquivo, não
// embutidos), então dá para trocar uma logo sem recompilar.
package ui

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // habilita ler .jpg/.jfif também
	_ "image/png"  // habilita ler .png
	"os"
	"path/filepath"

	g "github.com/AllenDang/giu"
	_ "golang.org/x/image/webp" // muita imagem baixada da web é WebP disfarçada
)

// logoServico é uma logo pronta para desenhar. Guardamos o tamanho original
// junto porque a textura do giu não informa as dimensões, e precisamos delas
// para desenhar a imagem sem distorcer.
type logoServico struct {
	Textura    *g.Texture
	Larg, Alt  float32
}

// texturasServicos guarda as logos já prontas para desenhar, por serviço.
//
// Não precisa de trava: tanto o preenchimento (no retorno do giu) quanto a
// leitura (ao desenhar) acontecem na mesma thread, a da interface.
var texturasServicos = map[string]*logoServico{}

var jaPedimosAsImagens bool

// carregarImagensDosServicos pede o envio de todas as logos. Chamar uma vez,
// ao iniciar.
func carregarImagensDosServicos() {
	if jaPedimosAsImagens {
		return
	}
	jaPedimosAsImagens = true

	for _, s := range servicos {
		qual := s.Qual
		imagem, err := lerImagemDoDisco(qual + ".png")
		if err != nil {
			// Sem arquivo: o botão usa o desenho em vetor. Não é problema.
			continue
		}

		// Guardamos o tamanho ANTES de enviar para a placa de vídeo, porque
		// depois a textura não sabe mais informar as dimensões.
		area := imagem.Bounds()
		larg, alt := float32(area.Dx()), float32(area.Dy())

		g.EnqueueNewTextureFromRgba(imagem, func(t *g.Texture) {
			texturasServicos[qual] = &logoServico{Textura: t, Larg: larg, Alt: alt}
		})
	}
}

// lerImagemDoDisco procura o arquivo nas pastas possíveis e o decodifica.
func lerImagemDoDisco(nome string) (*image.RGBA, error) {
	for _, caminho := range caminhosPossiveis(nome) {
		arquivo, err := os.Open(caminho)
		if err != nil {
			continue
		}
		imagem, _, err := image.Decode(arquivo)
		arquivo.Close()
		if err != nil {
			continue
		}
		return paraRGBA(imagem), nil
	}
	return nil, fmt.Errorf("nao achei a imagem %s", nome)
}

// caminhosPossiveis lista onde procurar: ao lado do .exe (que fica em build/,
// então subimos um nível) e na pasta atual.
func caminhosPossiveis(nome string) []string {
	relativo := filepath.Join("assets", "servicos", nome)
	lista := []string{relativo}

	if exe, err := os.Executable(); err == nil {
		pasta := filepath.Dir(exe)
		lista = append(lista,
			filepath.Join(pasta, relativo),
			filepath.Join(pasta, "..", relativo),
		)
	}
	return lista
}

// paraRGBA garante o formato que a placa de vídeo espera (4 bytes por pixel).
func paraRGBA(origem image.Image) *image.RGBA {
	if pronta, ok := origem.(*image.RGBA); ok {
		return pronta
	}
	area := origem.Bounds()
	destino := image.NewRGBA(image.Rect(0, 0, area.Dx(), area.Dy()))
	draw.Draw(destino, destino.Bounds(), origem, area.Min, draw.Src)
	return destino
}
