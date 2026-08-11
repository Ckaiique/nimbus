package atalhos

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// acoesDeTeste imita o que a interface registra de verdade.
func acoesDeTeste(t *testing.T) {
	t.Helper()
	Registrar([]Acao{
		{Nome: "youtube", Rotulo: "YouTube", PadraoMods: ModAlt, PadraoTecla: 0x31},   // Alt+1
		{Nome: "whatsapp", Rotulo: "WhatsApp", PadraoMods: ModAlt, PadraoTecla: 0x32}, // Alt+2
		{Nome: "parar", Rotulo: "Parar"},                                              // sem padrão
	})
}

// TestPadraoDeFabrica: quem tem combinação de fábrica nasce com ela, quem não
// tem nasce sem — e nunca com uma tecla inventada.
func TestPadraoDeFabrica(t *testing.T) {
	acoesDeTeste(t)

	a, tem := Do("youtube")
	if !tem || a.Texto() != "Alt+1" {
		t.Errorf("youtube deveria nascer com Alt+1, veio %q (tem=%v)", a.Texto(), tem)
	}
	if _, tem := Do("parar"); tem {
		t.Error("parar nao tem padrao de fabrica, nao deveria nascer com atalho")
	}
}

// TestPrecisaDeModificador é a regra que protege o uso normal do PC.
//
// Sem ela, configurar a tecla "1" faria o Nimbus trocar de serviço a cada vez
// que a pessoa digitasse 1 em qualquer programa — porque o Nimbus lê o teclado
// do PC inteiro, não só o dele.
func TestPrecisaDeModificador(t *testing.T) {
	acoesDeTeste(t)

	if err := Definir("youtube", 0, 0x31); err == nil {
		t.Error("aceitou um atalho sem Ctrl/Alt/Shift/Win — a trava falhou")
	}
	// E o atalho anterior tem de continuar de pé (a recusa não estraga nada).
	if a, _ := Do("youtube"); a.Texto() != "Alt+1" {
		t.Errorf("a recusa mexeu no atalho que ja existia: %q", a.Texto())
	}
}

// TestTeclaQueNaoServe: os modificadores e os botões normais do mouse não podem
// ser a tecla do atalho. O botão esquerdo do mouse, em especial, tomaria o
// clique do computador inteiro.
func TestTeclaQueNaoServe(t *testing.T) {
	acoesDeTeste(t)

	naoServem := map[string]uint16{
		"botao esquerdo do mouse": 0x01,
		"botao direito do mouse":  0x02,
		"botao do meio":           0x04,
		"o proprio Alt":           0x12,
		"o proprio Ctrl":          0x11,
	}
	for nome, tecla := range naoServem {
		if err := Definir("youtube", ModAlt, tecla); err == nil {
			t.Errorf("aceitou %s (codigo %#x) como tecla de atalho", nome, tecla)
		}
	}
}

// TestCombinacaoRepetidaTiraDaOutra: a mesma combinação em duas ações seria
// imprevisível (as duas dispariam no mesmo toque), então a nova vence.
func TestCombinacaoRepetidaTiraDaOutra(t *testing.T) {
	acoesDeTeste(t)

	// O WhatsApp rouba o Alt+1, que era do YouTube.
	if err := Definir("whatsapp", ModAlt, 0x31); err != nil {
		t.Fatalf("nao deveria recusar: %v", err)
	}
	if _, tem := Do("youtube"); tem {
		t.Error("o YouTube deveria ter perdido o Alt+1 para o WhatsApp")
	}
	if a, _ := Do("whatsapp"); a.Texto() != "Alt+1" {
		t.Errorf("whatsapp = %q, esperava Alt+1", a.Texto())
	}
}

// TestTextoEAnalisarSaoOInversoUmDoOutro: o que a tela escreve tem de ser
// exatamente o que o arquivo consegue ler de volta. É o que garante que salvar
// e abrir de novo não perde nem troca atalho.
func TestTextoEAnalisarSaoOInversoUmDoOutro(t *testing.T) {
	casos := []Atalho{
		{Mods: ModAlt, Tecla: 0x31},                      // Alt+1
		{Mods: ModCtrl | ModAlt, Tecla: 0x70},            // Ctrl+Alt+F1
		{Mods: ModCtrl | ModAlt | ModShift, Tecla: 0x41}, // Ctrl+Alt+Shift+A
		{Mods: ModWin, Tecla: 0x2D},                      // Win+Insert
		{Mods: ModShift, Tecla: 0x06},                    // Shift+Mouse5
		{Mods: ModAlt, Tecla: 0x69},                      // Alt+Num9
	}
	for _, a := range casos {
		texto := a.Texto()
		m, tecla, err := Analisar(texto)
		if err != nil {
			t.Errorf("Analisar(%q) devolveu erro: %v", texto, err)
			continue
		}
		if m != a.Mods || tecla != a.Tecla {
			t.Errorf("%q voltou diferente: mods %d->%d, tecla %#x->%#x",
				texto, a.Mods, m, a.Tecla, tecla)
		}
	}
}

// TestAnalisarRecusaLixo: o arquivo pode ter sido editado à mão com erro, e
// isso não pode virar um atalho estranho nem derrubar o programa.
func TestAnalisarRecusaLixo(t *testing.T) {
	lixo := []string{
		"",        // vazio
		"Alt",     // só o modificador
		"1",       // só a tecla (sem modificador)
		"Alt+Xis", // tecla que não existe
		"Alt+1+2", // duas teclas
		"Alt++1",  // separador sobrando com tecla válida... (o vazio é ignorado)
		"blabla",  // sem sentido
	}
	for _, texto := range lixo {
		if _, _, err := Analisar(texto); err == nil && texto != "Alt++1" {
			t.Errorf("Analisar(%q) deveria recusar", texto)
		}
	}
	// "Alt++1" é o único aceito da lista acima: o pedaço vazio é ignorado e
	// sobra "Alt" + "1", que é um atalho legítimo. Fica registrado aqui para
	// ninguém "consertar" isso sem saber que é de propósito.
	if _, _, err := Analisar("Alt++1"); err != nil {
		t.Errorf("Alt++1 deveria ser aceito como Alt+1: %v", err)
	}
}

// TestSalvarECarregar percorre o caminho inteiro: mudar, salvar, "reabrir o
// programa" e conferir que voltou igual.
func TestSalvarECarregar(t *testing.T) {
	pasta := t.TempDir()
	t.Setenv("LOCALAPPDATA", pasta) // é daqui que sai o os.UserCacheDir no Windows

	if !strings.HasPrefix(Arquivo(), pasta) {
		t.Skipf("o teste depende de LOCALAPPDATA (Arquivo() = %q)", Arquivo())
	}

	acoesDeTeste(t)
	if err := Definir("whatsapp", ModCtrl|ModShift, 0x57); err != nil { // Ctrl+Shift+W
		t.Fatal(err)
	}
	Limpar("youtube") // a pessoa apagou o atalho de fábrica
	if err := Salvar(); err != nil {
		t.Fatal(err)
	}

	// "Reabrir o programa": volta tudo ao de fábrica e lê o arquivo.
	acoesDeTeste(t)
	if err := Carregar(); err != nil {
		t.Fatal(err)
	}

	if a, _ := Do("whatsapp"); a.Texto() != "Ctrl+Shift+W" {
		t.Errorf("whatsapp voltou como %q", a.Texto())
	}
	// O ponto mais fácil de errar: o atalho APAGADO não pode voltar só porque
	// tem padrão de fábrica.
	if a, tem := Do("youtube"); tem {
		t.Errorf("o atalho apagado voltou do zero: %q", a.Texto())
	}
}

// TestArquivoAusenteMantemOsPadroes: na primeira vez que o Nimbus roda num PC o
// arquivo não existe. Isso é normal e o programa segue com os atalhos de
// fábrica, sem reclamar na tela.
func TestArquivoAusenteMantemOsPadroes(t *testing.T) {
	t.Setenv("LOCALAPPDATA", filepath.Join(t.TempDir(), "nao-existe"))
	acoesDeTeste(t)

	if err := Carregar(); err == nil {
		t.Error("ler um arquivo que nao existe deveria devolver erro (para depuracao)")
	}
	if a, _ := Do("youtube"); a.Texto() != "Alt+1" {
		t.Errorf("os padroes de fabrica deveriam continuar: youtube = %q", a.Texto())
	}
}

// TestArquivoCorrompidoNaoDerrubaNada: linha torta no meio é ignorada e as
// linhas boas continuam valendo.
func TestArquivoCorrompidoNaoDerrubaNada(t *testing.T) {
	pasta := t.TempDir()
	t.Setenv("LOCALAPPDATA", pasta)
	if !strings.HasPrefix(Arquivo(), pasta) {
		t.Skip("o teste depende de LOCALAPPDATA")
	}
	acoesDeTeste(t)

	conteudo := "# comentario\n" +
		"isto nao tem sinal de igual\n" +
		"youtube = Alt+9\n" +
		"servico-que-nao-existe = Alt+7\n" +
		"whatsapp = Alt+TeclaInventada\n"
	if err := os.MkdirAll(filepath.Dir(Arquivo()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Arquivo(), []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Carregar(); err != nil {
		t.Fatal(err)
	}

	if a, _ := Do("youtube"); a.Texto() != "Alt+9" {
		t.Errorf("a linha boa deveria valer: youtube = %q", a.Texto())
	}
	if _, tem := Do("whatsapp"); tem {
		t.Error("a linha com tecla inventada nao deveria virar atalho")
	}
}
