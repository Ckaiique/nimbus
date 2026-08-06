// Pacote listas: manter a lista de anúncios em dia, sem incomodar ninguém.
//
// ─── O QUE ESTE PACOTE FAZ ────────────────────────────────────────────────
//
// O Nimbus já nasce com uma lista de mais de cem mil domínios de anúncio dentro
// do .exe (veja `internal/adblock`). Ela funciona offline e não depende do
// servidor de ninguém — mas envelhece: a publicidade cria endereço novo todo
// dia. Este pacote busca uma versão mais nova de vez em quando e guarda no PC.
//
// Divisão de trabalho, para não misturar as coisas:
//
//	internal/adblock  -> ENTENDE de lista (o que é anúncio, como ler o arquivo)
//	internal/listas   -> vai BUSCAR na internet e guarda no disco
//
// ─── AS REGRAS QUE PROTEGEM O DONO DO PC ──────────────────────────────────
//
//  1. **Nunca atrasa a abertura.** Tudo acontece numa goroutine, começando
//     depois que a janela já está de pé. O programa abre no mesmo tempo de
//     antes, com ou sem internet.
//  2. **A cada 7 dias, não a cada abertura.** As listas somam quase 4 MB e
//     mudam pouquíssimo de um dia para o outro. Baixar sempre seria gastar a
//     internet do dono à toa e sobrecarregar um servidor que é mantido de graça
//     por voluntários.
//  3. **Falhou, ninguém fica sabendo.** Sem internet, servidor fora do ar,
//     arquivo estranho: o programa continua com a lista que já tinha e segue a
//     vida. Erro de atualização de lista não é assunto para interromper alguém
//     que está ouvindo música.
//  4. **Dá para desligar** (a caixinha "Atualizar listas sozinho", na aba
//     Config). Isso não é enfeite nem excesso de zelo: um programa que acessa a
//     internet por conta própria, sem avisar e sem poder ser impedido, é
//     invasivo — mesmo quando faz isso por um bom motivo. Quem manda no PC é o
//     dono dele.
//  5. **O que veio de fora nunca ganha da trava de segurança.** Os domínios
//     protegidos (`googlevideo.com` e companhia) continuam intocáveis, venha a
//     ordem de onde vier. A peneira é aplicada de novo na hora de ler o
//     arquivo — veja `adblock.ProtegidoOuSubdominio`.
package listas

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nimbus/internal/adblock"
)

const (
	// intervaloPadrao é de quanto em quanto tempo tentamos atualizar.
	intervaloPadrao = 7 * 24 * time.Hour

	// nomeDaLista e nomeDoEstado são os arquivos guardados na pasta de dados.
	nomeDaLista  = "listas-anuncios.txt"
	nomeDoEstado = "listas-anuncios.estado"

	// minimoFinal é o tamanho mínimo, em domínios, de um resultado aceitável.
	// As listas de verdade rendem mais de cem mil; se der menos que isso, algo
	// mudou no formato delas e é melhor ficar com a lista antiga do que trocar
	// uma boa por uma capenga.
	minimoFinal = 10000

	// tempoLimite de cada download. Generoso porque as listas são grandes e o
	// servidor às vezes está lento — mas existe, senão uma conexão pendurada
	// deixaria a goroutine viva para sempre.
	tempoLimite = 2 * time.Minute
)

// Atualizador guarda tudo que a atualização precisa saber.
//
// Ele existe como TIPO (e não só como um punhado de funções soltas) para que os
// testes possam criar um com pasta temporária e servidor de mentira, sem
// encostar na internet nem na pasta de dados de verdade do usuário.
type Atualizador struct {
	// Pasta é onde a lista baixada e a data da última tentativa são guardadas.
	Pasta string
	// Fontes são as listas públicas a baixar.
	Fontes []adblock.Fonte
	// Cliente é quem faz o download (trocável no teste).
	Cliente *http.Client
	// Intervalo é a espera entre uma tentativa e a próxima.
	Intervalo time.Duration
	// Minimo é quantos domínios o resultado precisa ter para ser aceito.
	// Zero significa "use o valor de sempre" (minimoFinal). Só os testes
	// mexem nisso, para trabalhar com listas de brinquedo.
	Minimo int

	mu          sync.Mutex
	automatico  bool
	atualizando bool
	ultimoErro  string
}

// Novo monta um Atualizador com os valores de sempre.
//
// A pasta é a de dados do usuário (`%LOCALAPPDATA%\Nimbus` no Windows). Se o
// sistema não souber informá-la — coisa rara —, a atualização automática fica
// desligada e o Nimbus segue com a lista embutida: melhor não atualizar do que
// espalhar arquivo em lugar errado.
func Novo() *Atualizador {
	return &Atualizador{
		Pasta:      pastaDeDados(),
		Fontes:     adblock.Fontes,
		Cliente:    &http.Client{Timeout: tempoLimite},
		Intervalo:  intervaloPadrao,
		automatico: true, // ligado de fábrica; a caixinha da Config desliga
	}
}

// pastaDeDados devolve `%LOCALAPPDATA%\Nimbus` (ou o equivalente do sistema).
//
// É a pasta certa para isto: guarda coisa que o programa pode rebaixar sozinho
// (a lista dá para baixar de novo), não vai junto no backup de documentos do
// usuário e não suja a pasta do programa — que num PC de verdade pode estar em
// "Arquivos de Programas", onde escrever exige permissão de administrador.
func pastaDeDados() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		return ""
	}
	return filepath.Join(base, "Nimbus")
}

// ArquivoDaLista é o caminho completo da lista guardada.
func (a *Atualizador) ArquivoDaLista() string {
	if a.Pasta == "" {
		return ""
	}
	return filepath.Join(a.Pasta, nomeDaLista)
}

// ─────────────────────────── o que a interface usa ────────────────────────

// As origens possíveis da lista em uso, repetidas aqui para a interface não
// precisar conhecer o pacote adblock.
//
// A regra da seta do projeto é `ui -> listas -> adblock`: a interface fala com
// quem cuida das listas, e quem cuida das listas fala com quem entende delas.
// Se a `ui` importasse o `adblock` direto, a seta viraria uma teia.
const (
	OrigemBaixada  = adblock.OrigemBaixada
	OrigemEmbutida = adblock.OrigemEmbutida
	OrigemReserva  = adblock.OrigemReserva
)

// Estado é o retrato da situação, para a aba Config mostrar.
type Estado struct {
	// Origem é "baixada", "embutida" ou "reserva".
	Origem string
	// Gerada é quando a lista em uso foi montada (zero quando não se sabe).
	Gerada time.Time
	// Quantos domínios ela bloqueia.
	Quantos int
	// Atualizando diz se há um download acontecendo agora.
	Atualizando bool
	// Automatico é a caixinha "Atualizar listas sozinho".
	Automatico bool
	// UltimoErro é a última falha, em texto curto. Fica guardado para quem
	// quiser investigar — mas NÃO é para aparecer como alerta na cara de
	// ninguém: falha de atualização de lista não atrapalha o uso do programa.
	UltimoErro string
}

// Estado conta como as coisas estão agora.
func (a *Atualizador) Estado() Estado {
	a.mu.Lock()
	atualizando, automatico, erro := a.atualizando, a.automatico, a.ultimoErro
	a.mu.Unlock()

	l := adblock.EmUso()
	return Estado{
		Origem:      l.Origem,
		Gerada:      l.Gerada,
		Quantos:     l.Quantos(),
		Atualizando: atualizando,
		Automatico:  automatico,
		UltimoErro:  erro,
	}
}

// DefinirAutomatico liga ou desliga a atualização sozinha.
func (a *Atualizador) DefinirAutomatico(ligado bool) {
	a.mu.Lock()
	a.automatico = ligado
	a.mu.Unlock()
}

// Automatico diz se a atualização sozinha está ligada.
func (a *Atualizador) Automatico() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.automatico
}

// Iniciar é o que a interface chama quando a janela já está de pé.
//
// Ele faz duas coisas, e as duas em SEGUNDO PLANO:
//
//  1. carrega a lista que já estiver guardada no disco (rápido, mas ainda
//     assim: ler e indexar 100 mil domínios não é coisa para fazer no meio de
//     um quadro da interface);
//  2. se a atualização automática estiver ligada e já tiver passado do prazo,
//     baixa uma versão nova.
//
// Chamar duas vezes não faz mal: a segunda vez encontra o trabalho em
// andamento e desiste.
func (a *Atualizador) Iniciar() {
	go func() {
		defer recuperar("Iniciar")

		// Força o carregamento da lista embutida AQUI, na goroutine. Ler e
		// indexar cem mil domínios custa algumas dezenas de milissegundos —
		// pouco, mas o suficiente para engasgar um quadro se acontecesse na
		// thread da interface, que é onde ele aconteceria se o primeiro a
		// precisar da lista fosse o texto da aba Config.
		adblock.EmUso()

		// A lista guardada tem prioridade sobre a embutida — é a mais nova.
		// "Não existe" é o caso NORMAL da primeira vez que o Nimbus roda.
		if caminho := a.ArquivoDaLista(); caminho != "" {
			if err := adblock.UsarArquivo(caminho); err != nil {
				anotar("nao usei a lista guardada: %v", err)
			} else {
				anotar("usando a lista guardada em %s", caminho)
			}
		}

		if !a.Automatico() {
			anotar("atualizacao automatica desligada pelo dono")
			return
		}
		if !a.PassouDoPrazo() {
			anotar("ainda nao passaram %v desde a ultima tentativa", a.intervalo())
			return
		}
		a.atualizarAgora()
	}()
}

// AtualizarAgora é o botão "Atualizar listas agora" da aba Config.
//
// Ignora o prazo de 7 dias (quem clicou quer agora) mas respeita o "já tem uma
// atualização acontecendo". Volta na hora: o trabalho vai para uma goroutine e
// a interface nem pisca.
func (a *Atualizador) AtualizarAgora() {
	go func() {
		defer recuperar("AtualizarAgora")
		a.atualizarAgora()
	}()
}

// ───────────────────────────── o miolo ────────────────────────────────────

func (a *Atualizador) intervalo() time.Duration {
	if a.Intervalo <= 0 {
		return intervaloPadrao
	}
	return a.Intervalo
}

// PassouDoPrazo diz se já dá para tentar de novo.
//
// Sem nenhuma tentativa registrada (primeira vez no PC), a resposta é sim.
func (a *Atualizador) PassouDoPrazo() bool {
	ultima := a.UltimaTentativa()
	if ultima.IsZero() {
		return true
	}
	return time.Since(ultima) >= a.intervalo()
}

// atualizarAgora faz a atualização de verdade, do começo ao fim.
//
// Devolve erro só para os testes: quem chama no programa de verdade ignora, e
// é de propósito — falha aqui não muda nada para quem está usando o Nimbus.
func (a *Atualizador) atualizarAgora() error {
	// Uma de cada vez. Sem isto, clicar duas vezes no botão baixaria 8 MB duas
	// vezes e as duas tentariam gravar o mesmo arquivo.
	a.mu.Lock()
	if a.atualizando {
		a.mu.Unlock()
		return fmt.Errorf("já há uma atualização em andamento")
	}
	a.atualizando = true
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.atualizando = false
		a.mu.Unlock()
	}()

	// A data da tentativa é gravada ANTES de baixar. Assim, se o servidor
	// estiver fora do ar, o Nimbus não fica tentando de novo a cada abertura —
	// espera o prazo normal, como faria se tivesse dado certo.
	a.marcarTentativa()

	err := a.baixarEGuardar()

	a.mu.Lock()
	if err != nil {
		a.ultimoErro = err.Error()
	} else {
		a.ultimoErro = ""
	}
	a.mu.Unlock()

	if err != nil {
		anotar("atualizacao falhou: %v", err)
		return err
	}
	anotar("listas atualizadas com sucesso")
	return nil
}

// baixarEGuardar é o caminho feliz inteiro: baixar, conferir, converter,
// gravar e passar a usar.
//
// Em QUALQUER tropeço ele devolve erro sem tocar no que já existia. Essa é a
// regra mais importante daqui: uma lista boa nunca pode ser perdida por causa
// de um download ruim.
func (a *Atualizador) baixarEGuardar() error {
	var baixadas []adblock.ListaBaixada
	for _, f := range a.Fontes {
		texto, err := a.baixar(f.URL)
		if err != nil {
			return fmt.Errorf("baixar %s: %w", f.Nome, err)
		}
		// A conferência que impede o pior caso: um servidor com problema não
		// devolve "erro", devolve uma PÁGINA DE ERRO em HTML, com código 200.
		// Aceitar isso trocaria a lista boa por lixo, e o Nimbus pararia de
		// bloquear sem ninguém entender por quê.
		if !adblock.ParecemListasDeVerdade(texto, adblock.MinimoPorLista) {
			return fmt.Errorf("o que veio de %s não parece uma lista da EasyList", f.Nome)
		}
		baixadas = append(baixadas, adblock.ListaBaixada{Fonte: f, Texto: texto})
	}

	minimo := a.Minimo
	if minimo <= 0 {
		minimo = minimoFinal
	}
	conteudo, res := adblock.MontarArquivo(baixadas, time.Now())
	if res.Final < minimo {
		return fmt.Errorf("só saíram %d domínios (o esperado passa de %d) — o formato das listas pode ter mudado",
			res.Final, minimo)
	}

	// Passar a usar ANTES de gravar: se o disco der problema, pelo menos esta
	// sessão já fica com a lista nova.
	if err := adblock.Usar(conteudo, adblock.OrigemBaixada); err != nil {
		return fmt.Errorf("a lista montada não passou na conferência: %w", err)
	}

	if a.Pasta == "" {
		return nil // sem pasta de dados: usa nesta sessão e pronto
	}
	if err := a.gravarComSeguranca(conteudo); err != nil {
		return err
	}
	return nil
}

// gravarComSeguranca grava primeiro num arquivo temporário e só então o
// renomeia para o nome definitivo.
//
// Por quê: gravar 2 MB direto por cima do arquivo bom é abrir uma janela de
// alguns instantes em que ele está pela metade. Se o PC desligar exatamente
// aí, o Nimbus abriria na próxima vez com meia lista. Com o troca-troca, o
// arquivo definitivo ou é o antigo inteiro ou o novo inteiro — nunca um
// pedaço dos dois.
func (a *Atualizador) gravarComSeguranca(conteudo []byte) error {
	if err := os.MkdirAll(a.Pasta, 0o755); err != nil {
		return fmt.Errorf("criar a pasta de dados: %w", err)
	}
	definitivo := a.ArquivoDaLista()
	temporario := definitivo + ".novo"

	if err := os.WriteFile(temporario, conteudo, 0o644); err != nil {
		return fmt.Errorf("gravar o arquivo temporário: %w", err)
	}
	// No Windows o Rename não substitui arquivo existente em algumas
	// situações; remover antes evita a falha. Se o programa morrer entre as
	// duas linhas, a próxima abertura não acha a lista guardada e usa a
	// embutida — chato, mas seguro.
	_ = os.Remove(definitivo)
	if err := os.Rename(temporario, definitivo); err != nil {
		_ = os.Remove(temporario)
		return fmt.Errorf("trocar o arquivo pelo novo: %w", err)
	}
	return nil
}

// baixar pega o texto de uma lista pública.
func (a *Atualizador) baixar(url string) (string, error) {
	cliente := a.Cliente
	if cliente == nil {
		cliente = &http.Client{Timeout: tempoLimite}
	}
	resp, err := cliente.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("o servidor respondeu %s", resp.Status)
	}
	// Teto de leitura: sem ele, um servidor maluco (ou trocado por outra
	// coisa) poderia despejar gigabytes na memória do Nimbus. As listas de
	// verdade não passam de poucos MB.
	corpo, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return "", err
	}
	return string(corpo), nil
}

// ────────────────────── a data da última tentativa ────────────────────────

// UltimaTentativa lê do disco quando foi a última vez que tentamos.
//
// Arquivo ausente ou ilegível devolve data zerada, que significa "nunca
// tentamos" — e no pior caso faz uma atualização acontecer uma vez a mais.
// Nada de grave.
func (a *Atualizador) UltimaTentativa() time.Time {
	if a.Pasta == "" {
		return time.Time{}
	}
	dados, err := os.ReadFile(filepath.Join(a.Pasta, nomeDoEstado))
	if err != nil {
		return time.Time{}
	}
	for _, linha := range strings.Split(string(dados), "\n") {
		linha = strings.TrimSpace(linha)
		if !strings.HasPrefix(linha, "tentativa=") {
			continue
		}
		t, err := time.Parse(time.RFC3339, strings.TrimPrefix(linha, "tentativa="))
		if err != nil {
			return time.Time{}
		}
		return t
	}
	return time.Time{}
}

// marcarTentativa grava a data de agora. Falha ao gravar é ignorada de
// propósito: no pior caso o Nimbus tentará atualizar de novo na próxima
// abertura, o que é bem menos ruim do que não abrir.
func (a *Atualizador) marcarTentativa() {
	if a.Pasta == "" {
		return
	}
	if err := os.MkdirAll(a.Pasta, 0o755); err != nil {
		return
	}
	texto := "# Quando o Nimbus tentou atualizar as listas de anuncio pela ultima vez.\n" +
		"# Apagar este arquivo faz ele tentar de novo na proxima abertura.\n" +
		"tentativa=" + time.Now().Format(time.RFC3339) + "\n"
	_ = os.WriteFile(filepath.Join(a.Pasta, nomeDoEstado), []byte(texto), 0o644)
}

// ──────────────────────── a instância de sempre ───────────────────────────

// padrao é o Atualizador que o programa usa. As funções soltas abaixo existem
// para a interface não precisar carregar um ponteiro para lá e para cá.
var padrao = Novo()

// Iniciar começa o trabalho em segundo plano. Chame depois que a janela já
// estiver de pé — nunca antes, para não atrasar a abertura.
func Iniciar() { padrao.Iniciar() }

// AtualizarAgora dispara uma atualização na hora (botão da aba Config).
func AtualizarAgora() { padrao.AtualizarAgora() }

// AtualEstado conta como as coisas estão, para a aba Config mostrar.
func AtualEstado() Estado { return padrao.Estado() }

// DefinirAutomatico liga/desliga a atualização sozinha (caixinha da Config).
func DefinirAutomatico(ligado bool) { padrao.DefinirAutomatico(ligado) }

// Automatico diz se a atualização sozinha está ligada.
func Automatico() bool { return padrao.Automatico() }

// recuperar é a rede de proteção das goroutines: um erro inesperado aqui
// derrubaria o Nimbus inteiro (goroutine que entra em pânico leva o programa
// junto), e nenhuma lista de anúncio vale isso.
func recuperar(onde string) {
	if r := recover(); r != nil {
		anotar("pânico em %s: %v", onde, r)
	}
}
