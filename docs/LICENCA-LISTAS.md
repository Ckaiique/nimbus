# Licença das listas de anúncios

> **Resumo em uma frase:** o **código** do Nimbus é MIT (faça o que quiser);
> as **listas de domínios** não são nossas — vêm da EasyList, são
> **CC BY-SA 3.0 / GPLv3**, e exigem que a origem seja creditada.

## Por que isto tem um arquivo só para si

O Nimbus bloqueia anúncios usando listas de domínios feitas e mantidas **por
outras pessoas**, de graça, há mais de quinze anos. Nós não escrevemos essas
listas: nós as **usamos**. Quem as mantém pede uma coisa em troca — que a
origem seja dita com clareza e que quem repassar o trabalho mantenha as mesmas
condições. É o mínimo, e este arquivo existe para cumprir isso direito.

Não é burocracia: é a diferença entre usar o trabalho de alguém e se apropriar
dele.

## As listas usadas

| Lista | Endereço oficial | Para que serve aqui |
|---|---|---|
| **EasyList** | <https://easylist.to/easylist/easylist.txt> | A lista principal de publicidade. É a base de quase todo bloqueador de anúncios do mundo. |
| **EasyPrivacy** | <https://easylist.to/easylist/easyprivacy.txt> | A lista de rastreadores: medição de audiência, perfis de usuário, pixels de acompanhamento. |
| **EasyList Portuguese** | <https://easylist-downloads.adblockplus.org/easylistportuguese.txt> | O complemento para sites em português, que a lista principal não cobre bem. |

- **Página do projeto:** <https://easylist.to/>
- **Página da licença:** <https://easylist.to/pages/licence.html>
- **Repositório:** <https://github.com/easylist/easylist>

## A licença

As listas da EasyList são distribuídas sob **duas** licenças, à escolha de quem
usa:

- **Creative Commons Attribution-ShareAlike 3.0 Unported (CC BY-SA 3.0)** —
  <https://creativecommons.org/licenses/by-sa/3.0/>
- **GNU General Public License v3 (GPLv3)** —
  <https://www.gnu.org/licenses/gpl-3.0.html>

O que as duas exigem, em português simples:

1. **Atribuição** — dizer de onde a lista veio;
2. **Compartilhar igual** — se você modificar a lista e distribuir a versão
   modificada, ela continua sob a mesma licença.

## Como o Nimbus cumpre isso

- O arquivo gerado (`internal/adblock/dados/easylist-dominios.txt`) traz, **no
  próprio cabeçalho**, a data, o endereço de cada lista de origem e o aviso de
  licença. Quem abrir o arquivo lê a atribuição na primeira tela.
- O `README.md` da raiz, o `internal/adblock/README.md` e o `LICENSE` dizem a
  mesma coisa, cada um no seu lugar.
- A lista que o Nimbus guarda **é uma versão modificada** (nós filtramos: só as
  regras de domínio inteiro entram, e removemos o que é redundante ou protegido).
  Por isso ela continua sob CC BY-SA / GPLv3 — a licença acompanha o arquivo,
  não o programa.

## Isso "contamina" o código do Nimbus?

**Não.** O código continua MIT.

O motivo é simples: a lista é **dado**, não código. Ela entra no `.exe` do mesmo
jeito que uma imagem ou um texto de ajuda entrariam — como conteúdo, não como
parte do programa. As duas licenças convivem no mesmo pacote, cada uma
cobrindo a sua parte:

| Parte | Licença | Quem fez |
|---|---|---|
| Código do Nimbus (`.go`, `.bat`, documentação) | MIT | KST |
| `internal/adblock/dados/easylist-dominios.txt` | CC BY-SA 3.0 / GPLv3 | Projeto EasyList |
| Lista curta escrita à mão (`internal/adblock/listas.go`) | MIT | KST |

Se um dia você distribuir o Nimbus, **mantenha o cabeçalho do arquivo de listas
e este documento junto**. É só isso que é pedido.

## Se algum dia trocar de lista

Ao acrescentar ou trocar uma fonte em `internal/adblock/arquivo.go` (variável
`Fontes`), **atualize este arquivo na mesma mudança**: nova lista, nova
licença, nova atribuição. Lista nenhuma entra sem passar por aqui.
