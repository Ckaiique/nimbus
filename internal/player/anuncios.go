// Liga o bloqueador de anúncios em cada navegador embutido.
//
// Aqui é só a "fiação": quem decide o que é anúncio é o pacote
// `internal/adblock` (uma função pura, com testes). Este arquivo apenas
// conversa com o WebView2:
//
//  1. pede para o navegador AVISAR a cada coisa que a página for buscar;
//  2. quando o endereço é de anúncio, devolve uma resposta vazia no lugar de
//     deixar o pedido sair para a internet;
//  3. injeta, em toda página, o script que esconde os espaços de anúncio e
//     pula o anúncio do YouTube.
//
// ⚠️ HONESTIDADE: o passo 2 resolve banner e rastreador, mas NÃO resolve o
// anúncio de vídeo do YouTube — ele vem do mesmo servidor que entrega o vídeo
// que você quer ver, então bloquear por endereço tiraria o vídeo junto. Para
// esse caso quem trabalha é o passo 3, e o YouTube muda as regras dele com
// frequência: pode parar de funcionar sem aviso.
package player

import (
	"github.com/jchv/go-webview2/pkg/edge"

	"nimbus/internal/adblock"
)

// BloquearAnuncios é a opção "Bloquear anuncios" da aba Config. Começa LIGADA
// porque é o que o dono do PC quer no dia a dia; quem não quiser desmarca.
//
// A troca vale NA HORA, sem recarregar página nenhuma: o filtro consulta esta
// variável a cada pedido, e o script da página consulta a chave que o
// DefinirBloqueioDeAnuncios manda.
var BloquearAnuncios = true

// prepararBloqueio arma o bloqueador num navegador recém-criado.
//
// Tem de ser chamada DEPOIS do Embed (é ele que cria o navegador por dentro) e
// uma vez só por navegador — o filtro e o script ficam registrados para sempre;
// ligar/desligar depois é assunto do DefinirBloqueioDeAnuncios.
func prepararBloqueio(nav *edge.Chromium) {
	if nav == nil {
		return
	}

	// Quem responde a cada pedido da página. Precisa ser definido ANTES do
	// filtro: o filtro é que faz os avisos começarem a chegar.
	nav.WebResourceRequestedCallback = func(
		pedido *edge.ICoreWebView2WebResourceRequest,
		resposta *edge.ICoreWebView2WebResourceRequestedEventArgs,
	) {
		decidirPedido(nav, pedido, resposta)
	}

	// "*" com o contexto ALL = queremos ser avisados de TUDO (imagem, script,
	// vídeo, pedido de rede...). É o filtro mais largo possível; a peneira fina
	// é a nossa, do lado de cá, e ela é barata (uma consulta a um mapa).
	nav.AddWebResourceRequestedFilter("*", edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_ALL)

	// O script que limpa a página, registrado para rodar em TODA página, antes
	// do conteúdo do site. Logo em seguida vai a chave dizendo se ele está
	// ligado agora — porque o navegador pode nascer com a opção já desmarcada.
	nav.Init(adblock.ScriptDeLimpeza())
	nav.Init(adblock.ChaveDeDesligar(BloquearAnuncios))
}

// decidirPedido é chamada pelo navegador a CADA coisa que a página pede.
//
// Ela precisa ser rápida e, acima de tudo, silenciosa em caso de erro: qualquer
// tropeço aqui tem de terminar em "deixa passar". Um anúncio a mais é chato;
// uma página que não carrega é o programa parecendo quebrado.
func decidirPedido(
	nav *edge.Chromium,
	pedido *edge.ICoreWebView2WebResourceRequest,
	resposta *edge.ICoreWebView2WebResourceRequestedEventArgs,
) {
	if !BloquearAnuncios || nav == nil || pedido == nil || resposta == nil {
		return
	}

	endereco, err := pedido.GetUri()
	if err != nil || endereco == "" {
		return
	}
	if !adblock.DeveBloquear(endereco) {
		return
	}

	ambiente := nav.Environment()
	if ambiente == nil {
		return
	}

	// Uma resposta VAZIA com código 403 ("recusado"). Não devolvemos erro de
	// rede porque muitos sites ficam tentando de novo sem parar quando a
	// conexão parece ter falhado; uma recusa clara e curta encerra o assunto.
	vazia, err := ambiente.CreateWebResourceResponse(nil, 403, "Bloqueado pelo Nimbus", "")
	if err != nil || vazia == nil {
		return
	}
	_ = resposta.PutResponse(vazia)
}

// DefinirBloqueioDeAnuncios liga ou desliga o bloqueador — e vale na hora, em
// todos os navegadores abertos, sem recarregar nada.
//
// São duas metades, e as duas precisam ser avisadas:
//
//	o filtro de endereços  -> lê a variável BloquearAnuncios a cada pedido;
//	o script dentro da página -> lê a chave que mandamos aqui embaixo.
//
// O Eval avisa a página que já está aberta; o Init avisa as PRÓXIMAS (o script
// principal não pode ser "desregistrado", mas ele consulta essa chave a cada
// volta, e o último Init registrado é o que vale para a página seguinte).
func DefinirBloqueioDeAnuncios(ligado bool) {
	if ligado == BloquearAnuncios {
		return
	}
	BloquearAnuncios = ligado

	chave := adblock.ChaveDeDesligar(ligado)
	for _, inst := range instancias {
		if inst == nil || inst.nav == nil {
			continue
		}
		inst.nav.Eval(chave)
		inst.nav.Init(chave)
	}
}
