# ferramentas — programinhas de manutenção do projeto

Aqui ficam os programas que **não** fazem parte do Nimbus: eles são rodados
**à mão**, por quem está mexendo no projeto, e o resultado deles é um arquivo
que entra no repositório.

| Pasta | O que faz |
|---|---|
| `gerar-listas/` | Baixa as listas públicas da EasyList e regenera a lista de domínios de anúncio que vai embutida no `.exe` |
| `converter-logo/` | Converte uma imagem baixada da internet (WebP, JFIF, JPG) na logo PNG do botão de um serviço |

## Por que `ferramentas/` e não `cmd/`

Em projetos Go é comum ter uma pasta `cmd/` com os programas. Aqui isso ficaria
confuso: o Nimbus é **um** programa só, e o `main.go` dele mora na raiz. Uma
pasta `cmd/` daria a entender que existem vários programas para o usuário, e
que este seria um deles.

`ferramentas/` diz o que essas coisas realmente são: **ferramentas de oficina**.
O dono do PC nunca roda nada daqui — e nenhum código daqui vai para dentro do
`.exe` que ele usa.
