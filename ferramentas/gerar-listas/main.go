// gerar-listas: a ferramenta de manutenção que atualiza, DENTRO DO PROJETO, a
// lista de domínios de anúncio que vai embutida no Nimbus.
//
// ─── QUANDO ISTO RODA ─────────────────────────────────────────────────────
//
// Só na mão, por quem está mexendo no projeto, antes de gerar uma versão nova
// do programa. O Nimbus em si nunca chama esta ferramenta.
//
// (Não confunda com o atualizador automático, `internal/listas`: aquele roda
// dentro do programa, no PC do dono, e guarda a lista nova na pasta de dados
// dele. Esta aqui atualiza o arquivo que vai DENTRO do .exe — a base que vale
// no primeiro uso e quando não há internet. Os dois usam exatamente o mesmo
// código de conversão, que mora em `internal/adblock`.)
//
// Como usar, a partir da pasta do projeto:
//
//	go run ./ferramentas/gerar-listas
//
// Ele baixa as listas públicas da EasyList, joga fora tudo que o nosso filtro
// não sabe aplicar e reescreve `internal/adblock/dados/easylist-dominios.txt`.
// Depois é só recompilar (`compilar.bat`).
//
// ─── LICENÇA (leia antes de mexer) ────────────────────────────────────────
//
// O código do Nimbus é MIT, mas as listas da EasyList são CC BY-SA 3.0 / GPLv3
// e continuam sendo delas. O arquivo gerado leva o aviso no cabeçalho e a
// explicação completa está em `docs/LICENCA-LISTAS.md`. Ao trocar de lista,
// atualize os dois lugares.
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"nimbus/internal/adblock"
)

// destino é onde o arquivo gerado é gravado, contado a partir da pasta do
// projeto. É o mesmo caminho que o `go:embed` do pacote adblock lê.
const destino = "internal/adblock/dados/easylist-dominios.txt"

func main() {
	if err := gerar(); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func gerar() error {
	// A ferramenta tem de ser rodada da pasta do projeto. Conferimos isso
	// ANTES de sair baixando megabytes: melhor falhar em um segundo do que
	// gravar arquivo no lugar errado.
	if _, err := os.Stat("go.mod"); err != nil {
		return fmt.Errorf("rode a partir da pasta do projeto (onde está o go.mod): %w", err)
	}

	var baixadas []adblock.ListaBaixada
	for _, f := range adblock.Fontes {
		fmt.Printf("baixando %s ...\n", f.Nome)
		texto, err := baixar(f.URL)
		if err != nil {
			return fmt.Errorf("baixar %s (%s): %w", f.Nome, f.URL, err)
		}
		fmt.Printf("  %d KB\n", len(texto)/1024)
		baixadas = append(baixadas, adblock.ListaBaixada{Fonte: f, Texto: texto})
	}

	conteudo, res := adblock.MontarArquivo(baixadas, time.Now())

	fmt.Printf("\n%6d dominios distintos vindos das listas\n", res.Brutos)
	fmt.Printf("-%5d por excecao da propria EasyList\n", res.PorExcecao)
	fmt.Printf("-%5d por estarem sob um dominio protegido do Nimbus\n", res.PorProtecao)
	fmt.Printf("-%5d por redundancia (o dominio de cima ja bloqueia)\n", res.Podados)
	fmt.Printf("=%5d dominios no arquivo, mais %d excecoes\n\n", res.Final, res.Excecoes)

	if res.Final < 10000 {
		return fmt.Errorf("só saíram %d domínios — isso é pouco demais para ser verdade; "+
			"o formato das listas pode ter mudado. O arquivo NÃO foi gravado", res.Final)
	}

	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		return fmt.Errorf("criar a pasta de destino: %w", err)
	}
	if err := os.WriteFile(destino, conteudo, 0o644); err != nil {
		return fmt.Errorf("gravar %s: %w", destino, err)
	}

	fmt.Printf("gravado em %s (%d KB)\n", destino, len(conteudo)/1024)
	fmt.Println("agora recompile o Nimbus para o arquivo novo entrar no .exe.")
	return nil
}

// baixar pega o texto de uma lista pública.
//
// Tempo limite generoso (2 minutos): as listas passam de 3 MB e o servidor às
// vezes está lento. Sem limite nenhum, um servidor que não responde deixaria a
// ferramenta pendurada para sempre.
func baixar(url string) (string, error) {
	cliente := &http.Client{Timeout: 2 * time.Minute}
	resp, err := cliente.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("o servidor respondeu %s", resp.Status)
	}
	corpo, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	// Uma resposta sem cabeçalho de lista, ou que não rende domínio nenhum,
	// quase sempre é página de erro — não lista.
	if !adblock.ParecemListasDeVerdade(string(corpo), adblock.MinimoPorLista) {
		return "", fmt.Errorf("a resposta não parece uma lista da EasyList (%d bytes)", len(corpo))
	}
	return string(corpo), nil
}
