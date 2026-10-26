@echo off
cd /d "%~dp0"
echo Mantenga Yam Display abierto en el iPad, conectado por USB.
echo Cierre el servidor Yam Display original del ordenador durante la prueba.
relevo-windows-amd64.exe --duration 20s --fps 5
set "trial_status=%errorlevel%"
echo.
echo Resultado de la prueba: %trial_status%
pause
exit /b %trial_status%
