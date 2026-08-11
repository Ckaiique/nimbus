// converter-logo: transforma uma imagem qualquer (WebP, JPG, JFIF, PNG) na
// logo PNG que o Nimbus lê para o botão de um serviço.
//
// ─── POR QUE ISTO EXISTE ──────────────────────────────────────────────────
//
// Logo baixada da internet quase nunca vem em PNG: hoje o normal é **WebP**
// (às vezes até sem extensão no nome). O carregador do Nimbus até decodifica
// WebP, mas ele procura o arquivo pelo nome `assets/servicos/<serviço>.png` —
// então um WebP renomeado para ".png" funcionaria, e isso é uma mentira no
// nome do arquivo: quem abrisse a pasta depois não entenderia nada.
//
// Esta ferramenta converte de verdade. Ela usa o MESMO decodificador que o
// Nimbus usa por dentro (`golang.org/x/image/webp`), então: se a imagem
// aparecer certa aqui, ela vai aparecer certa no botão.
//
// ⚠️ Não use o "Salvar como" do Paint nem conversor de site para isto. O
// decodificador de WebP do próprio Windows falha com WebP transparente: já
// aconteceu de ele entregar a imagem **toda preta** (foi o que motivou esta
// ferramenta).
//
// ─── COMO USAR ────────────────────────────────────────────────────────────
//
// A partir da pasta do projeto:
//
//	go run ./ferramentas/converter-logo <arquivo-de-entrada> <nome-do-serviço>
//
// Exemplo real (a logo do WhatsApp, baixada em WebP e sem extensão):
//
//	go run ./ferramentas/converter-logo ../img/whats whatsapp
//
// Isso grava `assets/servicos/whatsapp.png`. O nome do serviço tem de ser
// **igual** ao campo `Qual` da lista `servicos` (`internal/ui/overlay.go`) —
// é assim que o botão acha a imagem dele.
//
// O Nimbus lê as logos do DISCO, então basta reabrir o programa: não precisa
// recompilar para trocar uma figura.
package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	// Os três decodificadores que o Nimbus também habilita. Ficam com "_"
	// porque não chamamos nada deles: só de serem importados, eles se
	// registram no `image.Decode`, que descobre o formato pelo CONTEÚDO do
	// arquivo (e não pela extensão do nome — que aqui pode nem existir).
	_ "image/jpeg"

	_ "golang.org/x/image/webp"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("uso: go run ./ferramentas/converter-logo <arquivo> <nome-do-servico>")
		fmt.Println("exemplo: go run ./ferramentas/converter-logo ../img/whats whatsapp")
		os.Exit(2)
	}
	entrada, servico := os.Args[1], os.Args[2]

	imagem, formato, err := abrir(entrada)
	if err != nil {
		fmt.Println("nao consegui ler a imagem:", err)
		os.Exit(1)
	}

	area := imagem.Bounds()
	fmt.Printf("lido: %s (%s), %dx%d pixels\n", entrada, formato, area.Dx(), area.Dy())

	// Aviso e não erro: uma logo com fundo branco/preto ainda serve, só fica
	// com um quadrado em volta dentro do botão. Quem manda é o dono.
	if !temTransparencia(imagem) {
		fmt.Println("AVISO: esta imagem nao tem fundo transparente — no botao vai")
		fmt.Println("       aparecer o quadrado de fundo dela em volta da logo.")
	}

	destino := filepath.Join("assets", "servicos", servico+".png")
	if err := gravarPNG(destino, imagem); err != nil {
		fmt.Println("nao consegui gravar:", err)
		os.Exit(1)
	}
	fmt.Println("gravado:", destino)
}

func abrir(caminho string) (image.Image, string, error) {
	arquivo, err := os.Open(caminho)
	if err != nil {
		return nil, "", err
	}
	defer arquivo.Close()

	imagem, formato, err := image.Decode(arquivo)
	if err != nil {
		// Mensagem mais útil que o "unknown format" cru da biblioteca.
		if strings.Contains(err.Error(), "unknown format") {
			return nil, "", fmt.Errorf(
				"formato nao reconhecido (aceito: png, jpg/jfif, webp)")
		}
		return nil, "", err
	}
	return imagem, formato, nil
}

// gravarPNG escreve o arquivo só depois de a conversão dar certo.
//
// Grava primeiro num arquivo temporário e só então o renomeia por cima do
// definitivo: se algo falhar no meio, a logo que já estava lá continua
// inteira, em vez de virar um arquivo cortado que o Nimbus não conseguiria
// abrir (e o botão perderia a imagem sem motivo aparente).
func gravarPNG(destino string, imagem image.Image) error {
	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		return err
	}

	temporario := destino + ".novo"
	arquivo, err := os.Create(temporario)
	if err != nil {
		return err
	}

	err = png.Encode(arquivo, imagem)
	if erroAoFechar := arquivo.Close(); err == nil {
		err = erroAoFechar
	}
	if err != nil {
		os.Remove(temporario)
		return err
	}

	return os.Rename(temporario, destino)
}

// temTransparencia diz se a imagem tem algum pixel vazado.
//
// Serve só para o aviso na tela. Olha o canal alfa de cada pixel: se todos
// estão em "opaco", a imagem tem fundo pintado.
func temTransparencia(imagem image.Image) bool {
	area := imagem.Bounds()
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if _, _, _, alfa := imagem.At(x, y).RGBA(); alfa < 0xffff {
				return true
			}
		}
	}
	return false
}
