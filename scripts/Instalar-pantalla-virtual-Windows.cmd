@echo off
setlocal
set "RELEVO_INSTALLER_EXE=%~dp0relevo-windows-amd64.exe"
echo Se instalara el controlador virtual incluido, si aun no esta instalado.
echo Windows solicitara elevacion de administrador para esta instalacion.
powershell.exe -NoProfile -Command "try { $p = Start-Process -FilePath $env:RELEVO_INSTALLER_EXE -Verb RunAs -ArgumentList '--install-driver','--pause-install' -Wait -PassThru; exit $p.ExitCode } catch { Write-Error $_; exit 1 }"
set "trial_status=%errorlevel%"
echo.
echo Resultado del instalador: %trial_status%
echo Si termino correctamente, ejecuta Extender-en-Windows.cmd.
pause
exit /b %trial_status%
