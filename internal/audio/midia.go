// Teclas de mídia: próxima faixa, faixa anterior e play/pause.
//
// Por que funciona com qualquer player (Spotify, YouTube, VLC...): a gente não
// conversa com o player — a gente "finge" que o usuário apertou as teclas de
// mídia do teclado (aquelas de fone/teclado multimídia). O Windows entrega o
// comando para o player que estiver tocando.
package audio

import "syscall"

var (
	user32DLL       = syscall.NewLazyDLL("user32.dll")
	procKeybdEvent  = user32DLL.NewProc("keybd_event")
)

// Códigos oficiais do Windows para as teclas de mídia (Virtual-Key Codes).
const (
	teclaFaixaAnterior = 0xB1 // VK_MEDIA_PREV_TRACK
	teclaPlayPause     = 0xB3 // VK_MEDIA_PLAY_PAUSE
	teclaProximaFaixa  = 0xB0 // VK_MEDIA_NEXT_TRACK

	eventoSoltarTecla = 0x0002 // KEYEVENTF_KEYUP
)

// apertar simula pressionar e soltar uma tecla (como um toque rápido).
func apertar(tecla byte) {
	procKeybdEvent.Call(uintptr(tecla), 0, 0, 0)                        // pressiona
	procKeybdEvent.Call(uintptr(tecla), 0, uintptr(eventoSoltarTecla), 0) // solta
}

// ProximaFaixa pula para a próxima música/vídeo.
func ProximaFaixa() { apertar(teclaProximaFaixa) }

// FaixaAnterior volta para a música/vídeo anterior.
func FaixaAnterior() { apertar(teclaFaixaAnterior) }

// PlayPause pausa ou continua o que estiver tocando.
func PlayPause() { apertar(teclaPlayPause) }
