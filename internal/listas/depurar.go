// Depuração da atualização de listas (não afeta o uso normal).
//
// O Nimbus é compilado como programa de janela (`-H=windowsgui`), então ele não
// tem terminal para onde escrever: mensagem impressa some no vazio. Por isso o
// projeto usa o mesmo recurso em toda parte — uma variável de ambiente com o
// caminho de um arquivo.
//
//	NIMBUS_DEBUG_LISTAS=C:\temp\listas.txt
//
// Sem a variável, nada é gravado e nenhum arquivo é aberto.
package listas

import (
	"fmt"
	"os"
	"time"
)

var arquivoDeDepuracao = os.Getenv("NIMBUS_DEBUG_LISTAS")

// anotar registra uma linha no arquivo de depuração, se houver um.
//
// Ela nunca devolve erro nem interrompe nada: depuração que atrapalha o
// programa é pior que depuração nenhuma.
func anotar(formato string, args ...any) {
	if arquivoDeDepuracao == "" {
		return
	}
	f, err := os.OpenFile(arquivoDeDepuracao, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s  %s\n",
		time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(formato, args...))
}
