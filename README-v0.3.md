# Yam Trial 0.3 — pantalla virtual Windows y prueba de escritorio

Usa **la misma app Yam Display instalada en el iPad/iPhone**. Añade fuentes de escritorio al transporte USB que ya mostró el patrón. Es un prototipo; todavía no constituye un port completo.

| Modo | Función | Validación |
|---|---|---|
| `pattern` | Barras y texto sin capturar pantalla | Confirmación visual del usuario en 0.1; negociación y envío 0.2 probados en Mac |
| `desktop` Windows | Captura un monitor existente mediante GDI | Implementado y compilado; ejecución Windows pendiente |
| `desktop` Linux | Captura por portal, PipeWire y GStreamer | Implementado y compilado; ejecución Linux pendiente |
| `extend` Omarchy/Hyprland | Crea, transmite y elimina su propia salida virtual 1280×720 | Implementado y compilado; compositor real pendiente |
| `extend` Ubuntu/Bazzite GNOME/KDE | Solicita una pantalla virtual si el portal anuncia VIRTUAL | Implementado; disponibilidad depende de la versión/backend reales |
| `extend` Windows | Activa una pantalla virtual MTTVDD/IddCx, la captura y restaura su estado | Implementado con proveedor incluido; pendiente de prueba en Windows |
| `desktop` Mac | Captura un monitor mediante screencapture | Implementado; aquí bloqueado por permiso de grabación de pantalla |

El modo predeterminado sigue siendo `pattern`. Sólo `desktop` y `extend` comparten pantalla. La salida inicial es JPEG 1280×720, con 5 FPS solicitados; no se garantiza ese rendimiento. No hay audio ni inyección de teclado/touch. La transmisión no instala controladores; la instalación Windows se solicita aparte con `--install-driver` y la elevación normal del sistema.

Para Windows con pantalla virtual, empieza por **[WINDOWS-VIRTUAL.md](WINDOWS-VIRTUAL.md)**. El paquete incluye instalador explícito y lanzador de extensión.

## Preparación

Extrae todos los archivos a la misma carpeta. Conecta el iPad por USB, desbloquéalo, establece la confianza con el ordenador si el sistema lo solicita y mantén **Yam Display** en primer plano. Cierra otro servidor Yam durante la prueba. Los lanzadores Mac cierran normalmente el original y vuelven a abrirlo al terminar si estaba activo.

`--doctor` enumera transportes sin conectar a Yam. Si hay varios dispositivos USB, usa `--device ID` con el identificador temporal mostrado. La selección automática exige USB; no cambia silenciosamente a red.

## Windows

Necesita el servicio de dispositivos móviles de Apple (Apple Devices/iTunes). El programa busca usbmux local en `127.0.0.1:27015`; falta validar esa instalación en Windows. El ejecutable x64 de la app no está firmado. Incluye un paquete de controlador MTTVDD cuyo catálogo verifica Windows al instalarlo.

Abre `Probar-escritorio-Windows.cmd` o usa PowerShell:

```powershell
.\yam-trial-windows-amd64.exe --doctor
.\yam-trial-windows-amd64.exe --mode desktop --check-source
.\yam-trial-windows-amd64.exe --mode desktop --display 1 --duration 20s --log resultado.json
```

`--display 1`/`--display 2` seleccionan pantallas existentes en el modo de duplicación. Para la pantalla virtual, ejecuta primero `Instalar-pantalla-virtual-Windows.cmd` y después `Extender-en-Windows.cmd`. La guía [WINDOWS-VIRTUAL.md](WINDOWS-VIRTUAL.md) explica instalación, extensión, restauración y límites.

## Linux

Necesita `usbmuxd` en el host y confianza con el iPad. Para captura con portal: sesión gráfica D-Bus, xdg-desktop-portal y backend, PipeWire y GStreamer con `pipewiresrc`, `videoconvert`, `videoscale`, `videorate` y `jpegenc`.

Comprueba plugins con `gst-inspect-1.0 pipewiresrc` y `gst-inspect-1.0 jpegenc`. En Bazzite, servicios/plugins deben estar en el host; instalarlos sólo en Distrobox/Toolbox no basta. Prueba inicialmente Desktop Mode; Gaming Mode no está validado.

```sh
chmod +x yam-trial-linux-amd64
./yam-trial-linux-amd64 --doctor
./yam-trial-linux-amd64 --mode desktop --check-source
./yam-trial-linux-amd64 --mode desktop --duration 20s --log resultado.json
```

Usa el binario arm64 donde corresponda. `Probar-escritorio-Linux.sh` selecciona arquitectura. El portal puede presentar su selector/autorización de pantalla; `--display` no controla ese selector en Linux.

Para extensión, ejecuta `Extender-en-Linux.sh` o:

```sh
./yam-trial-linux-amd64 --mode extend --duration 60s --log resultado.json
```

En Omarchy/Hyprland necesita `hyprctl` y `wf-recorder` con MJPEG, dentro de la sesión Hyprland. Crea una salida `yam-...` y muestra el nombre. Mueve una ventana allí con los controles de Hyprland para verla en el iPad; una pantalla nueva vacía puede verse negra. El programa elimina sólo esa salida al cerrar, sin editar archivos de configuración. Usa sintaxis documentada en Hyprland 0.54; versiones incompatibles requieren adaptación. SIGKILL/caída del compositor no permiten garantizar limpieza.

En GNOME/KDE solicita `VIRTUAL` al portal. Si no se anuncia ese tipo de fuente, falla sin pasar a duplicación. El soporte upstream no prueba que esté habilitado en tu versión de Ubuntu/Bazzite.

## Mac de validación

`Probar-escritorio-Mac.command` duplica y `Probar-en-Mac.command` conserva el patrón. La captura requiere permiso de Grabación de pantalla para la aplicación que ejecuta el proceso. El programa no cambia ni elude ese permiso.

Aquí `CGPreflightScreenCaptureAccess()` devolvió false, con 3 pantallas activas. La captura falló y no se presenta como validada. No se guardan capturas en el paquete: el JPEG temporal privado se elimina tras leerlo.

## Evidencia y límites

En 0.1 el usuario confirmó visualmente barras y texto en la app del iPad. En la prueba 0.2 **modo patrón**: ReadyToReceive, 15 imágenes enviadas, 3 pongs, 0 errores de decodificación y salida 0; la app original se volvió a abrir. Logs: `analysis/desktop-v02-device-pattern.json` y `.txt`.

La captura real fue bloqueada por el permiso Mac. Linux/Windows se compilaron y no se ejecutaron en esos sistemas. Los modos reales de escritorio siguen pendientes de validación visual y funcional.

`display_verified:false` significa que el resultado automático no puede confirmar lo que ves. La confirmación humana de 0.1 se registra aparte. Un pong prueba control, no una imagen correcta. ErrorDecoding detiene la prueba. Los logs no guardan pantalla, números de serie ni valores de teclado/touch.

## Compilar y referencias

Go 1.23 o posterior. Dependencias: howett.net/plist 1.0.1 y godbus/dbus/v5 v5.1.0; licencias en `THIRD_PARTY_NOTICES.txt`. No se incluyen recursos o ejecutables propietarios de Yam.

```sh
go test -race ./...
go vet ./...
bash scripts/build.sh
python3 scripts/package.py
```

Fuentes primarias: [ScreenCast portal](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.ScreenCast.html), [Hyprland 0.54](https://wiki.hypr.land/0.54.0/Configuring/Using-hyprctl/), [wf-recorder](https://github.com/ammen99/wf-recorder), [GetDIBits](https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-getdibits), [IddCx](https://learn.microsoft.com/en-us/windows-hardware/drivers/display/indirect-display-driver-model-overview). Son referencias de implementación, no pruebas del prototipo.
