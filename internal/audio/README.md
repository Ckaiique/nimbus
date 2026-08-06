# internal/audio — som e mídia (LÓGICA)

Tudo que envolve **som** fica aqui. A interface (`internal/ui`) nunca fala com
o Windows diretamente — ela chama as funções deste pacote.

| Arquivo     | O que faz                                                            |
|-------------|----------------------------------------------------------------------|
| `volume.go` | Conecta no volume geral do Windows (Core Audio): pegar, definir, mudo. Se a conexão falhar, entra em **modo demonstração** (a janela abre mesmo assim). |
| `midia.go`  | Botões de mídia: próxima faixa, anterior e play/pause. Simula as teclas de mídia do teclado, por isso funciona com qualquer player (Spotify, YouTube, VLC...). |

**Por que separado da interface?** Se um dia mudar o jeito de falar com o
Windows, só esta pasta muda — a janelinha continua igual.
