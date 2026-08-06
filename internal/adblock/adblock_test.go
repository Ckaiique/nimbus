package adblock

import "testing"

// TestDeveBloquear cobre a decisão inteira.
//
// Os casos foram escolhidos pelos ERROS QUE ESTE TIPO DE CÓDIGO COSTUMA
// COMETER, não pelo caminho feliz:
//
//   - bloquear por "contém o texto" (e derrubar site de terceiro inocente);
//   - esquecer que o subdomínio também tem de cair;
//   - engasgar com endereço torto, vazio ou com porta;
//   - e o pior de todos: bloquear o servidor de onde o VÍDEO sai.
func TestDeveBloquear(t *testing.T) {
	casos := []struct {
		nome     string
		url      string
		bloquear bool
	}{
		// ── o básico ──────────────────────────────────────────────────────
		{"dominio exato", "https://doubleclick.net/algo.js", true},
		{"subdominio", "https://pagead2.googlesyndication.com/pagead/js/x.js", true},
		{"subdominio fundo", "https://a.b.c.googlesyndication.com/x", true},
		{"outro da lista", "http://www.criteo.com/pixel", true},

		// ── o caso que dá nome ao teste ───────────────────────────────────
		// "naogoogle-analytics.com.br" CONTÉM "google-analytics.com" como
		// texto, mas é outro domínio, de outra empresa. Bloquear isso seria
		// derrubar o site de alguém sem nenhum motivo.
		{"parecido mas nao e", "https://naogoogle-analytics.com.br/pagina", false},
		{"parecido mas nao e 2", "https://meudoubleclick.net.br/x", false},
		{"parecido no meio", "https://cdn.taboola.com.evil.example/x", false},

		// ── endereços tortos: nunca bloquear, nunca quebrar ───────────────
		{"vazio", "", false},
		{"so espacos", "   ", false},
		{"sem esquema nem nada", "isso nao e uma url", false},
		{"esquema interno", "about:blank", false},
		{"dados embutidos", "data:text/html,<b>oi</b>", false},
		{"blob", "blob:https://www.youtube.com/1234", false},
		{"so o esquema", "https://", false},

		// ── porta e maiúsculas ────────────────────────────────────────────
		{"com porta", "https://ads.doubleclick.net:8443/x.js", true},
		{"com porta e sem esquema", "doubleclick.net:8080/x", true},
		{"maiusculas", "HTTPS://PAGEAD2.GOOGLESYNDICATION.COM/X.JS", true},
		{"ponto final", "https://doubleclick.net./x", true},
		{"com usuario e senha", "https://eu:senha@doubleclick.net/x", true},
		{"websocket de rastreio", "wss://analytics.google.com/socket", true},

		// ── os quatro serviços: NUNCA podem cair ──────────────────────────
		{"youtube", "https://www.youtube.com/watch?v=abc", false},
		{"youtube api", "https://www.youtube.com/api/stats/ads?x=1", false},
		{"video do youtube", "https://r5---sn-abc.googlevideo.com/videoplayback?x", false},
		{"miniaturas do youtube", "https://i.ytimg.com/vi/abc/hq.jpg", false},
		{"yt music", "https://music.youtube.com/playlist", false},
		{"netflix", "https://www.netflix.com/browse", false},
		{"video da netflix", "https://ipv4-c001.nflxvideo.net/range/0-100", false},
		{"disney", "https://www.disneyplus.com/pt-br", false},
		{"video do disney", "https://vod-x.media.dssott.com/x.m3u8", false},

		// ── o desempate: o mais específico vence ──────────────────────────
		// "google.com" é protegido (não podemos quebrar o login), mas o
		// subdomínio de anúncio dele tem de cair mesmo assim.
		{"google normal", "https://accounts.google.com/login", false},
		{"anuncio do google", "https://adservice.google.com/pagead/x", true},
		{"medicao do google", "https://analytics.google.com/g/collect", true},
		{"arquivos do google", "https://fonts.gstatic.com/s/roboto.woff2", false},
	}

	for _, c := range casos {
		if got := DeveBloquear(c.url); got != c.bloquear {
			t.Errorf("%s: DeveBloquear(%q) = %v, esperado %v",
				c.nome, c.url, got, c.bloquear)
		}
	}
}

// TestDominioDaURL confere só a parte de "achar o nome do servidor", que é onde
// os endereços tortos costumam derrubar o programa.
func TestDominioDaURL(t *testing.T) {
	casos := []struct{ url, dominio string }{
		{"https://www.exemplo.com/a/b?c=1#d", "www.exemplo.com"},
		{"http://exemplo.com:8080", "exemplo.com"},
		{"https://EXEMPLO.com", "exemplo.com"},
		{"https://exemplo.com.", "exemplo.com"},
		{"https://eu@exemplo.com/x", "exemplo.com"},
		{"https://[::1]:8080/x", "::1"},
		{"about:blank", ""},
		{"", ""},
		{"   ", ""},
		{"ftp://exemplo.com/arquivo", ""},
		{"texto solto", ""},
	}
	for _, c := range casos {
		if got := DominioDaURL(c.url); got != c.dominio {
			t.Errorf("DominioDaURL(%q) = %q, esperado %q", c.url, got, c.dominio)
		}
	}
}

// TestListasCoerentes é uma rede de segurança contra o erro humano de digitar um
// domínio errado na lista — ou, pior, de escrever nela um domínio de serviço.
func TestListasCoerentes(t *testing.T) {
	if QuantosDominios() < 60 {
		t.Errorf("a lista de bloqueio ficou pequena demais: %d domínios", QuantosDominios())
	}

	// Nenhum domínio pode estar nas duas listas ao mesmo tempo com o MESMO
	// nome (aí o desempate por tamanho empataria e o comportamento ficaria
	// confuso de explicar).
	for d := range dominiosBloqueados {
		if dominiosProtegidos[d] {
			t.Errorf("%q está nas duas listas ao mesmo tempo", d)
		}
	}

	// Todo domínio tem de ser minúsculo, com ponto e sem espaço, senão nunca
	// casaria com nada (a comparação é feita em minúsculas).
	conferir := func(nome string, lista map[string]bool) {
		for d := range lista {
			if d == "" || d != DominioDaURL("https://"+d+"/") {
				t.Errorf("%s: domínio mal escrito: %q", nome, d)
			}
		}
	}
	conferir("bloqueio", dominiosBloqueados)
	conferir("protegidos", dominiosProtegidos)
}

// TestScriptDeLimpeza confere o mínimo que não pode faltar no JavaScript.
// Não dá para rodar navegador aqui, então testamos o que é possível: que o
// script existe, que ele consulta a chave de desligar e — o mais importante —
// que a trava do "só mexe quando estiver passando anúncio" continua lá.
func TestScriptDeLimpeza(t *testing.T) {
	s := ScriptDeLimpeza()
	if s == "" {
		t.Fatal("o script de limpeza veio vazio")
	}
	obrigatorios := []string{
		"ad-showing",               // a trava que impede adiantar o vídeo de verdade
		"__nimbusAdblockDesligado", // dá para desligar em tempo real
		"ytp-ad-skip-button",       // o botão de pular
		"setInterval",              // roda de tempos em tempos, sem travar a página
	}
	for _, o := range obrigatorios {
		if !contem(s, o) {
			t.Errorf("o script de limpeza perdeu %q", o)
		}
	}

	if ChaveDeDesligar(true) == ChaveDeDesligar(false) {
		t.Error("ligar e desligar deveriam gerar comandos diferentes")
	}
}

func contem(texto, pedaco string) bool {
	for i := 0; i+len(pedaco) <= len(texto); i++ {
		if texto[i:i+len(pedaco)] == pedaco {
			return true
		}
	}
	return false
}
