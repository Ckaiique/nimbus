// De onde sai a lista de domínios que o bloqueador usa — e a ordem de socorro
// quando alguma coisa dá errado.
//
// ─── TRÊS ORIGENS, NESTA ORDEM ────────────────────────────────────────────
//
//  1. **baixada** — o arquivo que o atualizador automático guardou na pasta de
//     dados do usuário (`internal/listas`). É o mais novo, então ganha;
//  2. **embutida** — o arquivo que veio dentro do .exe, gerado antes de
//     compilar. É o que vale no primeiro uso, e num PC que nunca viu internet;
//  3. **reserva** — os ~120 domínios escritos à mão em `listas.go`. Só entram
//     em cena se os dois de cima falharem (arquivo cortado no meio, vazio,
//     adulterado).
//
// A regra da casa do projeto é "a tela nunca fica em branco". Aqui ela vira:
// **o bloqueador nunca fica sem lista nenhuma**. Um bloqueador que silenciosamente
// para de bloquear é pior que um que bloqueia pouco: ninguém percebe.
//
// ─── A TRAVA QUE VALE PARA TODAS AS ORIGENS ───────────────────────────────
//
// Venha de onde vier, nenhum domínio PROTEGIDO pode ser bloqueado. A peneira
// acontece na leitura do arquivo (veja lerNossoFormato) e de novo na decisão
// (veja DeveBloquear). É de propósito que sejam dois lugares: a lista baixada
// vem da internet, e código que confia em dado da internet é código que um dia
// apaga o vídeo da pessoa por engano.
package adblock

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// listaEmbutida é o arquivo gerado que viaja DENTRO do .exe.
//
// Por que embutido (`go:embed`) e não lido do disco como as imagens de
// `assets/`: as imagens são enfeite — se faltarem, o Nimbus desenha um ícone de
// reserva e ninguém se machuca. A lista de bloqueio é comportamento: se ela
// sumisse da pasta, o programa passaria a deixar anúncio entrar sem avisar.
// Dentro do .exe ela não tem como sumir, o programa continua sendo UM arquivo
// só para copiar, e a promessa de "funciona offline" fica garantida.
//
//go:embed dados/easylist-dominios.txt
var listaEmbutida []byte

// Nomes das origens possíveis. Aparecem na aba Config, então são texto que o
// dono lê — nada de jargão.
const (
	OrigemBaixada  = "baixada"
	OrigemEmbutida = "embutida"
	OrigemReserva  = "reserva"
)

// minimoDeDominios é o quanto um arquivo de listas precisa render para ser
// levado a sério. Abaixo disso ele é considerado incompleto e caímos na origem
// seguinte.
//
// O número é folgado de propósito: as listas de verdade passam de 100 mil
// domínios, então 1000 nunca barra um arquivo bom — mas barra um download
// cortado no meio e uma página de erro em HTML, que é o que interessa.
const minimoDeDominios = 1000

// Lista é um conjunto de domínios já pronto para consulta.
//
// Ela é IMUTÁVEL depois de montada: para trocar de lista, monta-se outra e
// troca-se o ponteiro inteiro (veja Usar). Assim o navegador pode estar
// consultando a antiga no exato instante em que a nova chega, sem trava
// nenhuma e sem risco de ler um mapa pela metade.
type Lista struct {
	bloquear map[string]bool
	excecoes map[string]bool

	// Origem é "baixada", "embutida" ou "reserva".
	Origem string
	// Gerada é quando o arquivo foi montado (lido do cabeçalho dele). Fica
	// zerada quando não dá para saber — no caso da reserva, por exemplo.
	Gerada time.Time
}

// Quantos diz quantos domínios esta lista bloqueia.
func (l *Lista) Quantos() int {
	if l == nil {
		return 0
	}
	return len(l.bloquear)
}

var (
	// atual é a lista em uso. Ponteiro atômico porque ela é lida a cada pedido
	// do navegador (centenas por página) e trocada raríssimas vezes.
	atual atomic.Pointer[Lista]
	// trava serializa só a TROCA de lista (nunca a leitura).
	trava sync.Mutex
)

// listaEmUso devolve a lista atual, carregando-a na primeira vez que alguém
// precisa.
//
// O carregamento é PREGUIÇOSO (só na primeira consulta) de propósito: ele
// custa algumas dezenas de milissegundos e não pode entrar no caminho de abrir
// o programa. Quando ele acontece, a janela já está de pé há muito tempo.
func listaEmUso() *Lista {
	if l := atual.Load(); l != nil {
		return l
	}

	trava.Lock()
	defer trava.Unlock()
	// Conferir de novo com a trava na mão: outra goroutine pode ter carregado
	// (ou o atualizador pode ter trazido a lista baixada) enquanto esperávamos.
	if l := atual.Load(); l != nil {
		return l
	}

	l := doEmbutido()
	atual.Store(l)
	return l
}

// doEmbutido monta a lista a partir do arquivo que veio no .exe — e cai na
// reserva se ele não prestar.
func doEmbutido() *Lista {
	bloquear, excecoes, err := lerNossoFormato(listaEmbutida, minimoDeDominios)
	if err != nil {
		// Sem log barulhento: o programa continua funcionando com a reserva.
		// Quem quiser investigar tem o Origem() na aba Config dizendo
		// "reserva", que já é o sinal de que algo saiu do esperado.
		return daReserva()
	}
	juntarReserva(bloquear)
	return &Lista{
		bloquear: bloquear,
		excecoes: excecoes,
		Origem:   OrigemEmbutida,
		Gerada:   DataDoArquivo(listaEmbutida),
	}
}

// daReserva monta a lista com os domínios escritos à mão. É o último degrau:
// pouca coisa, mas os servidores de anúncio mais comuns da web.
func daReserva() *Lista {
	bloquear := make(map[string]bool, len(dominiosReserva))
	juntarReserva(bloquear)
	return &Lista{
		bloquear: bloquear,
		excecoes: map[string]bool{},
		Origem:   OrigemReserva,
	}
}

// juntarReserva acrescenta os domínios escritos à mão a qualquer lista.
//
// Por que a reserva entra SEMPRE, e não só quando o arquivo falha: ela é
// curada por nós, com casos que a EasyList não cobre do mesmo jeito — por
// exemplo "adservice.google.com", que fica DEBAIXO de um domínio protegido
// ("google.com") e por isso é descartado quando vem de fora. A lista de fora é
// grande; a nossa é confiável. As duas juntas são melhores que qualquer uma
// sozinha.
func juntarReserva(bloquear map[string]bool) {
	for d := range dominiosReserva {
		bloquear[d] = true
	}
}

// Usar troca a lista em uso pelo conteúdo de um arquivo no nosso formato.
//
// É o que o atualizador automático chama depois de baixar e conferir. Se o
// conteúdo não prestar, devolve erro e **nada é trocado** — a lista que já
// estava valendo continua valendo. Perder a lista boa por causa de um download
// ruim seria o pior resultado possível.
func Usar(dados []byte, origem string) error {
	return usar(dados, origem, minimoDeDominios)
}

// usar é o miolo da troca, com o mínimo aceitável como parâmetro. Só existe
// separado para os testes poderem trabalhar com listas de brinquedo, de três
// linhas, sem baixar 2 MB.
func usar(dados []byte, origem string, minimo int) error {
	bloquear, excecoes, err := lerNossoFormato(dados, minimo)
	if err != nil {
		return err
	}
	juntarReserva(bloquear)

	trava.Lock()
	defer trava.Unlock()
	atual.Store(&Lista{
		bloquear: bloquear,
		excecoes: excecoes,
		Origem:   origem,
		Gerada:   DataDoArquivo(dados),
	})
	return nil
}

// UsarArquivo é o mesmo que Usar, lendo de um arquivo do disco.
//
// Arquivo que não existe devolve erro — e quem chama simplesmente segue com a
// lista embutida. Não existir é o caso NORMAL na primeira vez que o Nimbus
// roda num PC.
func UsarArquivo(caminho string) error {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return fmt.Errorf("ler a lista guardada: %w", err)
	}
	return Usar(dados, OrigemBaixada)
}

// EmUso conta qual lista está valendo agora — para a aba Config mostrar.
func EmUso() *Lista { return listaEmUso() }

// QuantosDominios diz quantos domínios a lista de bloqueio tem. Serve para a
// interface e a documentação não ficarem com um número escrito na mão que
// desatualiza sem ninguém perceber.
func QuantosDominios() int { return listaEmUso().Quantos() }

// DataDoArquivo lê a linha "# Gerado em: ..." do cabeçalho.
//
// Serve para a aba Config poder dizer "listas de 3 dias atrás" em vez de um
// silêncio. Se não achar ou não entender, devolve data zerada — é informação
// de enfeite, não pode derrubar nada.
func DataDoArquivo(dados []byte) time.Time {
	inicio := dados
	if len(inicio) > 4096 {
		inicio = inicio[:4096]
	}
	const marca = "# Gerado em: "
	i := strings.Index(string(inicio), marca)
	if i < 0 {
		return time.Time{}
	}
	resto := string(inicio[i+len(marca):])
	if j := strings.IndexByte(resto, '\n'); j >= 0 {
		resto = resto[:j]
	}
	t, err := time.Parse("2006-01-02 15:04:05 -07:00", strings.TrimSpace(resto))
	if err != nil {
		return time.Time{}
	}
	return t
}
