# internal/adblock — o bloqueador de anúncios (LÓGICA)

Decide o que é anúncio/rastreador nos sites abertos **dentro do Nimbus**
(YouTube, YT Music, Netflix, Disney+). Ele não fala com o Windows nem com o
navegador: só responde "bloqueia" ou "deixa passar", e entrega o texto do
script que limpa a página. Quem executa é o `internal/player`.

| Arquivo | O que faz |
|---|---|
| `adblock.go` | A decisão: `DeveBloquear(url)` e `DominioDaURL(url)`. Função pura, sem estado — é o que permite testar tudo sem abrir navegador. |
| `listas.go` | As duas listas de domínios embutidas no código: a de **bloqueio** e a de **proteção** (o que nunca pode cair). |
| `limpeza.go` | O CSS/JavaScript injetado em toda página: esconde os espaços de anúncio e **pula o anúncio do YouTube**. |
| `adblock_test.go` | Os testes — inclusive dos erros clássicos que este tipo de código comete. |

## Como funciona, em duas frentes

**1. Não buscar o anúncio.** O navegador avisa o Nimbus a cada coisa que a
página vai buscar (imagem, script, pedido de rede). Se o endereço for de um
servidor de publicidade, respondemos com uma página vazia e o pedido nunca sai
para a internet. É o que corta banner, pop-up e pixel de rastreamento.

**2. Limpar o que sobra, dentro da página.** Um script roda em toda página, de
700 em 700 milissegundos: esconde o retângulo vazio que ficou no lugar do
anúncio e, no YouTube, clica no botão **"Pular anúncio"** assim que ele
aparece.

## Casamento de domínio: por rótulo, nunca "contém o texto"

Esta é a parte que mais dá errado em bloqueador caseiro. `googlesyndication.com`
**precisa** pegar `pagead2.googlesyndication.com` (é subdomínio dele), mas
**não pode** pegar `naogoogle-analytics.com.br` — que só por acaso tem aquelas
letras dentro do nome, e é o site de outra pessoa.

Por isso a comparação anda de ponto em ponto (`a.b.exemplo.com` →
`b.exemplo.com` → `exemplo.com` → `com`) e só aceita fronteira de ponto de
verdade. Tem teste para os dois casos.

## A trava: a lista de protegidos

Existe uma segunda lista com os domínios que **nunca** podem ser bloqueados —
`youtube.com`, `googlevideo.com`, `netflix.com`, `nflxvideo.net`,
`disneyplus.com`, `dssott.com` e companhia.

Não é otimização, é segurança: bloquear `googlevideo.com` por engano **não
tiraria um anúncio, apagaria o vídeo inteiro** — e o dono veria uma tela preta
sem entender por quê.

Quando as duas listas casam ao mesmo tempo, **vence a mais específica**. É o que
permite proteger `google.com` (login, fontes, APIs) e ainda assim barrar
`adservice.google.com`.

## O que este bloqueador FAZ e o que NÃO FAZ

Sendo honesto, para ninguém esperar o que ele não entrega:

**Faz bem:**

- corta banners, pop-ups e a maior parte da publicidade comum da web;
- corta os rastreadores mais conhecidos (medição de audiência, perfis de
  usuário vendidos entre empresas);
- esconde o buraco que o anúncio bloqueado deixa na página;
- no YouTube, **pula o anúncio de vídeo sozinho**.

**Não faz:**

- **não bloqueia o anúncio de vídeo do YouTube por endereço** — e isso não é
  preguiça: o anúncio vem do **mesmo servidor** que entrega o vídeo que você
  quer ver (`googlevideo.com`). Não existe endereço para separar um do outro.
  Por isso a saída é pular, não bloquear;
- **não garante que o "pular sozinho" vá funcionar sempre.** O YouTube muda a
  técnica com frequência e sabe detectar quem tenta pular. Isso é uma corrida
  sem fim, e não vamos ganhar dela;
- **não se atualiza sozinho.** A lista é embutida no código, de propósito: o
  Nimbus tem de funcionar offline e sem depender do servidor de ninguém. Em
  troca, ela pega o grosso — não pega tudo;
- **não é um uBlock Origin.** Não tem filtros por caminho, nem por regra
  cosmética avançada, nem lista de exceções por site. A proposta é ser útil no
  dia a dia do dono do PC, no navegador dele.

## Como acrescentar um domínio

Abra `listas.go` e escreva o **nome principal** numa linha da lista (ex.:
`"exemplo-de-anuncio.com"`). Todos os subdomínios vêm juntos automaticamente.
Rode `go test ./internal/adblock/` depois — os testes conferem se o domínio foi
escrito de um jeito que realmente casa com alguma coisa.
