// As duas listas de domínios: o que barrar e o que NUNCA barrar.
//
// ─── De onde saiu esta lista ──────────────────────────────────────────────
//
// Ela foi escrita à mão, escolhendo os servidores de publicidade e de
// rastreamento mais comuns na web — os mesmos nomes que aparecem no topo de
// qualquer lista pública do gênero (EasyList, EasyPrivacy, Peter Lowe's list) e
// nos "quem está te seguindo" que os navegadores mostram.
//
// O critério de escolha foi deliberadamente CONSERVADOR: só entrou domínio cuja
// razão de existir é servir anúncio, medir audiência ou seguir usuário. Nada de
// domínio de uso misto (por exemplo, o `gstatic.com` do Google, que serve
// arquivos de sites inteiros) — bloquear um desses quebraria páginas legítimas,
// e o dono ficaria com a impressão de que o Nimbus está com defeito.
//
// ⚠️ A lista NÃO se atualiza sozinha (é embutida, nada é baixado). Ela pega o
// grosso do que incomoda; não pega tudo. Para acrescentar um domínio, escreva
// só o "nome principal" — todos os subdomínios dele já vêm juntos.
package adblock

// dominiosBloqueados: servidores de anúncio e de rastreamento.
//
// Escreva sempre o domínio "de cima" (ex.: "doubleclick.net"). O casamento é
// por rótulo, então "googleads.g.doubleclick.net" já está incluído.
//
// É um map (e não uma lista) porque a consulta acontece a CADA pedido da
// página — centenas por site. Map é busca direta; percorrer uma lista de cem
// nomes a cada imagem seria desperdício.
var dominiosBloqueados = paraConjunto([]string{
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

// QuantosDominios diz quantos domínios a lista de bloqueio tem. Serve para a
// interface e para a documentação não ficarem com um número escrito na mão que
// desatualiza sem ninguém perceber.
func QuantosDominios() int { return len(dominiosBloqueados) }
