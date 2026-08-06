// Ler as listas públicas da EasyList e separar o que o nosso filtro sabe
// aplicar do que ele não sabe.
//
// ─── POR QUE ISTO MORA AQUI (e não junto do gerador) ──────────────────────
//
// Duas partes do Nimbus precisam entender o formato da EasyList:
//
//  1. o gerador `ferramentas/gerar-listas`, rodado à mão pelo dono;
//  2. o atualizador automático `internal/listas`, que roda em segundo plano.
//
// Se cada um tivesse a sua cópia, um dia elas divergiriam e o arquivo gerado à
// mão passaria a ser diferente do baixado sozinho — o tipo de diferença que
// ninguém percebe até o dia em que um site quebra. Então a leitura mora num
// lugar só, aqui, que é o pacote que entende de "isto é anúncio".
//
// Tudo neste arquivo é FUNÇÃO PURA: entra texto, sai lista. Nada de internet,
// nada de disco. É o que permite testar sem baixar nada.
package adblock

import (
	"bufio"
	"strings"
)

// Extraidas é o resultado de ler uma lista no formato EasyList.
type Extraidas struct {
	// Bloquear são os domínios das regras "||dominio.com^".
	Bloquear []string
	// Excecoes são os domínios das regras "@@||dominio.com^" — a própria
	// EasyList dizendo "este aqui, apesar de parecer, não é anúncio".
	Excecoes []string
	// Ignoradas conta as linhas que eram regra mas o nosso filtro não sabe
	// aplicar (cosmética, por caminho, com "$domain=..."). Serve para o
	// relatório ser honesto sobre o quanto ficou de fora.
	Ignoradas int
}

// opcoesQueMantemOSentido são as opções (o que vem depois do "$" na regra) que
// NÃO mudam o alvo do bloqueio — só dizem "que tipo de coisa" bloquear.
//
// Por que essa distinção é o coração da peneira: o nosso filtro é grosso, ele
// bloqueia o DOMÍNIO inteiro. Se a regra disser "bloqueie este domínio quando
// vier como imagem", nós bloqueamos sempre — fica mais largo que o original,
// mas o alvo continua sendo o mesmo servidor de anúncio.
//
// Já "$domain=globo.com" diz "bloqueie isto, mas SÓ dentro do globo.com".
// Aplicar como bloqueio geral seria inventar uma regra que ninguém escreveu — e
// é exatamente daí que sai o falso positivo que quebra uma página inocente. Por
// isso opção assim faz a regra inteira ser descartada.
var opcoesQueMantemOSentido = map[string]bool{
	"third-party":       true,
	"3p":                true,
	"all":               true,
	"popup":             true,
	"document":          true,
	"doc":               true,
	"subdocument":       true,
	"frame":             true,
	"script":            true,
	"image":             true,
	"img":               true,
	"stylesheet":        true,
	"css":               true,
	"object":            true,
	"object-subrequest": true,
	"xmlhttprequest":    true,
	"xhr":               true,
	"media":             true,
	"font":              true,
	"websocket":         true,
	"ping":              true,
	"beacon":            true,
	"other":             true,
	"important":         true,
}

// LerRegrasEasyList percorre o texto de uma lista no formato EasyList e devolve
// só o que o nosso filtro sabe aplicar.
//
// O que ENTRA: regra de domínio inteiro — "||dominio.com^", com ou sem opções
// depois do "$" — e a versão de exceção, "@@||dominio.com^".
//
// O que é IGNORADO (e por quê):
//
//   - regra cosmética ("##.banner", "#@#", "#?#"): esconde um pedaço da página
//     com CSS, não tem endereço nenhum para bloquear;
//   - regra por caminho ("||site.com/anuncios/*"): bloquear o domínio inteiro
//     por causa dela derrubaria o site;
//   - expressão regular ("/banner\d+/"): não sabemos avaliar;
//   - regra com opção que restringe o alcance ("$domain=", "$denyallow=") ou
//     que transforma o pedido em vez de barrar ("$redirect=", "$removeparam=").
//
// Linha em branco, comentário ("!") e cabeçalho ("[Adblock Plus 2.0]") também
// saem, mas não contam como ignoradas: nunca chegaram a ser regra.
func LerRegrasEasyList(texto string) Extraidas {
	var r Extraidas
	sc := bufio.NewScanner(strings.NewReader(texto))
	// Linha de lista de filtro pode ser comprida (regras com muitas opções). O
	// limite padrão do Scanner é 64 KB; ampliamos para a leitura não parar no
	// meio da lista por causa de uma linha grande.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		linha := strings.TrimSpace(sc.Text())

		if linha == "" || strings.HasPrefix(linha, "!") || strings.HasPrefix(linha, "[") {
			continue
		}

		dominio, excecao, ok := UmaRegraEasyList(linha)
		if !ok {
			r.Ignoradas++
			continue
		}
		if excecao {
			r.Excecoes = append(r.Excecoes, dominio)
		} else {
			r.Bloquear = append(r.Bloquear, dominio)
		}
	}
	return r
}

// UmaRegraEasyList analisa UMA linha e responde: qual domínio ela trata, se é
// exceção, e se dá para aproveitá-la.
//
// ok=false quer dizer "esta regra existe, mas o nosso filtro não sabe aplicá-la"
// — o que é o caso da maioria absoluta das linhas da EasyList, e está tudo bem.
func UmaRegraEasyList(linha string) (dominio string, excecao bool, ok bool) {
	// 1. Regra cosmética: esconde elemento da página com CSS ("##", "#@#",
	//    "#?#", "#$#"...). Todas têm "#" no meio, e um endereço de verdade
	//    nunca tem — então procurar "#" já separa as duas famílias.
	if strings.Contains(linha, "#") {
		return "", false, false
	}

	// 2. Exceção: "@@" na frente quer dizer "não bloqueie isto".
	if strings.HasPrefix(linha, "@@") {
		excecao = true
		linha = linha[2:]
	}

	// 3. Expressão regular: começa com "/". Não sabemos avaliar.
	if strings.HasPrefix(linha, "/") {
		return "", false, false
	}

	// 4. Só aproveitamos a âncora de domínio "||". Sem ela, a regra casa por
	//    pedaço de endereço — que é justamente o "contém o texto" que este
	//    projeto se recusa a fazer (veja o comentário de casamentoMaisEspecifico).
	if !strings.HasPrefix(linha, "||") {
		return "", false, false
	}
	resto := linha[2:]

	// 5. Separar o domínio das opções: o "$" abre as opções e o "^" é o
	//    "acaba aqui" do formato EasyList.
	opcoes := ""
	if i := strings.Index(resto, "$"); i >= 0 {
		opcoes = resto[i+1:]
		resto = resto[:i]
	}
	resto = strings.TrimSuffix(resto, "^")

	// 6. Depois de tirar "^" e opções, tem de ter sobrado SÓ o domínio. Se
	//    ainda houver "/" (caminho), "*" (curinga) ou "^" no meio, a regra é
	//    mais fina do que sabemos aplicar — e aplicá-la ao domínio inteiro
	//    bloquearia coisa demais.
	if strings.ContainsAny(resto, "/*^|") {
		return "", false, false
	}

	if !opcoesAceitas(opcoes) {
		return "", false, false
	}

	d := DominioValido(resto)
	if d == "" {
		return "", false, false
	}
	return d, excecao, true
}

// opcoesAceitas diz se TODAS as opções da regra são do tipo que não muda o alvo.
// Basta uma opção desconhecida ou restritiva para recusarmos a regra inteira —
// na dúvida, deixa passar (a regra da casa deste bloqueador).
func opcoesAceitas(opcoes string) bool {
	if opcoes == "" {
		return true
	}
	for _, op := range strings.Split(opcoes, ",") {
		op = strings.TrimSpace(strings.ToLower(op))
		if op == "" {
			return false
		}
		// "~script" quer dizer "tudo MENOS script": inverte o sentido. Fora.
		if strings.HasPrefix(op, "~") {
			return false
		}
		// Opção com valor ("domain=...", "redirect=...") restringe ou
		// transforma. Nenhuma das que aceitamos tem valor.
		if strings.Contains(op, "=") {
			return false
		}
		if !opcoesQueMantemOSentido[op] {
			return false
		}
	}
	return true
}

// DominioValido confere se o texto parece mesmo um nome de domínio e o devolve
// em minúsculas — ou string vazia se não parecer.
//
// A conferência é rígida de propósito: uma linha estranha da lista pública não
// pode virar uma entrada torta na nossa. Entrada torta ou nunca casa com nada
// (peso morto na memória) ou casa com o que não devia.
func DominioValido(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, ".")
	if s == "" || len(s) > 253 {
		return ""
	}
	// Tem de ter pelo menos um ponto: "com" sozinho bloquearia a internet
	// inteira, já que o casamento é por rótulo.
	if !strings.Contains(s, ".") {
		return ""
	}
	for _, rotulo := range strings.Split(s, ".") {
		if rotulo == "" || len(rotulo) > 63 {
			return ""
		}
		if strings.HasPrefix(rotulo, "-") || strings.HasSuffix(rotulo, "-") {
			return ""
		}
		for _, c := range rotulo {
			ehLetra := c >= 'a' && c <= 'z'
			ehNumero := c >= '0' && c <= '9'
			if !ehLetra && !ehNumero && c != '-' && c != '_' {
				return ""
			}
		}
	}
	return s
}

// MinimoPorLista é o mínimo de domínios que UMA lista baixada precisa render
// para ser levada a sério.
//
// O número é baixo (20) porque as listas regionais são pequenas: a EasyList
// Portuguese, por exemplo, tem só algumas centenas de regras, e a maioria é
// cosmética. Ele não serve para julgar qualidade — serve para separar "uma
// lista de verdade" de "uma página de erro em HTML" ou "um download cortado no
// meio", que rendem ZERO domínios. A conferência do resultado final (que tem de
// passar de dezenas de milhares) é feita por quem chama.
const MinimoPorLista = 20

// ParecemListasDeVerdade é a conferência de sanidade do que foi baixado.
//
// Existe por um perigo concreto: quando um servidor está com problema, ele não
// devolve "erro" — devolve uma PÁGINA de erro em HTML, com código 200. Se a
// gente aceitasse isso, o Nimbus trocaria a lista boa por lixo e ficaria sem
// bloquear nada, sem ninguém entender por quê.
//
// Duas conferências, e as duas precisam passar:
//
//  1. o texto tem o cabeçalho do formato ("[Adblock Plus" ou "! Title:");
//  2. dá para extrair pelo menos `minimo` domínios — o que uma página de erro,
//     um arquivo vazio ou um download cortado no meio nunca conseguem.
func ParecemListasDeVerdade(texto string, minimo int) bool {
	inicio := texto
	if len(inicio) > 4096 {
		inicio = inicio[:4096]
	}
	temCabecalho := strings.Contains(inicio, "[Adblock") ||
		strings.Contains(inicio, "! Title:") ||
		strings.Contains(inicio, "!Title:")
	if !temCabecalho {
		return false
	}
	return len(LerRegrasEasyList(texto).Bloquear) >= minimo
}
