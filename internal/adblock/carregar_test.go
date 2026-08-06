package adblock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// listaDeBrinquedo monta um arquivo no nosso formato, para os testes não
// precisarem de 2 MB de domínios de verdade.
func listaDeBrinquedo(linhas ...string) []byte {
	cabecalho := "# arquivo de teste\n# Gerado em: 2026-01-02 03:04:05 -03:00\n\n"
	return []byte(cabecalho + strings.Join(linhas, "\n") + "\n")
}

// trocarLista instala uma lista de teste e devolve a anterior no fim do teste.
// Sem isso, um teste estragaria o seguinte (a lista é global, como tem de ser:
// ela é consultada a cada pedido do navegador).
func trocarLista(t *testing.T, dados []byte) {
	t.Helper()
	anterior := atual.Load()
	t.Cleanup(func() { atual.Store(anterior) })

	if err := usar(dados, OrigemBaixada, 1); err != nil {
		t.Fatalf("não consegui instalar a lista de teste: %v", err)
	}
}

// TestListaEmbutidaCarrega confere o caminho normal: o arquivo que veio dentro
// do .exe é lido e rende uma lista grande.
//
// Se este teste falhar, quase sempre é porque alguém esqueceu de rodar
// `go run ./ferramentas/gerar-listas` — e o Nimbus está saindo com a lista
// curta de reserva, bloqueando bem menos do que deveria.
func TestListaEmbutidaCarrega(t *testing.T) {
	l := doEmbutido()
	if l.Origem != OrigemEmbutida {
		t.Fatalf("a lista embutida não carregou (origem=%q). "+
			"Rode: go run ./ferramentas/gerar-listas", l.Origem)
	}
	if l.Quantos() < 50000 {
		t.Errorf("a lista embutida tem só %d domínios — esperava dezenas de milhares", l.Quantos())
	}
	if l.Gerada.IsZero() {
		t.Error("não consegui ler a data de geração do cabeçalho do arquivo")
	}

	// A reserva escrita à mão entra SEMPRE junto, mesmo com a lista grande
	// carregada. É o que mantém casos como "adservice.google.com" valendo.
	for d := range dominiosReserva {
		if !l.bloquear[d] {
			t.Errorf("o domínio da reserva %q sumiu ao carregar a lista grande", d)
		}
	}
}

// TestArquivoAusenteCaiNaReserva: a lista guardada no disco pode simplesmente
// não existir (é o caso normal na primeira vez que o Nimbus roda num PC).
// Isso não pode virar erro visível nem lista vazia.
func TestArquivoAusenteCaiNaReserva(t *testing.T) {
	anterior := atual.Load()
	t.Cleanup(func() { atual.Store(anterior) })

	caminho := filepath.Join(t.TempDir(), "nao-existe.txt")
	if err := UsarArquivo(caminho); err == nil {
		t.Error("ler um arquivo que não existe deveria devolver erro")
	}

	// O ponto principal: a lista que estava valendo continua valendo.
	if !DeveBloquear("https://doubleclick.net/x") {
		t.Error("depois de um arquivo ausente, o bloqueador parou de bloquear")
	}
}

// TestArquivoCorrompidoNaoDerrubaNada cobre os três estragos possíveis: linha
// sem sentido no meio de um arquivo bom, arquivo cortado no meio e arquivo
// vazio.
func TestArquivoCorrompidoNaoDerrubaNada(t *testing.T) {
	// 1. Arquivo BOM com sujeira no meio: as linhas boas continuam valendo e
	//    as tortas são ignoradas em silêncio.
	trocarLista(t, listaDeBrinquedo(
		"anuncio-de-teste.com",
		"isto nao e um dominio",   // tem espaços
		"-comeca-com-traco.com",   // rótulo inválido
		"semponto",                // sem ponto nenhum
		"@excecao-de-teste.com",   // exceção, legítima
		"outro-anuncio-teste.net", // boa
	))
	if !DeveBloquear("https://anuncio-de-teste.com/x") {
		t.Error("a linha boa antes da sujeira deixou de funcionar")
	}
	if !DeveBloquear("https://cdn.outro-anuncio-teste.net/x") {
		t.Error("a linha boa depois da sujeira deixou de funcionar")
	}

	// 2. Arquivo vazio e 3. arquivo cortado no meio: recusados, e a lista
	//    anterior continua valendo (é o que impede um download ruim de apagar
	//    uma lista boa).
	for _, caso := range []struct{ nome, conteudo string }{
		{"vazio", ""},
		{"só comentário", "# Lista de dominios\n# Gerado em: nunca\n"},
		{"cortado no meio", "# Lista de dom"},
		{"html de erro", "<html><body>503 Service Unavailable</body></html>"},
	} {
		if err := usar([]byte(caso.conteudo), OrigemBaixada, 1); err == nil {
			t.Errorf("%s: deveria ter sido recusado", caso.nome)
		}
		if !DeveBloquear("https://anuncio-de-teste.com/x") {
			t.Errorf("%s: a lista anterior foi perdida", caso.nome)
		}
	}
}

// TestExcecaoDaEasyListImpedeOBloqueio: quando a lista diz "@@||algo^", ela
// está avisando que aquilo não é anúncio. Nós obedecemos.
func TestExcecaoDaEasyListImpedeOBloqueio(t *testing.T) {
	trocarLista(t, listaDeBrinquedo(
		"exemplo-misto.com",
		"@parte-boa.exemplo-misto.com",
	))

	if !DeveBloquear("https://exemplo-misto.com/anuncio.js") {
		t.Error("o domínio de cima deveria continuar bloqueado")
	}
	if DeveBloquear("https://parte-boa.exemplo-misto.com/x.js") {
		t.Error("a exceção da EasyList foi ignorada — este endereço não devia ser bloqueado")
	}
	// Subdomínio da exceção também é isento (a exceção vale para a árvore
	// inteira embaixo dela).
	if DeveBloquear("https://cdn.parte-boa.exemplo-misto.com/x.js") {
		t.Error("a exceção deveria valer também para os subdomínios dela")
	}
}

// TestProtegidoVenceRegraDaEasyList é a trava mais importante do bloqueador.
//
// Cenário real e nada improvável: a EasyList inclui um servidor de vídeo do
// YouTube porque ele também entrega anúncio. Num navegador comum isso custa um
// vídeo que não toca. No Nimbus custaria as QUATRO coisas que o programa existe
// para mostrar — e o dono veria só uma tela preta.
func TestProtegidoVenceRegraDaEasyList(t *testing.T) {
	trocarLista(t, listaDeBrinquedo(
		"googlevideo.com",             // a EasyList mandando bloquear o vídeo
		"r5---sn-abc.googlevideo.com", // e um subdomínio específico dele
		"nflxvideo.net",
		"dssott.com",
		"youtube.com",
		"anuncio-de-teste.com", // uma regra normal, para provar que o resto funciona
	))

	protegidos := []string{
		"https://r5---sn-abc.googlevideo.com/videoplayback?x=1",
		"https://redirector.googlevideo.com/x",
		"https://ipv4-c001.nflxvideo.net/range/0-100",
		"https://vod-x.media.dssott.com/x.m3u8",
		"https://www.youtube.com/watch?v=abc",
	}
	for _, u := range protegidos {
		if DeveBloquear(u) {
			t.Errorf("uma regra da EasyList conseguiu bloquear %q — a trava dos protegidos falhou", u)
		}
	}

	if !DeveBloquear("https://anuncio-de-teste.com/x") {
		t.Error("a regra normal da lista deveria continuar bloqueando")
	}
}

// TestListaBaixadaGanhaDaEmbutida: o arquivo baixado é mais novo, então é ele
// que vale.
func TestListaBaixadaGanhaDaEmbutida(t *testing.T) {
	anterior := atual.Load()
	t.Cleanup(func() { atual.Store(anterior) })

	// Um domínio que com certeza não está na lista embutida.
	const novo = "anuncio-inventado-para-o-teste.example"
	if DeveBloquear("https://" + novo + "/x") {
		t.Fatalf("%s não deveria estar na lista embutida", novo)
	}

	arquivo := filepath.Join(t.TempDir(), "listas.txt")
	if err := os.WriteFile(arquivo, listaDeBrinquedo(novo), 0o644); err != nil {
		t.Fatal(err)
	}
	// Caminho normal usa o mínimo de verdade; aqui vamos direto ao miolo com
	// um mínimo de brinquedo, para não precisar de 100 mil linhas no teste.
	dados, err := os.ReadFile(arquivo)
	if err != nil {
		t.Fatal(err)
	}
	if err := usar(dados, OrigemBaixada, 1); err != nil {
		t.Fatal(err)
	}

	if EmUso().Origem != OrigemBaixada {
		t.Errorf("origem = %q, esperava %q", EmUso().Origem, OrigemBaixada)
	}
	if !DeveBloquear("https://" + novo + "/x") {
		t.Error("a lista baixada não passou a valer")
	}
	// E a reserva escrita à mão continua junto.
	if !DeveBloquear("https://doubleclick.net/x") {
		t.Error("a lista baixada apagou a reserva escrita à mão")
	}
}

// TestDataDoArquivo confere a leitura da data do cabeçalho — que é o que
// permite a Config dizer "listas de 3 dias atrás".
func TestDataDoArquivo(t *testing.T) {
	if DataDoArquivo(listaDeBrinquedo("a.com")).IsZero() {
		t.Error("não li a data de um cabeçalho bem formado")
	}
	if !DataDoArquivo([]byte("# sem data aqui\n")).IsZero() {
		t.Error("inventei uma data onde não havia nenhuma")
	}
	if !DataDoArquivo([]byte("# Gerado em: ontem de tarde\n")).IsZero() {
		t.Error("aceitei uma data que não dá para entender")
	}
}

// ─────────────────────────── desempenho ───────────────────────────────────

// BenchmarkDeveBloquear mede o custo da decisão COM a lista grande carregada.
//
// Por que isto importa: DeveBloquear é chamada a cada coisa que a página pede —
// centenas de vezes por site, e no meio do carregamento. Se ela ficasse lenta,
// a navegação inteira ficaria lenta junto, e a culpa pareceria ser do WebView2.
//
// Os três casos cobrem o que acontece de verdade: um endereço bloqueado, um
// endereço comum (o caso mais frequente, que sai cedo) e um endereço protegido.
func BenchmarkDeveBloquear(b *testing.B) {
	casos := []struct{ nome, url string }{
		{"bloqueado", "https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js"},
		{"liberado", "https://www.exemplo-qualquer-do-mundo.com/imagens/foto-grande.jpg"},
		{"protegido", "https://r5---sn-abc.googlevideo.com/videoplayback?expire=1"},
	}
	// Garante a lista carregada antes de medir (o carregamento é preguiçoso).
	if QuantosDominios() < 1000 {
		b.Skipf("lista pequena (%d domínios): rode o gerador antes", QuantosDominios())
	}
	for _, c := range casos {
		b.Run(c.nome, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				DeveBloquear(c.url)
			}
		})
	}
}

// BenchmarkCarregarListaEmbutida mede quanto custa montar a lista a partir do
// arquivo embutido. Acontece UMA vez por execução do programa, em segundo
// plano — mas se passar de alguns décimos de segundo, vale repensar.
func BenchmarkCarregarListaEmbutida(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if l := doEmbutido(); l.Quantos() == 0 {
			b.Fatal("a lista veio vazia")
		}
	}
}
