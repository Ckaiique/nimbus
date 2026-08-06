// Medição de GPU no Windows.
//
// Por que é diferente da CPU: o Windows não tem uma função simples de "uso da
// GPU". A gente usa o PDH (Performance Data Helper) — o MESMO sistema de
// contadores que o Gerenciador de Tarefas usa na aba Desempenho. Pegamos o
// contador "GPU Engine / Utilization Percentage" (motor 3D) e somamos o uso
// de todos os programas.
//
// Se qualquer passo falhar (placa sem driver, contador desabilitado...),
// medirGPU devolve -1 e a janela mostra "GPU --" — o resto continua normal.
package monitor

import (
	"syscall"
	"unsafe"
)

var (
	pdh                    = syscall.NewLazyDLL("pdh.dll")
	procPdhOpenQuery       = pdh.NewProc("PdhOpenQueryW")
	procPdhAddEnglish      = pdh.NewProc("PdhAddEnglishCounterW")
	procPdhCollect         = pdh.NewProc("PdhCollectQueryData")
	procPdhGetFormattedArr = pdh.NewProc("PdhGetFormattedCounterArrayW")
)

const pdhFmtDouble = 0x00000200 // "quero o valor como número decimal"

// Formato que o Windows usa para devolver cada item do contador.
type itemContador struct {
	nome    *uint16 // nome da instância (qual programa/motor da GPU)
	status  uint32  // 0 = valor válido
	_       uint32  // espaçamento exigido pelo formato de 64 bits
	valor   float64 // a porcentagem em si
}

var (
	consultaGPU  uintptr // "sessão" de consulta aberta com o Windows
	contadorGPU  uintptr // o contador de uso da GPU dentro da sessão
	gpuDisponivel bool
)

// iniciarGPU abre a consulta e faz a primeira coleta (que serve só de ponto
// de partida — porcentagem sempre compara dois momentos).
func iniciarGPU() {
	if ret, _, _ := procPdhOpenQuery.Call(0, 0, uintptr(unsafe.Pointer(&consultaGPU))); ret != 0 {
		return
	}
	// engtype_3D = o motor principal de desenho da placa de vídeo
	caminho, err := syscall.UTF16PtrFromString(`\GPU Engine(*engtype_3D)\Utilization Percentage`)
	if err != nil {
		return
	}
	if ret, _, _ := procPdhAddEnglish.Call(consultaGPU, uintptr(unsafe.Pointer(caminho)), 0, uintptr(unsafe.Pointer(&contadorGPU))); ret != 0 {
		return
	}
	if ret, _, _ := procPdhCollect.Call(consultaGPU); ret != 0 {
		return
	}
	gpuDisponivel = true
}

// medirGPU coleta de novo e soma o uso 3D de todos os programas.
// Devolve -1 se a medição não estiver disponível.
func medirGPU() float64 {
	if !gpuDisponivel {
		return -1
	}
	if ret, _, _ := procPdhCollect.Call(consultaGPU); ret != 0 {
		return -1
	}

	// Primeiro perguntamos "quanto espaço preciso?", depois pedimos os dados.
	var tamanho, quantidade uint32
	procPdhGetFormattedArr.Call(contadorGPU, pdhFmtDouble, uintptr(unsafe.Pointer(&tamanho)), uintptr(unsafe.Pointer(&quantidade)), 0)
	if tamanho == 0 {
		return -1
	}
	buffer := make([]byte, tamanho)
	if ret, _, _ := procPdhGetFormattedArr.Call(contadorGPU, pdhFmtDouble, uintptr(unsafe.Pointer(&tamanho)), uintptr(unsafe.Pointer(&quantidade)), uintptr(unsafe.Pointer(&buffer[0]))); ret != 0 {
		return -1
	}

	itens := unsafe.Slice((*itemContador)(unsafe.Pointer(&buffer[0])), quantidade)
	total := 0.0
	for _, item := range itens {
		if item.status == 0 {
			total += item.valor
		}
	}
	if total > 100 {
		total = 100
	}
	return total
}
