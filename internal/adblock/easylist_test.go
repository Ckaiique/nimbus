package adblock

import (
	"strings"
	"testing"
	"time"
)

// pedacoDeEasyList é um trecho no formato EasyList escrito à mão, com um
// exemplo de cada família de regra.
//
// Está aqui dentro do teste de propósito: assim o teste do gerador roda SEM
// INTERNET. Baixar de verdade só acontece quando alguém roda
// `go run ./ferramentas/gerar-listas`.
const pedacoDeEasyList = `[Adblock Plus 2.0]
! Title: Lista de mentirinha para o teste
! Expires: 1 days

! ── o que TEM de entrar ─────────────────────────────────────────────
||anuncio-simples.com^
||anuncio-com-opcoes.com^$third-party
||anuncio-varias-opcoes.com^$script,image,third-party
||sem-circunflexo.com
||MAIUSCULAS.COM^
||sub.dominio.anuncio.com^

! ── exceções: a lista dizendo "isto NÃO é anúncio" ─────────────────
@@||nao-e-anuncio.com^
@@||outro-liberado.com^$third-party

! ── o que NÃO pode entrar ───────────────────────────────────────────
! (1) restringe a sites específicos: virar bloqueio geral seria inventar regra
||parece-anuncio.com^$domain=globo.com
||tambem-restrito.com^$domain=~uol.com.br
! (2) transformam o pedido em vez de barrar
||com-redirect.com^$redirect=noop.js
||com-removeparam.com^$removeparam=utm_source
! (3) regra por caminho: o domínio inteiro não é anúncio, só aquela pasta
||site-de-noticias.com/anuncios/*
||outro-site.com/banner.jpg
! (4) cosméticas: escondem pedaço da página, não têm endereço para bloquear
##.banner-de-anuncio
site.com##.publicidade
site.com#@#.publicidade
! (5) expressão regular: não sabemos avaliar
/banners?[0-9]+/
! (6) sem a âncora "||": casaria por pedaço de texto, que é o erro clássico
/anuncios/*
.com/ads.
! (7) curinga no meio do domínio
||anuncio*.com^
! (8) nem domínio é
||localhost^
||-comeca-com-traco.com^
`

// TestLerRegrasEasyList é o teste do GERADOR: dado um trecho de lista de
// verdade, ele extrai o que sabemos aplicar e ignora o resto.
//
// Cada linha ignorada tem um motivo escrito no trecho acima. Se algum dia
// alguém "melhorar" o gerador para aproveitar mais regras, este teste é quem
// vai perguntar: você tem certeza de que sabe aplicar isso?
func TestLerRegrasEasyList(t *testing.T) {
	r := LerRegrasEasyList(pedacoDeEasyList)

	esperadoBloquear := []string{
		"anuncio-simples.com",
		"anuncio-com-opcoes.com",
		"anuncio-varias-opcoes.com",
		"sem-circunflexo.com",
		"maiusculas.com", // vira minúsculo: a comparação é sempre em minúsculas
		"sub.dominio.anuncio.com",
	}
	conferirConjunto(t, "bloquear", r.Bloquear, esperadoBloquear)

	esperadoExcecoes := []string{"nao-e-anuncio.com", "outro-liberado.com"}
	conferirConjunto(t, "excecoes", r.Excecoes, esperadoExcecoes)

	// Nenhuma das linhas "não pode entrar" pode ter escapado.
	proibidos := []string{
		"parece-anuncio.com", "tambem-restrito.com",
		"com-redirect.com", "com-removeparam.com",
		"site-de-noticias.com", "outro-site.com",
		"site.com", "localhost", "-comeca-com-traco.com",
	}
	for _, p := range proibidos {
		for _, d := range r.Bloquear {
			if d == p {
				t.Errorf("a regra de %q não deveria ter sido aproveitada", p)
			}
		}
	}

	if r.Ignoradas == 0 {
		t.Error("nenhuma regra foi contada como ignorada — a conta não bate")
	}
}

// TestUmaRegraEasyList olha regra por regra, para a mensagem de erro apontar
// exatamente qual formato quebrou.
func TestUmaRegraEasyList(t *testing.T) {
	casos := []struct {
		linha   string
		dominio string
		excecao bool
		ok      bool
	}{
		{"||ads.com^", "ads.com", false, true},
		{"||ads.com", "ads.com", false, true},
		{"||ads.com^$third-party", "ads.com", false, true},
		{"||ads.com^$image,script", "ads.com", false, true},
		{"@@||liberado.com^", "liberado.com", true, true},

		{"||ads.com^$domain=globo.com", "", false, false},
		{"||ads.com^$~third-party", "", false, false},
		{"||ads.com^$redirect=noop", "", false, false},
		{"||ads.com^$opcao-que-nao-conhecemos", "", false, false},
		{"||ads.com/caminho^", "", false, false},
		{"||ads*.com^", "", false, false},
		{"|http://ads.com", "", false, false},
		{"/regex/", "", false, false},
		{"site.com##.banner", "", false, false},
		{"##.banner", "", false, false},
		{"||com^", "", false, false}, // sem ponto: bloquearia meio mundo
		{"||^", "", false, false},
	}
	for _, c := range casos {
		d, exc, ok := UmaRegraEasyList(c.linha)
		if d != c.dominio || exc != c.excecao || ok != c.ok {
			t.Errorf("UmaRegraEasyList(%q) = (%q,%v,%v), esperado (%q,%v,%v)",
				c.linha, d, exc, ok, c.dominio, c.excecao, c.ok)
		}
	}
}

// TestMontarArquivo confere as três peneiras da conversão, que são o que
// separa "juntar tudo" de "juntar com juízo".
func TestMontarArquivo(t *testing.T) {
	lista := `[Adblock Plus 2.0]
! Title: teste
||anuncio-a.com^
||anuncio-b.com^
||sub.anuncio-a.com^
||googlevideo.com^
||algumacoisa.nflxvideo.net^
||sera-isento.com^
@@||sera-isento.com^
@@||outra-excecao.com^
`
	conteudo, res := MontarArquivo(
		[]ListaBaixada{{Fonte: Fonte{Nome: "teste"}, Texto: lista}},
		time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))

	texto := string(conteudo)

	// 1. A exceção da própria lista tira o domínio do bloqueio.
	if res.PorExcecao != 1 {
		t.Errorf("esperava 1 removido por exceção, deu %d", res.PorExcecao)
	}
	if temLinha(texto, "sera-isento.com") {
		t.Error("um domínio isento pela própria EasyList foi parar na lista de bloqueio")
	}

	// 2. A trava dos protegidos: nada sob um serviço do Nimbus entra.
	if res.PorProtecao != 2 {
		t.Errorf("esperava 2 removidos por proteção, deu %d", res.PorProtecao)
	}
	if temLinha(texto, "googlevideo.com") || temLinha(texto, "algumacoisa.nflxvideo.net") {
		t.Error("uma regra sobre servidor de vídeo protegido entrou no arquivo")
	}

	// 3. A poda: "sub.anuncio-a.com" é redundante, já que "anuncio-a.com"
	//    bloqueia a árvore inteira embaixo dele.
	if res.Podados != 1 {
		t.Errorf("esperava 1 podado por redundância, deu %d", res.Podados)
	}
	if temLinha(texto, "sub.anuncio-a.com") {
		t.Error("a poda do redundante não funcionou")
	}

	// O que tinha de sobrar, sobrou.
	if !temLinha(texto, "anuncio-a.com") || !temLinha(texto, "anuncio-b.com") {
		t.Error("um domínio bom se perdeu na conversão")
	}
	if !temLinha(texto, "@outra-excecao.com") {
		t.Error("a exceção não foi guardada no arquivo")
	}

	// O cabeçalho tem de trazer a licença e a data — é obrigação nossa com
	// quem mantém as listas de graça.
	for _, obrigatorio := range []string{"CC BY-SA", "GPLv3", "EasyList", "2026-01-02"} {
		if !strings.Contains(texto, obrigatorio) {
			t.Errorf("o cabeçalho do arquivo gerado perdeu %q", obrigatorio)
		}
	}

	// E o arquivo gerado tem de ser lido de volta sem susto.
	bloquear, excecoes, err := lerNossoFormato(conteudo, 1)
	if err != nil {
		t.Fatalf("o arquivo que nós mesmos geramos não foi lido de volta: %v", err)
	}
	if !bloquear["anuncio-a.com"] || !excecoes["outra-excecao.com"] {
		t.Error("a ida e volta pelo arquivo perdeu informação")
	}
}

// TestParecemListasDeVerdade é a peneira que protege do pior caso do download:
// o servidor devolver uma página de erro em HTML com código 200, e nós
// trocarmos a lista boa por lixo.
func TestParecemListasDeVerdade(t *testing.T) {
	bons := []string{pedacoDeEasyList}
	for _, b := range bons {
		if !ParecemListasDeVerdade(b, 3) {
			t.Error("recusei uma lista de verdade")
		}
	}

	ruins := map[string]string{
		"vazio":           "",
		"html de erro":    "<!DOCTYPE html><html><head><title>503</title></head><body>Service Unavailable</body></html>",
		"json de erro":    `{"error":"not found"}`,
		"cortado no meio": "[Adblock Plus 2.0]\n! Title: cortad",
		"sem cabecalho":   "||anuncio.com^\n||outro.com^\n||terceiro.com^\n||quarto.com^\n",
	}
	for nome, r := range ruins {
		if ParecemListasDeVerdade(r, 3) {
			t.Errorf("aceitei um conteúdo que não é lista: %s", nome)
		}
	}
}

// TestProtegidoOuSubdominio confere a trava usada em todo dado vindo de fora.
func TestProtegidoOuSubdominio(t *testing.T) {
	sim := []string{
		"googlevideo.com",
		"r5---sn-abc.googlevideo.com",
		"youtube.com",
		"www.youtube.com",
		"nflxvideo.net",
		"vod.media.dssott.com",
	}
	for _, d := range sim {
		if !ProtegidoOuSubdominio(d) {
			t.Errorf("%q deveria estar protegido", d)
		}
	}
	nao := []string{
		"doubleclick.net",
		"naogooglevideo.com", // parecido, mas é outro domínio
		"googlevideo.com.br",
		"exemplo.com",
	}
	for _, d := range nao {
		if ProtegidoOuSubdominio(d) {
			t.Errorf("%q NÃO deveria contar como protegido", d)
		}
	}

	if len(DominiosProtegidos()) != len(dominiosProtegidos) {
		t.Error("DominiosProtegidos() não devolveu a lista inteira")
	}
}

// ───────────────────────────── auxiliares ─────────────────────────────────

func conferirConjunto(t *testing.T, nome string, deu, esperado []string) {
	t.Helper()
	if len(deu) != len(esperado) {
		t.Errorf("%s: vieram %d itens (%v), esperava %d (%v)", nome, len(deu), deu, len(esperado), esperado)
		return
	}
	tem := map[string]bool{}
	for _, d := range deu {
		tem[d] = true
	}
	for _, e := range esperado {
		if !tem[e] {
			t.Errorf("%s: faltou %q (vieram: %v)", nome, e, deu)
		}
	}
}

// temLinha procura o domínio como LINHA INTEIRA do arquivo — não como pedaço
// de texto. Procurar por pedaço daria falso positivo justamente nos casos que
// este teste existe para pegar (ex.: "anuncio-a.com" aparece dentro de
// "sub.anuncio-a.com").
func temLinha(texto, alvo string) bool {
	for _, l := range strings.Split(texto, "\n") {
		if strings.TrimSpace(l) == alvo {
			return true
		}
	}
	return false
}
