// A tabela de teclas: como o Windows chama cada tecla (um número) e como a
// gente escreve isso para uma pessoa ler ("Alt+1").
//
// Por que uma tabela escrita à mão, e não algo automático: o Windows não tem
// uma função que devolva o nome da tecla de forma confiável para todo teclado
// (o mesmo botão muda de nome conforme o idioma). Uma tabela curta, com as
// teclas que alguém realmente usaria num atalho, é mais previsível — e é o que
// aparece no arquivo de configuração, que tem de ficar legível.
package atalhos

import (
	"sort"
	"strings"
)

// Mods são as teclas que ficam SEGURADAS junto com a tecla do atalho.
//
// É um "mapa de bits": cada uma vale um número que é potência de dois, então dá
// para somar as quatro num só valor e depois perguntar por cada uma. Fica bem
// mais simples do que guardar uma lista.
type Mods uint8

const (
	ModCtrl Mods = 1 << iota
	ModAlt
	ModShift
	ModWin
)

// Números do Windows para cada modificador ("virtual-key codes").
//
// ⚠️ Usamos os códigos GENÉRICOS (0x11, 0x12, 0x10), que respondem tanto ao
// esquerdo quanto ao direito. É o que uma pessoa espera: quem configurou
// "Alt+1" quer que funcione com qualquer Alt, não só o do lado que estava
// usando na hora de gravar. (A tecla Windows não tem código genérico, então
// para ela conferimos os dois lados.)
const (
	vkCtrl    = 0x11
	vkAlt     = 0x12
	vkShift   = 0x10
	vkWinEsq  = 0x5B
	vkWinDir  = 0x5C
	vkCtrlEsq = 0xA2
	vkCtrlDir = 0xA3
	vkAltEsq  = 0xA4
	vkAltDir  = 0xA5
	vkShiftE  = 0xA0
	vkShiftD  = 0xA1
)

// Texto escreve o modificador do jeito que aparece na tela: "Ctrl+Alt".
// A ordem é sempre a mesma (Ctrl, Alt, Shift, Win) para o mesmo atalho nunca
// aparecer escrito de dois jeitos diferentes.
func (m Mods) Texto() string {
	partes := make([]string, 0, 4)
	if m&ModCtrl != 0 {
		partes = append(partes, "Ctrl")
	}
	if m&ModAlt != 0 {
		partes = append(partes, "Alt")
	}
	if m&ModShift != 0 {
		partes = append(partes, "Shift")
	}
	if m&ModWin != 0 {
		partes = append(partes, "Win")
	}
	return strings.Join(partes, "+")
}

// modPorNome é usado ao LER o arquivo de configuração.
var modPorNome = map[string]Mods{
	"ctrl":  ModCtrl,
	"alt":   ModAlt,
	"shift": ModShift,
	"win":   ModWin,
}

// nomeDaTecla: o número do Windows -> o nome que a gente mostra.
//
// Só entram teclas que fazem sentido num atalho. Ficam DE FORA de propósito:
//
//   - os próprios modificadores (Ctrl, Alt, Shift, Win): eles seguram o atalho,
//     não disparam;
//   - os botões esquerdo, direito e do meio do mouse: virar atalho tomaria o
//     clique normal do PC inteiro. Os botões EXTRAS do mouse (o 4 e o 5, os do
//     lado do polegar) entram, porque ninguém depende deles para clicar.
var nomeDaTecla = montarNomes()

func montarNomes() map[uint16]string {
	n := map[uint16]string{
		0x08: "Backspace",
		0x09: "Tab",
		0x0D: "Enter",
		0x1B: "Esc",
		0x20: "Espaco",
		0x21: "PageUp",
		0x22: "PageDown",
		0x23: "End",
		0x24: "Home",
		0x25: "Esquerda",
		0x26: "Cima",
		0x27: "Direita",
		0x28: "Baixo",
		0x2D: "Insert",
		0x2E: "Delete",
		0xBD: "Menos",
		0xBB: "Mais",
		0xC0: "Til",
		0xBC: "Virgula",
		0xBE: "Ponto",

		// Os botões extras do mouse (perto do polegar). Ficam aqui porque o
		// dono do projeto pediu para poder usar o mouse no atalho.
		0x05: "Mouse4",
		0x06: "Mouse5",
	}
	// Números de cima do teclado: 0x30 é o "0", 0x31 o "1"...
	for i := 0; i <= 9; i++ {
		n[uint16(0x30+i)] = string(rune('0' + i))
	}
	// Letras: 0x41 é o "A".
	for i := 0; i < 26; i++ {
		n[uint16(0x41+i)] = string(rune('A' + i))
	}
	// Teclado numérico (o bloco da direita).
	for i := 0; i <= 9; i++ {
		n[uint16(0x60+i)] = "Num" + string(rune('0'+i))
	}
	// F1 até F12.
	for i := 1; i <= 12; i++ {
		nome := "F" + itoa(i)
		n[uint16(0x70+i-1)] = nome
	}
	return n
}

// teclaPorNome é o caminho inverso, usado ao LER o arquivo.
var teclaPorNome = inverter(nomeDaTecla)

func inverter(de map[uint16]string) map[string]uint16 {
	para := make(map[string]uint16, len(de))
	for codigo, nome := range de {
		para[strings.ToLower(nome)] = codigo
	}
	return para
}

// TeclasConhecidas devolve os nomes de todas as teclas que podem disparar um
// atalho, em ordem. Existe para a documentação e para os testes.
func TeclasConhecidas() []string {
	nomes := make([]string, 0, len(nomeDaTecla))
	for _, nome := range nomeDaTecla {
		nomes = append(nomes, nome)
	}
	sort.Strings(nomes)
	return nomes
}

// itoa converte um número pequeno em texto sem puxar o strconv (a tabela é
// montada uma vez só, no início do programa).
func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
