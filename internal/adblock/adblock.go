// Pacote adblock: a decisão de "isto é anúncio/rastreador, não deixa passar".
//
// ─── O QUE ESTE PACOTE É (e o que NÃO é) ──────────────────────────────────
//
// Ele é a PARTE PENSANTE do bloqueador: recebe o endereço de cada coisa que a
// página pede (imagem, script, vídeo, pedido de rede...) e responde sim ou não.
// Quem realmente barra o pedido é o `internal/player`, que fala com o navegador.
//
// Fizemos assim de propósito: a decisão é uma FUNÇÃO PURA (só entra texto, só
// sai sim/não), então dá para testar tudo sem abrir navegador nenhum. E é bom
// que seja testado: um erro aqui não "deixa um anúncio passar" — ele pode
// derrubar o vídeo inteiro, se bloquearmos por engano o servidor que entrega o
// filme.
//
// ─── HONESTIDADE: o que ele resolve e o que não resolve ───────────────────
//
// RESOLVE BEM: banners, pop-ups, pixels de rastreamento e a maior parte da
// publicidade comum da web. Isso funciona porque essas coisas vêm de servidores
// de publicidade com endereço próprio (doubleclick.net, criteo.com...), e basta
// não buscar nada nesses endereços.
//
// NÃO RESOLVE por endereço: o anúncio de vídeo do YouTube. Ele vem do MESMO
// servidor que entrega o vídeo que você quer ver (googlevideo.com). Bloquear
// esse endereço não tira o anúncio: tira o vídeo também. Por isso o YouTube é
// tratado de outro jeito — a gente PULA o anúncio (veja limpeza.go).
//
// E o YouTube muda a técnica dele com frequência, então o "pular sozinho" pode
// parar de funcionar de um dia para o outro, e o site pode até perceber e
// reclamar. Isso é uma corrida sem fim. A meta aqui é ser útil no dia a dia do
// dono do PC — não virar um uBlock Origin.
//
// ─── De onde vem a lista (e por que ela nunca falta) ──────────────────────
//
// O grosso da lista vem das listas públicas da EasyList, convertidas para um
// formato simples e guardadas DENTRO do .exe. O Nimbus não baixa nada para
// funcionar: ele abre offline, com a lista que já tem.
//
// Ele pode, sim, buscar uma versão mais nova em segundo plano (a cada 7 dias, e
// dá para desligar na aba Config) — mas isso é um extra. Se a internet estiver
// fora, se o servidor sumir ou se o dono desligar a opção, tudo continua
// funcionando com a lista embutida. E se até ela falhar, existe a lista curta
// escrita à mão em `listas.go`. Veja `carregar.go` para a ordem completa.
package adblock

import "strings"

// DeveBloquear responde se aquele endereço é de anúncio ou rastreamento.
//
// A regra tem três listas e um critério de desempate:
//
//  1. a lista de bloqueio (EasyList + os domínios escritos à mão);
//  2. a lista de proteção (dominiosProtegidos), com os serviços que o dono usa;
//  3. a lista de exceções que a própria EasyList declara ("isto não é anúncio");
//  4. no empate, VENCE O MAIS ESPECÍFICO (o nome mais comprido que casou), e um
//     empate exato é resolvido a favor de NÃO bloquear.
//
// O passo 4 existe por um caso real: "google.com" está protegido (não podemos
// quebrar o login do Google), mas "adservice.google.com" é servidor de anúncio.
// Como o segundo é mais específico, ele ganha e o anúncio é barrado — sem
// derrubar o resto do Google.
//
// Endereço que não dá para entender, vazio, ou de um esquema que não é web
// (data:, blob:, about:...) devolve false. A regra da casa aqui é
// **na dúvida, deixa passar**: um anúncio a mais é chato; uma página quebrada
// é o programa parecendo defeituoso.
func DeveBloquear(url string) bool {
	dominio := DominioDaURL(url)
	if dominio == "" {
		return false
	}

	l := listaEmUso()

	bloqueio := casamentoMaisEspecifico(dominio, l.bloquear)
	if bloqueio == 0 {
		return false // ninguém mandou bloquear: caminho mais comum, sai cedo
	}
	// A trava dos serviços do dono. Empate (mesmo nome nas duas listas) conta
	// como proteção — é o "na dúvida, deixa passar".
	if casamentoMaisEspecifico(dominio, dominiosProtegidos) >= bloqueio {
		return false
	}
	// A própria EasyList dizendo "este aqui não é anúncio".
	if casamentoMaisEspecifico(dominio, l.excecoes) >= bloqueio {
		return false
	}
	return true
}

// DominioDaURL tira o nome do servidor de dentro do endereço.
//
// Exemplo: "https://pagead2.googlesyndication.com:443/x.js?a=1" vira
// "pagead2.googlesyndication.com".
//
// Devolve string vazia quando não é um endereço da web que a gente saiba
// tratar. É exportada porque também serve para depurar e para os testes.
func DominioDaURL(url string) string {
	u := strings.TrimSpace(strings.ToLower(url))
	if u == "" {
		return ""
	}

	// 1. O esquema (a parte antes de "://"). Só nos interessam os da web.
	//    Coisas como "data:", "blob:", "about:blank" e "edge://" são internas
	//    do navegador — nunca bloqueamos nada disso.
	if i := strings.Index(u, "://"); i >= 0 {
		esquema := u[:i]
		if esquema != "http" && esquema != "https" && esquema != "ws" && esquema != "wss" {
			return ""
		}
		u = u[i+3:]
	} else if i := strings.IndexAny(u, ":/?#"); i >= 0 && u[i] == ':' {
		// Tem dois-pontos mas não tem "://". Pode ser uma PORTA
		// ("exemplo.com:8080/x") ou um esquema esquisito ("data:algo").
		// Se o que vem depois for número, é porta e seguimos; senão, desistimos.
		if i+1 >= len(u) || u[i+1] < '0' || u[i+1] > '9' {
			return ""
		}
	}

	// 2. Fora tudo que vem depois do servidor (caminho, busca e âncora).
	if i := strings.IndexAny(u, "/?#"); i >= 0 {
		u = u[:i]
	}

	// 3. Fora o "usuario:senha@" que alguns endereços trazem na frente.
	if i := strings.LastIndex(u, "@"); i >= 0 {
		u = u[i+1:]
	}

	// 4. Fora a porta. Endereço IPv6 vem entre colchetes ("[::1]:80") e o
	//    dois-pontos de dentro NÃO é porta — por isso o caso separado.
	if strings.HasPrefix(u, "[") {
		if i := strings.Index(u, "]"); i > 0 {
			u = u[1:i]
		} else {
			return ""
		}
	} else if i := strings.LastIndex(u, ":"); i >= 0 {
		u = u[:i]
	}

	// 5. O ponto final é válido em nome de domínio ("exemplo.com.") e significa
	//    a mesma coisa. Tiramos para as comparações baterem.
	u = strings.TrimSuffix(u, ".")

	// 6. Sobrou algo que pareça um nome de servidor? Espaço no meio já diz que
	//    não era um endereço de verdade.
	if u == "" || strings.ContainsAny(u, " \t") {
		return ""
	}
	return u
}

// casamentoMaisEspecifico procura, na lista dada, o domínio mais comprido que
// casa com o servidor — e devolve o tamanho dele (0 se nenhum casou).
//
// ⚠️ O CASAMENTO É POR RÓTULO, nunca por "contém o texto". Rótulo é cada
// pedaço entre pontos. Então "googlesyndication.com" casa com
// "pagead2.googlesyndication.com" (é um subdomínio dele), mas NÃO casa com
// "naogoogle-analytics.com.br" nem com "meugooglesyndication.com.br" — que só
// por acaso têm aquelas letras dentro do nome.
//
// Fazer isso com strings.Contains seria o erro clássico: bloquearia sites
// legítimos de terceiros que escolheram um nome parecido.
func casamentoMaisEspecifico(dominio string, lista map[string]bool) int {
	maior := 0
	// Andamos rótulo a rótulo: "a.b.exemplo.com" → "a.b.exemplo.com",
	// "b.exemplo.com", "exemplo.com", "com". Assim só comparamos fronteiras
	// de ponto de verdade.
	for parte := dominio; parte != ""; {
		if lista[parte] && len(parte) > maior {
			maior = len(parte)
		}
		i := strings.Index(parte, ".")
		if i < 0 {
			break
		}
		parte = parte[i+1:]
	}
	return maior
}
