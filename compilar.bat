@echo off
rem ============================================================
rem  Compila e abre o Nimbus.
rem  Duplo clique aqui e pronto. A primeira vez demora alguns
rem  minutos (o Go compila o ImGui, que e feito em C++). Depois
rem  fica rapido, porque o resultado fica guardado em cache.
rem ============================================================
cd /d "%~dp0"

echo Compilando o Nimbus... (a primeira vez pode demorar uns minutos)
go build -ldflags "-H=windowsgui" -o build\nimbus.exe .
if errorlevel 1 (
    echo.
    echo Deu erro na compilacao. Leia a mensagem acima.
    echo Dica: se falou que nao achou "gcc", abra um terminal novo
    echo e rode este .bat de novo ^(o PATH so vale em janelas novas^).
    pause
    exit /b 1
)

echo Abrindo o Nimbus...
start "" "build\nimbus.exe"
