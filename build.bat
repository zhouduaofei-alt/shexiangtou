@echo off
setlocal
set PATH=C:\Program Files\Go\bin;%PATH%
set GOPROXY=https://goproxy.cn,direct
set GOSUMDB=sum.golang.google.cn
cd /d "%~dp0"
set RSRC=%USERPROFILE%\go\bin\rsrc.exe
if exist "%RSRC%" "%RSRC%" -arch amd64 -ico icon.ico -o rsrc.syso
go build -ldflags "-s -w -H windowsgui" -o Lumina.exe .
if errorlevel 1 exit /b 1
echo 已生成 Lumina.exe
