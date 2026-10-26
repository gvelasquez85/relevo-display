@echo off
cd /d "%~dp0"
echo Se enviara la pantalla principal al iPad durante 20 segundos.
echo Mantenga Yam Display abierto en el iPad, conectado por USB.
relevo-windows-amd64.exe --mode desktop --duration 20s --fps 20
set "trial_status=%errorlevel%"
echo.
echo Resultado de la prueba: %trial_status%
pause
exit /b %trial_status%
