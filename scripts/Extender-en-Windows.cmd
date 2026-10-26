@echo off
setlocal
cd /d "%~dp0"
echo Se activara la pantalla virtual y se enviara al iPad durante 60 segundos.
echo Mantenga Yam Display abierto en el iPad y conectado por USB.
relevo-windows-amd64.exe --mode extend --duration 60s --fps 20 --log resultado-extension.json
set "trial_status=%errorlevel%"
echo.
echo Resultado de la extension: %trial_status%
pause
exit /b %trial_status%
