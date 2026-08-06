// As duas listas de domínios ESCRITAS À MÃO: a reserva e a de proteção.
//
// ─── O que é a "reserva" ──────────────────────────────────────────────────
//
// Estes ~120 domínios eram, até pouco tempo atrás, o bloqueador inteiro. Hoje o
// grosso vem das listas públicas da EasyList (mais de 100 mil domínios, veja
// `carregar.go`), e esta lista curta passou a ter dois papéis:
//
//  1. **rede de segurança** — se o arquivo grande vier corrompido ou cortado
//     no meio, o Nimbus cai aqui em vez de ficar sem bloquear nada. É a regra
//     da casa ("a tela nunca fica em branco") aplicada ao bloqueador;
//  2. **complemento permanente** — ela é somada à lista grande sempre, porque
//     tem casos que a de fora não cobre do mesmo jeito. Exemplo:
//     "adservice.google.com" fica DEBAIXO de um domínio protegido
//     ("google.com"), então regras vindas de fora sobre ele são descartadas por
//     precaução — mas esta, escrita e conferida por nós, vale.
//
// O critério de escolha foi deliberadamente CONSERVADOR: só entrou domínio cuja
// razão de existir é servir anúncio, medir audiência ou seguir usuário. Nada de
// domínio de uso misto (por exemplo, o `gstatic.com` do Google, que serve
// arquivos de sites inteiros) — bloquear um desses quebraria páginas legítimas,
// e o dono ficaria com a impressão de que o Nimbus está com defeito.
package adblock

import "strings"

// dominiosReserva: servidores de anúncio e de rastreamento, escritos à mão.
//
// Escreva sempre o domínio "de cima" (ex.: "doubleclick.net"). O casamento é
// por rótulo, então "googleads.g.doubleclick.net" já está incluído.
//
// É um map (e não uma lista) porque a consulta acontece a CADA pedido da
// página — centenas por site. Map é busca direta; percorrer uma lista de cem
// nomes a cada imagem seria desperdício.
var dominiosReserva = paraConjunto([]string{
	// ── Google: publicidade e medição ─────────────────────────────────────
	// (o vídeo do YouTube NÃO passa por aqui — veja dominiosProtegidos)
	"doubleclick.net",
	"googlesyndication.com",
	"googleadservices.com",
	"adservice.google.com",
	"googletagservices.com",
	"googletagmanager.com",
	"google-analytics.com",
	"analytics.google.com",
	"2mdn.net",
	"app-measurement.com",

	// ── Medição de audiência e "análise de comportamento" ─────────────────
	"scorecardresearch.com",
	"quantserve.com",
	"quantcount.com",
	"hotjar.com",
	"mouseflow.com",
	"crazyegg.com",
	"luckyorange.com",
	"inspectlet.com",
	"clicktale.net",
	"fullstory.com",
	"heapanalytics.com",
	"mixpanel.com",
	"amplitude.com",
	"segment.com",
	"segment.io",
	"optimizely.com",
	"flurry.com",

	// ── Leilão de anúncio em tempo real (o "miolo" da publicidade online) ──
	"adnxs.com",
	"adnxs-simple.com",
	"rubiconproject.com",
	"pubmatic.com",
	"openx.net",
	"casalemedia.com",
	"smartadserver.com",
	"sharethrough.com",
	"33across.com",
	"indexww.com",
	"bidswitch.net",
	"teads.tv",
	"media.net",
	"yieldmo.com",
	"yieldlab.net",
	"sovrn.com",
	"lijit.com",
	"gumgum.com",
	"districtm.io",
	"contextweb.com",
	"mathtag.com",
	"improvedigital.com",
	"adtelligent.com",
	"smartyads.com",
	"adform.net",
	"serving-sys.com",
	"flashtalking.com",
	"sitescout.com",
	"simpli.fi",
	"zedo.com",

	// ── Redes de anúncio e "conteúdo recomendado" ─────────────────────────
	"criteo.com",
	"criteo.net",
	"taboola.com",
	"taboolasyndication.com",
	"outbrain.com",
	"outbrainimg.com",
	"revcontent.com",
	"mgid.com",
	"adroll.com",
	"advertising.com",
	"amazon-adsystem.com",
	"assoc-amazon.com",
	"media6degrees.com",
	"moatads.com",
	"undertone.com",
	"spotxchange.com",
	"spotx.tv",
	"tremorhub.com",
	"infolinks.com",
	"bidvertiser.com",
	"vidoomy.com",

	// ── Anúncio agressivo (pop-up, pop-under, redirecionamento) ───────────
	"propellerads.com",
	"popads.net",
	"adsterra.com",
	"exoclick.com",
	"juicyads.com",
	"trafficjunky.com",
	"hilltopads.net",
	"onclickads.net",
	"adcash.com",
	"clickadu.com",

	// ── Perfis de usuário / "quem é você" vendido entre empresas ──────────
	"bluekai.com",
	"demdex.net",
	"everesttech.net",
	"omtrdc.net",
	"2o7.net",
	"adobedtm.com",
	"krxd.net",
	"exelator.com",
	"rlcdn.com",
	"agkn.com",
	"tapad.com",
	"eyeota.net",
	"crwdcntrl.net",

	// ── Rastreamento das redes sociais e de instalação de aplicativo ──────
	"connect.facebook.net",
	"ads-twitter.com",
	"analytics.twitter.com",
	"bat.bing.com",
	"ads.linkedin.com",
	"ads.yahoo.com",
	"branch.io",
	"appsflyer.com",
	"adjust.com",
	"kochava.com",
	"onesignal.com",
	"pushcrew.com",

	// ── Verificação de anúncio (existe só para medir o anúncio) ───────────
	"adsafeprotected.com",
	"doubleverify.com",

	// ── Publicidade dentro de aplicativos ─────────────────────────────────
	"adcolony.com",
	"applovin.com",
	"inmobi.com",
	"vungle.com",
	"chartboost.com",
	"supersonicads.com",
	"smaato.net",
	"mopub.com",
})

// dominiosProtegidos: o que NUNCA pode ser bloqueado, aconteça o que acontecer.
//
// São os quatro serviços que o Nimbus abre, mais os servidores de onde o vídeo
// deles realmente sai. Isto é uma TRAVA DE SEGURANÇA, não uma otimização:
// bloquear "googlevideo.com" por engano não tiraria um anúncio — apagaria o
// vídeo inteiro, e o dono veria só uma tela preta sem entender o motivo.
//
// Quando um domínio daqui e um da lista de bloqueio casam ao mesmo tempo, ganha
// o mais específico (veja DeveBloquear). É o que permite proteger
// "google.com" e ainda assim barrar "adservice.google.com".
var dominiosProtegidos = paraConjunto([]string{
	// YouTube e YouTube Music
	"youtube.com",
	"youtu.be",
	"youtube-nocookie.com",
	"ytimg.com",
	"ggpht.com",
	"googlevideo.com", // é DAQUI que o vídeo sai — jamais bloquear

	// Infraestrutura do Google que os sites usam de verdade (login, fontes,
	// imagens, APIs). Nada disso é anúncio.
	"google.com",
	"gstatic.com",
	"googleapis.com",
	"googleusercontent.com",

	// Netflix
	"netflix.com",
	"nflxvideo.net",
	"nflximg.net",
	"nflxext.com",
	"nflxso.net",

	// Disney+
	"disneyplus.com",
	"disney-plus.net",
	"dssott.com",
	"bamgrid.com",
	"go.com",
	"edgedatg.com",
})

// paraConjunto transforma a lista escrita acima num map de consulta rápida.
// Existe só para a lista ficar legível (uma linha por domínio, com vírgula).
func paraConjunto(nomes []string) map[string]bool {
	m := make(map[string]bool, len(nomes))
	for _, n := range nomes {
		m[n] = true
	}
	return m
}

// ProtegidoOuSubdominio diz se o domínio é um serviço protegido — ou algo
// debaixo dele.
//
// É a trava aplicada a TUDO que vem de fora: uma regra da EasyList sobre
// "googlevideo.com", ou sobre "r5---sn-abc.googlevideo.com", nunca entra na
// nossa lista. Não é exagero: a EasyList é feita para navegador comum, onde
// bloquear um servidor de vídeo por engano estraga um site. Aqui estragaria
// justamente as quatro coisas que o Nimbus existe para mostrar.
//
// ⚠️ Isto NÃO se aplica à lista escrita à mão (dominiosReserva), que é curada
// por nós e usa o desempate por especificidade — é o que mantém
// "adservice.google.com" bloqueado com "google.com" protegido.
func ProtegidoOuSubdominio(dominio string) bool {
	for parte := dominio; parte != ""; {
		if dominiosProtegidos[parte] {
			return true
		}
		i := strings.Index(parte, ".")
		if i < 0 {
			return false
		}
		parte = parte[i+1:]
	}
	return false
}

// DominiosProtegidos devolve, em ordem qualquer, os domínios que nunca podem
// ser bloqueados. Existe para o gerador de listas poder aplicar a mesma trava
// que o programa aplica — sem precisar copiar a lista para outro lugar (cópia
// de lista é cópia que um dia fica desatualizada).
func DominiosProtegidos() []string {
	fora := make([]string, 0, len(dominiosProtegidos))
	for d := range dominiosProtegidos {
		fora = append(fora, d)
	}
	return fora
}
