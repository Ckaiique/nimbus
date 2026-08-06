// O NOSSO formato de arquivo de listas: como as regras da EasyList viram um
// arquivo simples, e como esse arquivo é lido de volta.
//
// ─── Por que não guardamos as listas da EasyList como elas vêm ────────────
//
// As três listas originais somam vários megabytes de regras — e o nosso filtro
// só sabe aplicar uma fatia delas (bloqueio por domínio). Guardar o resto seria
// carregar peso morto para dentro do .exe e reprocessá-lo a cada abertura.
//
// Então convertemos uma vez e guardamos só o resultado: um domínio por linha.
// O formato é o mais bobo possível de propósito —
//
//	# comentário
//	dominio-de-anuncio.com
//	@dominio-que-nao-deve-ser-bloqueado.com
//
// — assim o leitor cabe em vinte linhas, o arquivo abre no Bloco de Notas e dá
// para entender o que está lá dentro sem ser programador.
package adblock

import (
	"bufio"
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Fonte é uma lista pública que o Nimbus consome.
type Fonte struct {
	// Nome é como ela aparece no cabeçalho do arquivo gerado.
	Nome string
	// URL é o endereço oficial. Veja docs/LICENCA-LISTAS.md.
	URL string
	// ParaQueE explica, em uma linha, por que ela entrou.
	ParaQueE string
}

// Fontes são as listas oficiais da EasyList que entram na nossa.
//
// Os endereços vieram da página oficial https://easylist.to/ (seção
// "Subscriptions"). São os mesmos que o uBlock Origin e o Adblock Plus usam.
//
// Por que estas três:
//
//   - EasyList: a lista principal de PUBLICIDADE, base de quase todo bloqueador
//     do mundo;
//   - EasyPrivacy: a lista de RASTREADORES (quem mede e segue o usuário). É
//     separada da primeira porque nem todo mundo quer as duas coisas — nós
//     queremos;
//   - EasyList Portuguese: o complemento para sites em PORTUGUÊS, que a lista
//     principal (feita de olho no mundo inteiro) não cobre bem. O dono do PC é
//     brasileiro, então esta é a que faz diferença no dia a dia dele.
//
// ⚠️ Ao mexer aqui, atualize também `docs/LICENCA-LISTAS.md`: cada lista tem a
// licença dela e a atribuição é obrigatória.
var Fontes = []Fonte{
	{
		Nome:     "EasyList",
		URL:      "https://easylist.to/easylist/easylist.txt",
		ParaQueE: "publicidade em geral (a lista principal)",
	},
	{
		Nome:     "EasyPrivacy",
		URL:      "https://easylist.to/easylist/easyprivacy.txt",
		ParaQueE: "rastreadores: medicao de audiencia e perfis de usuario",
	},
	{
		Nome:     "EasyList Portuguese",
		URL:      "https://easylist-downloads.adblockplus.org/easylistportuguese.txt",
		ParaQueE: "complemento para sites em portugues",
	},
}

// ListaBaixada é o texto de uma Fonte, já em mãos.
type ListaBaixada struct {
	Fonte Fonte
	Texto string
}

// Resumo conta o que aconteceu na conversão — os números que aparecem no
// cabeçalho do arquivo e na tela do gerador.
type Resumo struct {
	Brutos      int // domínios distintos que saíram das listas
	PorExcecao  int // tirados porque a própria EasyList os isenta (@@||)
	PorProtecao int // tirados por estarem sob um domínio protegido do Nimbus
	Podados     int // tirados por redundância (o domínio "de cima" já bloqueia)
	Final       int // os que sobraram, e que vão para o arquivo
	Excecoes    int // quantas exceções foram guardadas
}

// MontarArquivo converte as listas baixadas no nosso formato.
//
// São três peneiras, nesta ordem, e cada uma existe por um motivo:
//
//  1. **as exceções da própria EasyList** — se a lista diz "@@||exemplo.com^",
//     ela está avisando que aquilo não é anúncio. Resolver isso agora é mais
//     barato do que decidir a cada pedido do navegador;
//  2. **a trava dos protegidos** — nenhuma regra vinda de fora pode encostar
//     nos serviços que o Nimbus abre. Se a EasyList mandar bloquear
//     "googlevideo.com" (ou um subdomínio dele), a linha simplesmente não é
//     escrita. O programa refaz esta mesma peneira ao carregar o arquivo; aqui
//     é para o arquivo já nascer limpo e o número do cabeçalho ser verdade;
//  3. **a poda do redundante** — o nosso casamento é por rótulo, então se
//     "exemplo.com" está bloqueado, "ads.exemplo.com" já cai junto. Guardar os
//     dois só gasta memória e engorda o arquivo, sem mudar nada.
func MontarArquivo(baixadas []ListaBaixada, quando time.Time) ([]byte, Resumo) {
	bloquear := map[string]bool{}
	excecoes := map[string]bool{}
	var relatorio []string

	for _, b := range baixadas {
		r := LerRegrasEasyList(b.Texto)
		for _, d := range r.Bloquear {
			bloquear[d] = true
		}
		for _, d := range r.Excecoes {
			excecoes[d] = true
		}
		relatorio = append(relatorio, fmt.Sprintf(
			"#   - %s (%s)\n#     %s\n#     %d regras de dominio aproveitadas, %d excecoes, %d regras ignoradas",
			b.Fonte.Nome, b.Fonte.ParaQueE, b.Fonte.URL,
			len(r.Bloquear), len(r.Excecoes), r.Ignoradas))
	}

	var res Resumo
	res.Brutos = len(bloquear)

	for d := range excecoes {
		if bloquear[d] {
			delete(bloquear, d)
			res.PorExcecao++
		}
	}
	for d := range bloquear {
		if ProtegidoOuSubdominio(d) {
			delete(bloquear, d)
			res.PorProtecao++
		}
	}
	for d := range bloquear {
		if paiNaLista(d, bloquear) {
			delete(bloquear, d)
			res.Podados++
		}
	}
	res.Final = len(bloquear)
	res.Excecoes = len(excecoes)

	return escrever(bloquear, excecoes, relatorio, res, quando), res
}

// escrever monta o texto final: um cabeçalho que explica de onde tudo veio
// (com a licença) e depois um domínio por linha.
func escrever(bloquear, excecoes map[string]bool, relatorio []string,
	res Resumo, quando time.Time) []byte {

	var b bytes.Buffer

	b.WriteString("# Lista de dominios de anuncio e rastreamento do Nimbus\n")
	b.WriteString("#\n")
	b.WriteString("# ARQUIVO GERADO AUTOMATICAMENTE - NAO EDITE NA MAO.\n")
	b.WriteString("# Para gerar de novo:  go run ./ferramentas/gerar-listas\n")
	b.WriteString("#\n")
	b.WriteString("# Gerado em: " + quando.Format("2006-01-02 15:04:05 -07:00") + "\n")
	b.WriteString("#\n")
	b.WriteString("# ---------------------------------------------------------------------\n")
	b.WriteString("# LICENCA - IMPORTANTE\n")
	b.WriteString("# ---------------------------------------------------------------------\n")
	b.WriteString("# O codigo do Nimbus e MIT. ESTE ARQUIVO NAO E: ele deriva das listas\n")
	b.WriteString("# publicas da EasyList, distribuidas sob CC BY-SA 3.0 e GPLv3.\n")
	b.WriteString("# Ao redistribuir o Nimbus com este arquivo, mantenha a atribuicao.\n")
	b.WriteString("# Detalhes e creditos completos: docs/LICENCA-LISTAS.md\n")
	b.WriteString("#\n")
	b.WriteString("# Listas de origem:\n")
	for _, l := range relatorio {
		b.WriteString(l + "\n")
	}
	b.WriteString("#\n")
	b.WriteString("# ---------------------------------------------------------------------\n")
	b.WriteString("# O QUE FOI PENEIRADO\n")
	b.WriteString("# ---------------------------------------------------------------------\n")
	fmt.Fprintf(&b, "#   %6d dominios distintos vindos das listas\n", res.Brutos)
	fmt.Fprintf(&b, "# - %6d removidos por excecao da propria EasyList (@@||)\n", res.PorExcecao)
	fmt.Fprintf(&b, "# - %6d removidos por estarem sob um dominio PROTEGIDO do Nimbus\n", res.PorProtecao)
	fmt.Fprintf(&b, "# - %6d removidos por redundancia (o dominio de cima ja bloqueia)\n", res.Podados)
	fmt.Fprintf(&b, "# = %6d DOMINIOS NESTE ARQUIVO\n", res.Final)
	fmt.Fprintf(&b, "#   %6d excecoes (linhas com @ na frente)\n", res.Excecoes)
	b.WriteString("#\n")
	b.WriteString("# Formato: um por linha. '#' e comentario. '@' na frente e excecao\n")
	b.WriteString("# (dominio que NAO deve ser bloqueado).\n")
	b.WriteString("\n")

	for _, d := range ordenado(bloquear) {
		b.WriteString(d)
		b.WriteByte('\n')
	}
	for _, d := range ordenado(excecoes) {
		b.WriteByte('@')
		b.WriteString(d)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

// ordenado devolve as chaves do map em ordem alfabética.
//
// A ordem importa por um motivo prático: sem ela o Go entrega as chaves numa
// ordem diferente a cada execução, e o arquivo mudaria INTEIRO a cada geração —
// o Git mostraria 100 mil linhas alteradas mesmo quando quase nada mudou.
func ordenado(m map[string]bool) []string {
	fora := make([]string, 0, len(m))
	for k := range m {
		fora = append(fora, k)
	}
	sort.Strings(fora)
	return fora
}

// paiNaLista diz se algum domínio "de cima" já está na lista — ou seja, se esta
// entrada é redundante. Para "a.b.exemplo.com" ele confere "b.exemplo.com" e
// "exemplo.com" (o próprio nome não conta, senão tudo seria redundante).
func paiNaLista(dominio string, lista map[string]bool) bool {
	for {
		i := strings.Index(dominio, ".")
		if i < 0 {
			return false
		}
		dominio = dominio[i+1:]
		if lista[dominio] {
			return true
		}
	}
}

// lerNossoFormato transforma o conteúdo do arquivo de volta em dois conjuntos.
//
// Ele é DESCONFIADO de propósito: linha torta é pulada em silêncio, não
// derruba nada. O arquivo pode ter sido baixado pela metade, editado à mão por
// curiosidade ou corrompido por um desligamento no meio da gravação — e nada
// disso pode virar um programa que não abre.
//
// A única falha que ele reporta é "veio pouca coisa" (menos que `minimo`
// domínios): aí o arquivo não presta como lista de bloqueio e quem chamou deve
// cair na reserva.
func lerNossoFormato(dados []byte, minimo int) (bloquear, excecoes map[string]bool, err error) {
	bloquear = make(map[string]bool, 150000)
	excecoes = make(map[string]bool, 20000)

	sc := bufio.NewScanner(bytes.NewReader(dados))
	sc.Buffer(make([]byte, 0, 4096), 64*1024)
	for sc.Scan() {
		linha := strings.TrimSpace(sc.Text())
		if linha == "" || strings.HasPrefix(linha, "#") {
			continue
		}
		ehExcecao := strings.HasPrefix(linha, "@")
		if ehExcecao {
			linha = linha[1:]
		}
		d := DominioValido(linha)
		if d == "" {
			continue // linha torta: ignora e segue
		}
		if ehExcecao {
			excecoes[d] = true
			continue
		}
		// A TRAVA: mesmo que o arquivo mande bloquear um serviço protegido
		// (arquivo antigo, editado à mão ou adulterado), nós não obedecemos.
		if ProtegidoOuSubdominio(d) {
			continue
		}
		bloquear[d] = true
	}
	if err := sc.Err(); err != nil {
		return nil, nil, fmt.Errorf("ler o arquivo de listas: %w", err)
	}
	if len(bloquear) < minimo {
		return nil, nil, fmt.Errorf(
			"o arquivo de listas tem só %d dominios (o minimo aceitavel e %d) — provavelmente veio incompleto",
			len(bloquear), minimo)
	}
	return bloquear, excecoes, nil
}
