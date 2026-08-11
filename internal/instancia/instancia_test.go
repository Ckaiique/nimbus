package instancia

import "testing"

// Testa o coração da trava: a primeira reserva passa, a segunda com o MESMO
// nome é recusada, e depois de liberar dá para reservar de novo.
//
// Detalhe do Windows que faz isso funcionar: pedir a mesma plaquinha duas
// vezes devolve "já existe" mesmo dentro do MESMO programa — por isso o teste
// não precisa abrir dois processos.
//
// Usamos um nome só do teste para nunca esbarrar num Nimbus de verdade aberto
// na máquina de quem está rodando os testes.
const nomeDeTeste = "Nimbus.Overlay.KST.Teste.InstanciaUnica"

func TestSegundaTentativaEhRecusada(t *testing.T) {
	primeiro, jaExistia := reservar(nomeDeTeste)
	if primeiro == 0 {
		t.Fatal("não consegui criar a trava nem na primeira vez")
	}
	if jaExistia {
		t.Fatal("a primeira reserva não devia encontrar plaquinha existente")
	}

	segundo, jaExistia := reservar(nomeDeTeste)
	if !jaExistia {
		t.Error("a segunda reserva devia acusar que já existe um Nimbus aberto")
	}
	if segundo != 0 {
		procCloseHandle.Call(segundo)
	}

	// Soltando a primeira, a plaquinha some e uma nova reserva volta a passar
	// — é o que garante que reabrir o Nimbus depois de fechar funciona.
	procReleaseMutex.Call(primeiro)
	procCloseHandle.Call(primeiro)

	terceiro, jaExistia := reservar(nomeDeTeste)
	if jaExistia {
		t.Error("depois de liberar, a trava devia estar livre de novo")
	}
	if terceiro != 0 {
		procReleaseMutex.Call(terceiro)
		procCloseHandle.Call(terceiro)
	}
}
