// Guardar os atalhos em disco, para não se perderem ao fechar o Nimbus.
//
// O arquivo é TEXTO, uma linha por ação, de propósito: dá para abrir no
// Bloco de Notas e entender (ou consertar) sem programa nenhum. Um formato
// binário economizaria uns bytes e custaria toda essa clareza.
package atalhos

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const nomeDoArquivo = "atalhos.txt"

// Arquivo é o caminho completo onde os atalhos ficam guardados:
// `%LOCALAPPDATA%\Nimbus\atalhos.txt`.
//
// É a mesma pasta que o atualizador de listas usa, e pelo mesmo motivo: a pasta
// do programa pode estar em "Arquivos de Programas", onde escrever exige
// permissão de administrador — o Nimbus não vai pedir isso para salvar um
// atalho.
func Arquivo() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		return ""
	}
	return filepath.Join(base, "Nimbus", nomeDoArquivo)
}

// Carregar lê o arquivo e aplica o que estiver lá em cima das combinações de
// fábrica. Chamar UMA vez, ao iniciar, depois de Registrar.
//
// Devolver erro aqui é normal e não é problema: na primeira vez que o Nimbus
// roda num PC o arquivo não existe, e o programa segue com os atalhos de
// fábrica. Quem chama só usa o erro para depuração.
func Carregar() error {
	caminho := Arquivo()
	if caminho == "" {
		return fmt.Errorf("nao descobri a pasta de dados do usuario")
	}
	arquivo, err := os.Open(caminho)
	if err != nil {
		return err
	}
	defer arquivo.Close()

	// O arquivo MANDA: se ele existe, ele diz a configuração inteira —
	// inclusive "esta ação está sem atalho" (a linha simplesmente não está
	// lá). Sem limpar antes, um atalho de fábrica que a pessoa apagou
	// voltaria a cada vez que o Nimbus abrisse.
	amarras = make(map[string]Atalho, len(acoes))

	conhecidas := map[string]bool{}
	for _, a := range acoes {
		conhecidas[a.Nome] = true
	}

	leitor := bufio.NewScanner(arquivo)
	for leitor.Scan() {
		linha := strings.TrimSpace(leitor.Text())
		if linha == "" || strings.HasPrefix(linha, "#") {
			continue
		}
		nome, combinacao, achou := strings.Cut(linha, "=")
		if !achou {
			continue // linha torta: ignora em silêncio, não derruba o resto
		}
		nome = strings.TrimSpace(strings.ToLower(nome))
		if !conhecidas[nome] {
			// Pode ser um serviço que existia numa versão antiga. Ignorar é
			// melhor do que reclamar de algo que a pessoa não fez.
			continue
		}
		m, tecla, err := Analisar(combinacao)
		if err != nil {
			continue
		}
		Definir(nome, m, tecla) // erro aqui já foi coberto pelo Analisar
	}

	// Acabamos de LER o arquivo, então ele está em dia: se não baixássemos a
	// bandeirinha, a interface salvaria de volta um arquivo idêntico no
	// primeiro quadro (o Definir acima a levanta a cada linha).
	MarcarSalvo()
	return leitor.Err()
}

// Salvar grava o arquivo. Chamar sempre que a pessoa mudar um atalho.
//
// Grava primeiro num arquivo temporário e só então renomeia por cima do
// definitivo: se faltar energia no meio da escrita, o arquivo antigo continua
// inteiro em vez de ficar cortado (e um arquivo cortado faria o Nimbus abrir
// com metade dos atalhos, o que é bem mais confuso do que abrir com os
// anteriores).
func Salvar() error {
	caminho := Arquivo()
	if caminho == "" {
		return fmt.Errorf("nao descobri a pasta de dados do usuario")
	}
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		return err
	}

	var texto strings.Builder
	texto.WriteString("# Atalhos do Nimbus.\n")
	texto.WriteString("# O programa reescreve este arquivo quando voce muda um atalho na aba Atalhos.\n")
	texto.WriteString("# Formato: acao = Ctrl+Alt+Tecla   (acao sem linha aqui = sem atalho)\n\n")
	// Na ordem da tela, para o arquivo ficar igual ao que a pessoa vê.
	for _, acao := range acoes {
		a, tem := amarras[acao.Nome]
		if !tem {
			continue
		}
		texto.WriteString(acao.Nome)
		texto.WriteString(" = ")
		texto.WriteString(a.Texto())
		texto.WriteString("\n")
	}

	temporario := caminho + ".novo"
	if err := os.WriteFile(temporario, []byte(texto.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(temporario, caminho)
}

// Analisar entende um texto como "Ctrl+Alt+1" e devolve as partes.
//
// É uma função pura (não olha nem mexe em nada de fora), então dá para testar
// direito — e é ela que protege o programa de um arquivo editado à mão com
// erro de digitação.
func Analisar(texto string) (Mods, uint16, error) {
	var m Mods
	var tecla uint16

	partes := strings.Split(strings.TrimSpace(texto), "+")
	for _, parte := range partes {
		parte = strings.TrimSpace(strings.ToLower(parte))
		if parte == "" {
			continue
		}
		if bit, ehModificador := modPorNome[parte]; ehModificador {
			m |= bit
			continue
		}
		codigo, conhecida := teclaPorNome[parte]
		if !conhecida {
			return 0, 0, fmt.Errorf("nao conheco a tecla %q", parte)
		}
		if tecla != 0 {
			return 0, 0, fmt.Errorf("mais de uma tecla no mesmo atalho")
		}
		tecla = codigo
	}

	if tecla == 0 {
		return 0, 0, fmt.Errorf("falta a tecla")
	}
	if m == 0 {
		return 0, 0, fmt.Errorf("falta o Ctrl, Alt, Shift ou Win")
	}
	return m, tecla, nil
}
