package listas

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"nimbus/internal/adblock"
)

// ─────────────────────── o servidor de mentirinha ─────────────────────────
//
// Nenhum teste deste arquivo encosta na internet. Todos falam com um servidor
// criado na hora pelo httptest, que responde o que o teste mandar responder —
// inclusive as respostas RUINS, que são justamente as que precisam de teste.

// listaFalsa monta um texto no formato EasyList com `quantos` domínios
// inventados, mais o que o teste quiser acrescentar.
func listaFalsa(quantos int, extras ...string) string {
	var b strings.Builder
	b.WriteString("[Adblock Plus 2.0]\n! Title: Lista de teste\n")
	for i := 0; i < quantos; i++ {
		fmt.Fprintf(&b, "||anuncio-de-teste-%05d.example^\n", i)
	}
	for _, e := range extras {
		b.WriteString(e + "\n")
	}
	return b.String()
}

// servidorQueResponde devolve sempre o mesmo texto, e conta quantas vezes foi
// procurado (é assim que os testes provam que NÃO houve download).
func servidorQueResponde(texto string, vezes *int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if vezes != nil {
			*vezes++
		}
		fmt.Fprint(w, texto)
	}))
}

// novoParaTeste monta um Atualizador apontando para o servidor de mentirinha e
// para uma pasta temporária — nunca para a pasta de dados de verdade do PC.
func novoParaTeste(t *testing.T, srv *httptest.Server) *Atualizador {
	t.Helper()
	return &Atualizador{
		Pasta:      t.TempDir(),
		Fontes:     []adblock.Fonte{{Nome: "teste", URL: srv.URL}},
		Cliente:    srv.Client(),
		Intervalo:  7 * 24 * time.Hour,
		Minimo:     100, // lista de brinquedo: o mínimo de verdade é 10 mil
		automatico: true,
	}
}

// ───────────────────────────── os testes ──────────────────────────────────

// TestAtualizaQuandoPassouDoPrazo é o caminho feliz: nunca tentamos antes,
// então baixa, aceita e passa a usar.
func TestAtualizaQuandoPassouDoPrazo(t *testing.T) {
	var vezes int
	srv := servidorQueResponde(listaFalsa(1200), &vezes)
	defer srv.Close()

	a := novoParaTeste(t, srv)
	if !a.PassouDoPrazo() {
		t.Fatal("sem nenhuma tentativa registrada, deveria estar na hora de atualizar")
	}
	if err := a.atualizarAgora(); err != nil {
		t.Fatalf("a atualização falhou: %v", err)
	}
	if vezes != 1 {
		t.Errorf("o servidor foi procurado %d vezes, esperava 1", vezes)
	}

	// O arquivo ficou guardado na pasta de dados...
	if _, err := os.Stat(a.ArquivoDaLista()); err != nil {
		t.Errorf("a lista baixada não foi guardada: %v", err)
	}
	// ...e não sobrou arquivo temporário pela metade.
	if _, err := os.Stat(a.ArquivoDaLista() + ".novo"); err == nil {
		t.Error("sobrou um arquivo temporário depois da troca")
	}
	// ...e a data da tentativa foi registrada.
	if a.UltimaTentativa().IsZero() {
		t.Error("a data da tentativa não foi gravada")
	}
	// ...e agora ela é que vale.
	if adblock.EmUso().Origem != adblock.OrigemBaixada {
		t.Errorf("a lista em uso é %q, esperava %q", adblock.EmUso().Origem, adblock.OrigemBaixada)
	}
	if !adblock.DeveBloquear("https://anuncio-de-teste-00007.example/x.js") {
		t.Error("um domínio da lista baixada não está sendo bloqueado")
	}
}

// TestNaoAtualizaAntesDoPrazo: baixar 4 MB toda vez que o programa abre seria
// gastar a internet do dono à toa e sobrecarregar um servidor mantido de graça
// por voluntários.
func TestNaoAtualizaAntesDoPrazo(t *testing.T) {
	var vezes int
	srv := servidorQueResponde(listaFalsa(1200), &vezes)
	defer srv.Close()

	a := novoParaTeste(t, srv)
	a.marcarTentativa() // "acabamos de tentar"

	if a.PassouDoPrazo() {
		t.Error("passou do prazo logo depois de uma tentativa?")
	}

	// O Iniciar tem de olhar o prazo e não baixar nada.
	a.Iniciar()
	esperarParar(t, a)
	if vezes != 0 {
		t.Errorf("baixou %d vezes mesmo dentro do prazo", vezes)
	}
}

// TestDesligadoNaoBaixa: a caixinha "Atualizar listas sozinho" tem de valer de
// verdade. Um programa que acessa a internet sozinho e não pode ser impedido é
// invasivo, mesmo fazendo isso por um bom motivo.
func TestDesligadoNaoBaixa(t *testing.T) {
	var vezes int
	srv := servidorQueResponde(listaFalsa(1200), &vezes)
	defer srv.Close()

	a := novoParaTeste(t, srv)
	a.DefinirAutomatico(false)

	a.Iniciar()
	esperarParar(t, a)

	if vezes != 0 {
		t.Errorf("baixou %d vezes com a atualização automática desligada", vezes)
	}
	if a.Estado().Automatico {
		t.Error("o estado ainda diz que a atualização automática está ligada")
	}
}

// TestDownloadFalhandoMantemAListaAnterior: sem internet, servidor fora do ar,
// erro 500 — nada disso pode custar a lista que já funcionava.
func TestDownloadFalhandoMantemAListaAnterior(t *testing.T) {
	// Primeiro, uma atualização que dá certo.
	bom := servidorQueResponde(listaFalsa(1200, "||so-na-lista-boa.example^"), nil)
	defer bom.Close()

	a := novoParaTeste(t, bom)
	if err := a.atualizarAgora(); err != nil {
		t.Fatalf("a primeira atualização deveria ter funcionado: %v", err)
	}
	if !adblock.DeveBloquear("https://so-na-lista-boa.example/x") {
		t.Fatal("a lista boa não entrou")
	}
	conteudoBom, err := os.ReadFile(a.ArquivoDaLista())
	if err != nil {
		t.Fatal(err)
	}

	// Agora um servidor com problema.
	ruim := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "deu ruim", http.StatusInternalServerError)
	}))
	defer ruim.Close()

	a.Fontes = []adblock.Fonte{{Nome: "teste", URL: ruim.URL}}
	a.Cliente = ruim.Client()
	if err := a.atualizarAgora(); err == nil {
		t.Error("uma resposta 500 deveria ter sido recusada")
	}

	// O que importa: nada foi perdido.
	if !adblock.DeveBloquear("https://so-na-lista-boa.example/x") {
		t.Error("a lista boa foi perdida por causa de um download que falhou")
	}
	depois, err := os.ReadFile(a.ArquivoDaLista())
	if err != nil || string(depois) != string(conteudoBom) {
		t.Error("o arquivo guardado foi estragado por um download que falhou")
	}
	if a.Estado().UltimoErro == "" {
		t.Error("o erro deveria ter ficado registrado no estado (mesmo sem aparecer para o usuário)")
	}
}

// TestConteudoInvalidoERecusado é o teste do pior caso: o servidor responde
// 200 OK, mas o que vem não é lista nenhuma. Aceitar isso trocaria a lista boa
// por lixo e o Nimbus pararia de bloquear sem ninguém entender por quê.
func TestConteudoInvalidoERecusado(t *testing.T) {
	casos := map[string]string{
		"pagina de erro em html": "<!DOCTYPE html><html><body><h1>503 Service Unavailable</h1></body></html>",
		"vazio":                  "",
		"cortado no meio":        "[Adblock Plus 2.0]\n! Title: Lista de te",
		"json de erro":           `{"error":"quota exceeded"}`,
		"lista quase vazia":      listaFalsa(2),
	}

	for nome, resposta := range casos {
		t.Run(nome, func(t *testing.T) {
			srv := servidorQueResponde(resposta, nil)
			defer srv.Close()

			a := novoParaTeste(t, srv)
			if err := a.atualizarAgora(); err == nil {
				t.Fatal("deveria ter recusado")
			}
			// Nada foi gravado na pasta de dados.
			if _, err := os.Stat(a.ArquivoDaLista()); err == nil {
				t.Error("gravou um arquivo a partir de conteúdo inválido")
			}
			// E o bloqueador continua funcionando com o que já tinha.
			if !adblock.DeveBloquear("https://doubleclick.net/x") {
				t.Error("o bloqueador parou de funcionar depois de um download inválido")
			}
		})
	}
}

// TestProtegidoContinuaProtegido é a trava de segurança olhada do lado de fora:
// mesmo que a lista BAIXADA mande bloquear o servidor de onde sai o vídeo, nós
// não obedecemos. Num navegador comum isso custaria um site; aqui custaria as
// quatro coisas que o Nimbus existe para mostrar.
func TestProtegidoContinuaProtegido(t *testing.T) {
	srv := servidorQueResponde(listaFalsa(1200,
		"||googlevideo.com^",
		"||r5---sn-abc.googlevideo.com^",
		"||nflxvideo.net^",
		"||dssott.com^",
		"||youtube.com^",
	), nil)
	defer srv.Close()

	a := novoParaTeste(t, srv)
	if err := a.atualizarAgora(); err != nil {
		t.Fatalf("a atualização falhou: %v", err)
	}

	protegidos := []string{
		"https://r5---sn-abc.googlevideo.com/videoplayback?x=1",
		"https://redirector.googlevideo.com/x",
		"https://ipv4-c001.nflxvideo.net/range/0-100",
		"https://vod-x.media.dssott.com/x.m3u8",
		"https://www.youtube.com/watch?v=abc",
	}
	for _, u := range protegidos {
		if adblock.DeveBloquear(u) {
			t.Errorf("a lista baixada conseguiu bloquear %q — a trava falhou", u)
		}
	}

	// E o arquivo guardado no disco também nasceu limpo.
	dados, err := os.ReadFile(a.ArquivoDaLista())
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(dados), "\n") {
		if strings.TrimSpace(l) == "googlevideo.com" {
			t.Error("um domínio protegido foi gravado no arquivo de bloqueio")
		}
	}
}

// TestUmaAtualizacaoDeCadaVez: clicar duas vezes no botão não pode baixar tudo
// duas vezes nem deixar dois gravadores disputando o mesmo arquivo.
func TestUmaAtualizacaoDeCadaVez(t *testing.T) {
	// O servidor segura a resposta até o teste mandar soltar, para as duas
	// chamadas se encontrarem no meio do caminho.
	solte := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-solte
		fmt.Fprint(w, listaFalsa(1200))
	}))
	defer srv.Close()

	a := novoParaTeste(t, srv)

	pronto := make(chan error, 1)
	go func() { pronto <- a.atualizarAgora() }()

	// Espera a primeira estar em andamento.
	esperar(t, func() bool { return a.Estado().Atualizando })

	if err := a.atualizarAgora(); err == nil {
		t.Error("a segunda atualização simultânea deveria ter desistido")
	}

	close(solte)
	if err := <-pronto; err != nil {
		t.Errorf("a primeira atualização falhou: %v", err)
	}
}

// TestCarregaListaGuardadaAoIniciar: na abertura seguinte, a lista que já foi
// baixada uma vez tem de valer — sem precisar baixar de novo.
func TestCarregaListaGuardadaAoIniciar(t *testing.T) {
	var vezes int
	srv := servidorQueResponde(listaFalsa(1200, "||guardada-no-disco.example^"), &vezes)
	defer srv.Close()

	pasta := t.TempDir()

	// Primeira execução: baixa e guarda.
	a1 := novoParaTeste(t, srv)
	a1.Pasta = pasta
	if err := a1.atualizarAgora(); err != nil {
		t.Fatal(err)
	}

	// Segunda "execução": só inicia. Não pode baixar (está dentro do prazo),
	// mas tem de carregar o arquivo do disco.
	vezes = 0
	a2 := novoParaTeste(t, srv)
	a2.Pasta = pasta
	a2.Iniciar()
	esperarParar(t, a2)

	if vezes != 0 {
		t.Errorf("baixou %d vezes na segunda abertura, dentro do prazo", vezes)
	}
	if !adblock.DeveBloquear("https://guardada-no-disco.example/x") {
		t.Error("a lista guardada no disco não foi carregada na abertura seguinte")
	}
	if adblock.EmUso().Origem != adblock.OrigemBaixada {
		t.Errorf("origem em uso = %q", adblock.EmUso().Origem)
	}
}

// TestPastaAusenteNaoDerruba: num PC onde o sistema não sabe informar a pasta
// de dados, tudo continua funcionando — só não guarda nada.
func TestPastaAusenteNaoDerruba(t *testing.T) {
	srv := servidorQueResponde(listaFalsa(1200), nil)
	defer srv.Close()

	a := novoParaTeste(t, srv)
	a.Pasta = ""

	if a.ArquivoDaLista() != "" {
		t.Error("sem pasta de dados, não deveria haver caminho de arquivo")
	}
	if !a.PassouDoPrazo() {
		t.Error("sem pasta não dá para saber a última tentativa: tem de tentar")
	}
	if err := a.atualizarAgora(); err != nil {
		t.Errorf("deveria funcionar mesmo sem onde guardar: %v", err)
	}
}

// ───────────────────────────── auxiliares ─────────────────────────────────

// esperar espera uma condição virar verdadeira, com limite de tempo. Serve
// para os testes de goroutine não ficarem pendurados para sempre quando algo
// dá errado.
func esperar(t *testing.T, condicao func() bool) {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) {
		if condicao() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("esperei demais por uma condição que não aconteceu")
}

// esperarParar espera o trabalho em segundo plano terminar.
func esperarParar(t *testing.T, a *Atualizador) {
	t.Helper()
	// Um instante para a goroutine sair do lugar, e então esperamos ela parar.
	time.Sleep(20 * time.Millisecond)
	esperar(t, func() bool { return !a.Estado().Atualizando })
}
