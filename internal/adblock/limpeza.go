// A limpeza que roda DENTRO da página: esconder espaços de anúncio e pular o
// anúncio do YouTube.
//
// ─── Por que precisa disto, se já existe o bloqueio por endereço ──────────
//
// Barrar o endereço impede o anúncio de CHEGAR, mas o buraco onde ele ia ficar
// continua lá: aquele retângulo vazio no meio do texto. Um pouco de CSS resolve.
//
// E tem o caso que endereço nenhum resolve: o anúncio de vídeo do YouTube vem
// do MESMO servidor que entrega o vídeo que você quer ver. Não existe endereço
// para bloquear — bloquear derrubaria o vídeo junto. O que dá para fazer é o
// que uma pessoa faria: esperar aparecer o botão "Pular anúncio" e clicar nele.
// É isso que o script abaixo faz.
//
// ─── O que esperar (sem prometer demais) ──────────────────────────────────
//
// O YouTube muda essas coisas com frequência (nome dos botões, jeito de tocar o
// anúncio) e sabe detectar quem tenta pular. Um dia isso funciona, no outro
// pode parar de funcionar, e às vezes o site reclama. É uma corrida sem fim, e
// nós não vamos ganhar dela — a proposta aqui é ajudar no dia a dia, não ser um
// bloqueador profissional.
package adblock

// ScriptDeLimpeza devolve o JavaScript que o navegador roda em TODA página,
// antes do conteúdo do site (o "AddScriptToExecuteOnDocumentCreated" do
// WebView2).
//
// Três cuidados que estão dentro dele, e o porquê de cada um:
//
//  1. Ele NÃO trava a página: em vez de vigiar cada mudança do HTML (o que num
//     site pesado como o YouTube dispara milhares de vezes por segundo), ele
//     acorda de tempos em tempos, dá uma olhada e volta a dormir.
//  2. Ele só mexe no vídeo QUANDO O PLAYER DIZ QUE ESTÁ PASSANDO ANÚNCIO
//     (a marca "ad-showing"). Sem essa trava, ele adiantaria o filme que você
//     está assistindo até o fim — que é o pior estrago possível.
//  3. Ele pode ser DESLIGADO em tempo real: a cada volta consulta a chave
//     window.__nimbusAdblockDesligado, que o Nimbus define quando o dono
//     desmarca a opção na aba Config.
func ScriptDeLimpeza() string { return scriptDeLimpeza }

// intervaloMs é de quanto em quanto tempo o script acorda.
//
// 700ms é um meio-termo pensado: rápido o bastante para o botão "Pular" ser
// clicado quase na hora em que aparece (ele só fica disponível depois de ~5s de
// anúncio), e devagar o bastante para não custar nada de processador.
const intervaloMs = 700

// cssEsconder são os espaços de anúncio que escondemos.
//
// A escolha é PROPOSITALMENTE conservadora: só entram nomes que praticamente só
// existem para anúncio ("adsbygoogle", "div-gpt-ad", os blocos de anúncio do
// YouTube). Nomes genéricos demais, como a classe ".ad" ou ".ads", ficaram DE
// FORA — eles aparecem em sites por outros motivos (de "adicionar" a "adaptado")
// e esconder tudo isso quebraria páginas honestas.
const cssEsconder = `
ins.adsbygoogle,
iframe[id^="google_ads_iframe"],
iframe[src*="doubleclick.net"],
iframe[src*="googlesyndication.com"],
div[id^="div-gpt-ad"],
div[id^="google_ads"],
.adsbygoogle,
.adsbox,
.ad-banner,
.ad-container,
.ad-wrapper,
.ad-slot,
.advertisement,
.advert-container,
.taboola-container,
.OUTBRAIN,
#player-ads,
.ytp-ad-overlay-slot,
.ytp-ad-overlay-container,
ytd-ad-slot-renderer,
ytd-display-ad-renderer,
ytd-promoted-video-renderer,
ytd-promoted-sparkles-web-renderer,
ytd-in-feed-ad-layout-renderer,
ytd-banner-promo-renderer,
ytd-statement-banner-renderer,
ytmusic-mealbar-promo-renderer {
  display: none !important;
}
`

// scriptDeLimpeza é montado uma vez só, no início do programa.
//
// ⚠️ Escrito com ASPAS SIMPLES no JavaScript de propósito: o texto todo mora
// dentro de um literal Go entre crases, e uma crase perdida aqui dentro
// encerraria o literal e quebraria a compilação.
var scriptDeLimpeza = `(function () {
  'use strict';

  // Se por algum motivo o script for injetado duas vezes na mesma página,
  // a segunda desiste — senão ficariam dois relógios fazendo a mesma coisa.
  if (window.__nimbusLimpezaLigada) { return; }
  window.__nimbusLimpezaLigada = true;

  var CSS = ` + "`" + cssEsconder + "`" + `;
  var ID_ESTILO = 'nimbus-adblock-css';

  function desligado() {
    return window.__nimbusAdblockDesligado === true;
  }

  // Coloca (ou tira) a folha de estilo que esconde os espaços de anúncio.
  // "Tira" importa: se o dono desmarcar a opção, a página volta ao normal na
  // hora, sem precisar recarregar.
  function cuidarDoEstilo() {
    var alvo = document.head || document.documentElement;
    if (!alvo) { return; }
    var estilo = document.getElementById(ID_ESTILO);
    if (desligado()) {
      if (estilo && estilo.parentNode) { estilo.parentNode.removeChild(estilo); }
      return;
    }
    if (estilo) { return; }
    estilo = document.createElement('style');
    estilo.id = ID_ESTILO;
    estilo.textContent = CSS;
    alvo.appendChild(estilo);
  }

  // ─── O pulo do anúncio do YouTube ──────────────────────────────────────
  //
  // A ordem aqui é a de uma pessoa apressada:
  //   1. tem botão de pular? clica.
  //   2. não tem ainda? adianta o anúncio até o fim, que é quando o botão
  //      costuma liberar.
  //   3. tem aquele banner por cima do vídeo? fecha.
  function pularAnuncioDoYoutube() {
    var player = document.querySelector('.html5-video-player');
    if (!player) { return; }

    // ⚠️ A TRAVA MAIS IMPORTANTE DESTE ARQUIVO.
    // Só seguimos se o PRÓPRIO player disser que está passando anúncio. Sem
    // isto, o passo 2 adiantaria o vídeo de verdade até o final.
    var passandoAnuncio = player.classList.contains('ad-showing') ||
                          player.classList.contains('ad-interrupting');
    if (!passandoAnuncio) { return; }

    var botao = player.querySelector(
      '.ytp-ad-skip-button, .ytp-ad-skip-button-modern, ' +
      '.ytp-skip-ad-button, button[class*="ytp-ad-skip-button"]');
    if (botao) {
      botao.click();
      return;
    }

    // Adianta o anúncio. Só mexe se a duração for um número de verdade:
    // enquanto o anúncio está carregando ela vem como NaN ou Infinity, e
    // atribuir isso ao vídeo dá erro no console.
    var video = player.querySelector('video');
    if (video && isFinite(video.duration) && video.duration > 0) {
      if (video.currentTime < video.duration - 0.2) {
        video.currentTime = video.duration;
      }
      // Tocar o anúncio no talo faz ele acabar mesmo quando adiantar não pega.
      if (video.paused) { try { video.play(); } catch (e) {} }
    }

    var fechar = player.querySelector('.ytp-ad-overlay-close-button');
    if (fechar) { fechar.click(); }
  }

  function umaVolta() {
    try {
      cuidarDoEstilo();
      if (desligado()) { return; }
      pularAnuncioDoYoutube();
    } catch (e) {
      // Nunca deixamos um erro nosso vazar para a página: o site não tem
      // nada a ver com isso, e um erro solto poderia atrapalhar o que ELE
      // está fazendo.
    }
  }

  umaVolta();
  setInterval(umaVolta, ` + itoa(intervaloMs) + `);
})();`

// itoa converte um número pequeno em texto sem puxar o pacote strconv só para
// isto (o script é montado uma vez, no início do programa).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// ChaveDeDesligar devolve o JavaScript de uma linha que liga ou desliga a
// limpeza numa página que JÁ está aberta.
//
// Por que existe: o script principal é registrado quando o navegador nasce e
// não dá para "desregistrar". Em vez disso ele consulta esta chave a cada
// volta — então trocar a chave desliga (ou religa) tudo na hora, sem recarregar
// a página e sem interromper a música que estiver tocando.
func ChaveDeDesligar(ligado bool) string {
	if ligado {
		return "window.__nimbusAdblockDesligado=false;"
	}
	return "window.__nimbusAdblockDesligado=true;"
}
