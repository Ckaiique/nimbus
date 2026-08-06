// Pacote audio: conversa com o sistema de som do Windows (Core Audio).
//
// Por que existe: a interface (pasta ui) não deve saber COMO o Windows controla
// o volume — ela só pede "me dá o volume" ou "coloca em 50%". Se um dia o jeito
// de falar com o Windows mudar, só este arquivo muda.
package audio

import (
	"errors"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

// Controle guarda a conexão com o volume do Windows.
// Se a conexão falhar, Demo fica true e o programa funciona em modo
// demonstração (o slider mexe, mas não muda o som de verdade) — assim a
// janela nunca deixa de abrir.
type Controle struct {
	Demo bool // true = não conseguiu conectar no áudio; modo demonstração

	dispositivo *wca.IMMDevice
	volume      *wca.IAudioEndpointVolume
	volumeDemo  int32 // usado só no modo demonstração
	mudoDemo    bool
	comAberto   bool // true se a "conversa" COM com o Windows foi aberta
}

// Novo tenta se conectar ao dispositivo de som padrão do Windows.
// Nunca retorna erro: se algo falhar, devolve um Controle em modo demonstração.
func Novo() *Controle {
	c := &Controle{Demo: true, volumeDemo: 50}

	// COM é o "idioma" que o Windows usa para programas conversarem com ele.
	// Precisamos "abrir a conversa" antes de qualquer coisa.
	//
	// CUIDADO com o resultado: a biblioteca devolve "erro" para QUALQUER
	// resposta diferente de zero, mas duas dessas respostas significam
	// "está tudo bem, já estava aberto":
	//
	//	S_FALSE          -> o COM já tinha sido aberto nesta thread
	//	RPC_E_CHANGED_MODE -> já estava aberto em outro modo (dá para usar)
	//
	// Tratar essas duas como falha era o motivo de o Nimbus cair no modo
	// demonstração ("demo - sem som") de vez em quando: dependendo da thread
	// em que o Go rodava, o COM já estava aberto e a gente desistia à toa.
	const (
		jaEstavaAberto = 0x00000001 // S_FALSE
		outroModo      = 0x80010106 // RPC_E_CHANGED_MODE
	)
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		codigo := codigoDoErro(err)
		if codigo != jaEstavaAberto && codigo != outroModo {
			return c // aí sim é falha de verdade: modo demonstração
		}
		// Já estava aberto por outra parte do programa: seguimos usando, mas
		// NÃO fechamos no final (quem abriu é que fecha).
	} else {
		c.comAberto = true
	}

	// Pede ao Windows a lista de dispositivos de som...
	var enumerador *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &enumerador); err != nil {
		return c
	}
	defer enumerador.Release()

	// ...e pega o dispositivo padrão de SAÍDA (caixinha/fone).
	if err := enumerador.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &c.dispositivo); err != nil {
		return c
	}

	// Por fim, pede o "controle remoto" de volume desse dispositivo.
	if err := c.dispositivo.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &c.volume); err != nil {
		return c
	}

	c.Demo = false
	return c
}

// Pegar devolve o volume atual, de 0 a 100.
func (c *Controle) Pegar() int32 {
	if c.Demo {
		return c.volumeDemo
	}
	var nivel float32 // o Windows usa 0.0 a 1.0; convertemos para 0 a 100
	if err := c.volume.GetMasterVolumeLevelScalar(&nivel); err != nil {
		return c.volumeDemo
	}
	return int32(nivel*100 + 0.5) // +0.5 arredonda em vez de cortar
}

// Definir muda o volume do Windows (recebe de 0 a 100).
func (c *Controle) Definir(valor int32) {
	if valor < 0 {
		valor = 0
	}
	if valor > 100 {
		valor = 100
	}
	if c.Demo {
		c.volumeDemo = valor
		return
	}
	c.volume.SetMasterVolumeLevelScalar(float32(valor)/100, nil)
}

// Mudo diz se o som está mutado.
func (c *Controle) Mudo() bool {
	if c.Demo {
		return c.mudoDemo
	}
	var mudo bool
	if err := c.volume.GetMute(&mudo); err != nil {
		return false
	}
	return mudo
}

// AlternarMudo liga/desliga o mudo (como a tecla de mute do teclado).
func (c *Controle) AlternarMudo() {
	if c.Demo {
		c.mudoDemo = !c.mudoDemo
		return
	}
	c.volume.SetMute(!c.Mudo(), nil)
}

// codigoDoErro pega o número que o Windows devolveu dentro do erro.
func codigoDoErro(err error) uint32 {
	var erroOle *ole.OleError
	if errors.As(err, &erroOle) {
		return uint32(erroOle.Code())
	}
	return 0xFFFFFFFF // não é um erro do Windows: trata como falha
}

// Fechar libera a conexão com o Windows. Chamar ao sair do programa,
// para não deixar "conversa aberta" com o sistema.
func (c *Controle) Fechar() {
	if c.volume != nil {
		c.volume.Release()
	}
	if c.dispositivo != nil {
		c.dispositivo.Release()
	}
	if c.comAberto {
		ole.CoUninitialize()
	}
}
