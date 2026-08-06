// Pacote monitor: mede o uso do computador (CPU, GPU, memória e os processos
// que mais consomem).
//
// Por que em segundo plano: medir isso leva um tempinho (o Windows precisa
// comparar dois momentos para calcular porcentagem). Se fizéssemos dentro do
// desenho da janela, ela travaria. Então uma rotina separada mede a cada 2
// segundos e guarda o resultado; a janela só LÊ o valor pronto.
package monitor

import (
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

// Processo é um item da lista "quem mais consome CPU".
type Processo struct {
	Nome string
	CPU  float64 // porcentagem do total do computador
}

// Leitura é a "foto" mais recente das medições.
type Leitura struct {
	CPU       float64 // 0 a 100 (%)
	GPU       float64 // 0 a 100 (%); -1 = não conseguiu medir
	RAM       float64 // 0 a 100 (%)
	Processos []Processo
}

var (
	trava  sync.Mutex
	atual  Leitura // última medição pronta (a janela lê daqui)
)

// Atual devolve a última medição pronta (sem esperar nada).
func Atual() Leitura {
	trava.Lock()
	defer trava.Unlock()
	return atual
}

// Iniciar liga a medição em segundo plano (a cada 2 segundos).
func Iniciar() {
	iniciarGPU() // prepara o medidor de GPU (se der erro, GPU fica em -1)
	go func() {
		for {
			medir()
			time.Sleep(2 * time.Second)
		}
	}()
}

func medir() {
	nova := Leitura{GPU: -1}

	// CPU total: 0 no primeiro ciclo (precisa de 2 medições para comparar).
	if cpus, err := cpu.Percent(0, false); err == nil && len(cpus) > 0 {
		nova.CPU = cpus[0]
	}

	// Memória RAM em uso.
	if m, err := mem.VirtualMemory(); err == nil {
		nova.RAM = m.UsedPercent
	}

	nova.GPU = medirGPU()
	nova.Processos = maisPesados(3)

	trava.Lock()
	atual = nova
	trava.Unlock()
}

// guardamos os processos entre medições: a porcentagem de CPU de um processo
// só existe comparando "agora" com "da última vez que olhei".
var conhecidos = map[int32]*process.Process{}

// maisPesados devolve os N processos que mais usam CPU neste momento.
func maisPesados(n int) []Processo {
	pids, err := process.Pids()
	if err != nil {
		return nil
	}

	vivos := map[int32]bool{}
	lista := []Processo{}

	for _, pid := range pids {
		vivos[pid] = true
		p, ok := conhecidos[pid]
		if !ok {
			novo, err := process.NewProcess(pid)
			if err != nil {
				continue
			}
			conhecidos[pid] = novo
			p = novo
			p.Percent(0) // primeira olhada: só marca o ponto de partida
			continue     // sem comparação ainda, entra na próxima rodada
		}

		uso, err := p.Percent(0)
		if err != nil || uso <= 0 {
			continue
		}
		nome, err := p.Name()
		if err != nil || nome == "" {
			continue
		}
		// O valor vem "por núcleo" (pode passar de 100); dividimos pelo número
		// de núcleos para virar porcentagem DO COMPUTADOR TODO.
		lista = append(lista, Processo{Nome: nome, CPU: uso / float64(runtime.NumCPU())})
	}

	// Esquece processos que já fecharam (para não acumular memória).
	for pid := range conhecidos {
		if !vivos[pid] {
			delete(conhecidos, pid)
		}
	}

	sort.Slice(lista, func(i, j int) bool { return lista[i].CPU > lista[j].CPU })
	if len(lista) > n {
		lista = lista[:n]
	}
	return lista
}
