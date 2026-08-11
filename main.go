// Nimbus — porta de entrada do programa: só liga as partes.
//
// Modo normal (sem argumentos):
//  1. audio.Novo()      -> conecta no som do Windows (ou entra em modo demo)
//  2. monitor.Iniciar() -> começa a medir CPU/GPU/RAM em segundo plano
//  3. ui.Rodar(...)     -> abre o overlay e fica rodando até fechar
//
// Modo player ("nimbus.exe --player youtube" ou "--player music"):
//  abre só o mini-player do YouTube numa janela separada. É o plano B de
//  quando o WebView2 não pode ser embutido no overlay.
package main

import (
	"os"

	"nimbus/internal/audio"
	"nimbus/internal/bandeja"
	"nimbus/internal/instancia"
	"nimbus/internal/monitor"
	"nimbus/internal/player"
	"nimbus/internal/ui"
)

func main() {
	// O modo player é um processo FILHO, aberto pelo próprio Nimbus — ele tem
	// de passar antes da trava, senão o pai impediria o próprio filho de abrir.
	if len(os.Args) >= 3 && os.Args[1] == "--player" {
		player.Rodar(os.Args[2])
		return
	}

	// Um Nimbus por vez. Abrir dois dá bug: dois ícones na bandeja, dois
	// overlays disputando o topo da tela e a tecla Insert respondendo em dobro.
	if !instancia.Unica() {
		return // já tem um aberto; a pessoa foi avisada por uma caixinha
	}
	defer instancia.Liberar()

	som := audio.Novo()
	defer som.Fechar() // ao sair, encerra a "conversa" com o Windows

	// Ícone na bandeja do sistema (ao lado do relógio): clique liga/desliga
	// os painéis, botão direito abre o menu.
	bandeja.Iniciar("Nimbus — clique para mostrar/esconder (ou tecla Insert)")
	defer bandeja.Encerrar() // não deixa ícone fantasma ao sair

	monitor.Iniciar()
	ui.Rodar(som)
}
