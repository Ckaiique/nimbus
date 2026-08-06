# gerar-listas — atualiza a lista de anúncios que vai DENTRO do .exe

Baixa as listas públicas da EasyList, joga fora tudo que o nosso filtro não sabe
aplicar e reescreve o arquivo `internal/adblock/dados/easylist-dominios.txt`.

## Como usar

Na pasta do projeto (a que tem o `go.mod`):

```powershell
go run ./ferramentas/gerar-listas
```

Depois **recompile** (`compilar.bat`) — o arquivo entra no `.exe` na
compilação.

Saída esperada, mais ou menos assim:

```
baixando EasyList ...
  2204 KB
baixando EasyPrivacy ...
  1457 KB
baixando EasyList Portuguese ...
  72 KB

102318 dominios distintos vindos das listas
-    0 por excecao da propria EasyList
-   22 por estarem sob um dominio protegido do Nimbus
-  272 por redundancia (o dominio de cima ja bloqueia)
=102024 dominios no arquivo, mais 38 excecoes

gravado em internal/adblock/dados/easylist-dominios.txt (2010 KB)
```

## Isto é a única coisa que baixa da internet?

Não — mas é a única que baixa **para dentro do repositório**.

Existem dois caminhos, e é bom não confundir:

| Quem | Quando roda | Onde guarda |
|---|---|---|
| **Esta ferramenta** | à mão, por quem mexe no projeto | no repositório (vai para dentro do `.exe`) |
| **`internal/listas`** | sozinho, no PC do dono, a cada 7 dias | na pasta de dados do usuário (`%LOCALAPPDATA%\Nimbus`) |

Os dois usam **exatamente o mesmo código de conversão**, que mora em
`internal/adblock` (`easylist.go` e `arquivo.go`). Se fossem dois códigos
parecidos, um dia eles divergiriam e a lista gerada à mão passaria a ser
diferente da baixada sozinha — o tipo de diferença que ninguém percebe até
alguma coisa quebrar.

**Nenhum teste do projeto baixa nada.** O download acontece só quando alguém
roda esta ferramenta, ou quando o Nimbus se atualiza no PC do dono.

## O que entra e o que fica de fora

Entram só as regras de **domínio inteiro** (`||dominio.com^`), com ou sem opções
depois do `$`, e as **exceções** (`@@||dominio.com^`).

Ficam de fora, e cada motivo importa:

| Tipo de regra | Por que não entra |
|---|---|
| Cosmética (`##.banner`) | Esconde um pedaço da página com CSS; não tem endereço para bloquear |
| Por caminho (`\|\|site.com/anuncios/*`) | Bloquear o domínio inteiro por causa dela **derrubaria o site** |
| Expressão regular (`/banner\d+/`) | Não sabemos avaliar |
| Com `$domain=` | Ela vale **só dentro de certos sites**. Aplicar como bloqueio geral é inventar uma regra que ninguém escreveu — e é assim que nasce o falso positivo |
| Com `$redirect=`, `$removeparam=` | Transformam o pedido em vez de barrar; não sabemos fazer isso |

Depois disso vêm três peneiras (todas explicadas em
`internal/adblock/arquivo.go`):

1. **exceções** da própria EasyList;
2. **a trava dos protegidos** — nada que fique sob `googlevideo.com`,
   `netflix.com` e companhia entra, aconteça o que acontecer;
3. **poda do redundante** — se `exemplo.com` já está na lista,
   `ads.exemplo.com` é jogado fora (o casamento é por rótulo, ele já cairia
   junto).

## Licença

⚠️ As listas são da **EasyList** (CC BY-SA 3.0 / GPLv3), não nossas. O arquivo
gerado leva o aviso no cabeçalho. Leia `docs/LICENCA-LISTAS.md` antes de mexer
nas fontes.
