# Relevo Display 0.10 — iPad como pantalla extendida

Usa la misma app Yam Display instalada en el iPad. Prototipo experimental Windows/Linux, con transporte USB compatible. El usuario confirmó imagen desde Windows en 0.3; los cambios 0.6 necesitan nueva prueba real.

## Windows: prueba gráfica

Extrae `Relevo-Windows-0.10.0.zip` y abre **Relevo-Windows.exe**. Al abrir aparece una ventana con resolución, FPS y calidad JPEG. Los ajustes se guardan para el próximo inicio. También hay un icono propio junto al reloj, posiblemente dentro de iconos ocultos. Ya no requiere BAT para el flujo gráfico. [Instrucciones completas](WINDOWS-VIRTUAL.md).

- Prepara el controlador desde el menú incluso si instalaste 0.3; **reinicia Windows** para anunciar los modos nuevos.
- Conecta un solo iPad por USB, desbloquéalo, acepta Confiar y abre Yam Display.
- Elige **1024×768**, nativo del iPad 2, e inicia. También ofrece 1280×960,1600×1200 y2048×1536 como escritorios lógicos 4:3.
- El cursor se incorpora a la captura. Durante UAC se envía negro y se retoma al volver al escritorio normal, sin entrar al escritorio protegido.
- Usa Detener antes de cambiar modos o configuración de pantallas. Salir espera la limpieza de la sesión.

La conexión USB aún necesita el servicio de Apple. Si ya funciona, no reinstales iTunes. El menú abre la guía oficial Apple Devices cuando falta; el programa no redistribuye software Apple ni instala esa dependencia automáticamente. El exe gráfico tampoco constituye todavía un MSI/MSIX firmado con actualizaciones. Windows x64 nativo; driver ARM64 no incluido.

## Consola y otros sistemas

El ZIP completo conserva los ejecutables Mac arm64, Linux amd64/arm64 y Windows de consola, junto con lanzadores de diagnóstico. La consola sin parámetros conserva el patrón en Mac/Linux; Windows sin parámetros abre el icono. `--version`, `--doctor`, `--mode pattern|desktop|extend`, `--check-source`, `--width` y `--height` permiten pruebas explícitas. La CLI sigue limitada a 1s–5min por sesión; el icono mantiene sesión hasta detener.

```sh
./relevo-linux-amd64 --doctor
./relevo-linux-amd64 --mode extend --width 1024 --height 768 --duration 60s --log resultado.json
```

Linux necesita usbmuxd en el host y confianza con iPad. Hyprland crea y elimina su salida propia al tamaño elegido, usando hyprctl y wf-recorder con MJPEG. GNOME/KDE dependen del portal ScreenCast/PipeWire y GStreamer; sólo permiten extensión si el portal anuncia VIRTUAL. El tamaño del JPEG se fija, pero el portal no garantiza fijar el tamaño del escritorio del compositor. Bazzite requiere las dependencias en host y probar inicialmente Desktop Mode. Linux real no se ha validado.

Mac usa screencapture para desktop y conserva patrón; no implementa extensión. El permiso de grabación de pantalla estaba ausente en este Mac y bloqueó captura real. El patrón sí se mostró en iPad y recibió confirmación visual del usuario. Los lanzadores Mac cierran normalmente el servidor Yam original y vuelven a abrirlo al terminar.

## Evidencia y límites

Pruebas automáticas: codec/transporte, negociación de dimensiones, sesiones acotadas y continuas cancelables, selección exacta del monitor, restauración/cambios externos, formato JPEG, geometría de cursor, pausa/reanudación simuladas y configuración del proveedor. Compilación cruzada y análisis estático Windows; no equivalen a ejecutar Windows. El indicador automático `display_verified:false` conserva la distinción entre envío y confirmación visual.

El modo extend usa 1024×768 inicial; pattern/desktop conservan1280×720 salvo tamaño explícito compatible. JPEG, 20 FPS solicitados por defecto, ajustables a 10/15/20/30/60, sin audio ni touch/teclado remoto. No se garantiza rendimiento de monitor de producción.

La sesión gráfica guarda un resultado sin capturas ni seriales en `%APPDATA%\RelevoDisplay\last-session.json`. Fuente/procedencia y limitaciones del controlador en [WINDOWS-VIRTUAL.md](WINDOWS-VIRTUAL.md). Licencias en `THIRD_PARTY_NOTICES.txt`; no se incluyen ejecutables o recursos propietarios de Yam.

## Compilar

Go1.23 o posterior; dependencias howett.net/plist1.0.1 y godbus/dbus/v55.1.0.

```sh
go test -race ./...
go vet ./...
bash scripts/build.sh
python3 scripts/package.py
```

Referencias: [ScreenCast portal](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.ScreenCast.html), [proveedor MTTVDD](https://github.com/VirtualDrivers/Virtual-Display-Driver), [escritorios Windows](https://learn.microsoft.com/en-us/windows/win32/winstation/desktops). Historial detallado de las versiones previas en README-v0.2.md y README-v0.3.md.

La ventana muestra FPS enviados, no FPS presentados por el iPad. Captura/compresión y escritura se miden por separado en last-session.json. El rendimiento final requiere prueba en Windows/iPad. La etapa JPEG1024x768 a calidad80 pasó de49,28a14,91ms en benchmark MacM1; no es una medición de fluidez Windows.

Identidad propia: Relevo Display. Esta versión sigue usando el receptor Yam Display instalado en el iPad; no lo elimina ni lo reemplaza todavía. JPEG sigue activo; H.264 se evaluó, pero su envío no está implementado ni validado. Al iniciar se migran los ajustes previos si falta la configuración nueva. Cierra las versiones anteriores antes de abrir Relevo Display.

Actualización del equipo de referencia: German confirmó iPad2 con **iOS9.3.5**, mínimo aceptado para el cliente propio. 0.6.1 incorpora percentiles recientes p95 de captura/compresión, escritura e intervalo de envío para diagnosticar el lag. No son latencia de pantalla ni una mejora de códec. JPEG sigue activo. El cliente UIKit propio está en clients/ios como código de prueba local; no está aún compilado, firmado o instalado.

## Controles nuevos 0.7

Conservamos la app Yam instalada en el iPad2/iOS9.3.5; la investigación del cliente propio queda pausada por indicación de German.

La ventana permite elegir **Extender escritorio** o **Ver pantalla principal**. Este último comparte la imagen principal adaptada al iPad, sin cambiar la topología de Windows al modo Duplicar. En extensión puedes colocar el monitor a **Derecha, Izquierda, Arriba o Abajo**; la posición se guarda para el próximo inicio.

**Mantener pantalla encendida durante la sesión** es opcional y está desactivado inicialmente. Mantiene una solicitud de energía sólo durante la sesión y la libera al detener/cerrar/error, sin modificar el plan permanente de Windows. El reposo forzado por el usuario o políticas del equipo no se garantiza.

El resultado JSON incluye los ajustes completos y `stats.input_event_counts`, con contadores de eventos conocidos y sin coordenadas ni contenido del teclado. Esto facilita validar que llegan gestos; **todavía no los convierte en acciones de Windows**.

Consulta FEATURES-WINDOWS.md para la comparación completa. H264, reconexión automática, inicio de sesión, autoactualizaciones, zoom y teleprompter no se presentan como implementados. Esta entrega necesita prueba real de los controles nuevos en Windows; las pruebas automáticas no sustituyen esa validación.

## Prueba de fluidez 0.8

Esta entrega prioriza rendimiento: capturaWindowsdirectaDIB/BitBlt, menoscopias y encoder **libjpeg-turbo3.2.0 SIMD** incluido en el ejecutablex64. No necesita instalar FFmpeg ni una DLL manualmente. El formato enviado continúa siendoJPEGcompatibleconlaappactual.

La ventana indica **JPEG SIMD** cuando la ruta nativa está activa. El primer uso realiza una autoprueba pequeña de codificación/decodificación; si falla la carga o la compresión, conserva **JPEG Go** y registra el motivo en last-session.json. El caché temporal se retira al cerrar normalmente la app. No fuerces el cierre durante la prueba.

Para comparar con la medición informada porGerman, conservar **1024×768,30FPS y calidad60**. La versión0.8 no cambia los ajustes guardados. Registrar FPSenviados, capturap95, escriturap95 y si el puntero/scroll se sienten más inmediatos. Si la tasa sube pero el lag visual persiste, revisar recepción/presentación y códec; no declarar resuelto por alcanzar30FPSenviados.

En una prueba aislada deMacarm64, mismaimagen/calidad60, GoJPEG tardó16–20ms y la bibliotecanativa1,9–2,3ms porcuadro. No es una medición de la capturaWindows ni de la latencia del iPad. Estas rutas nuevas requieren prueba real en Windows. El informe añade control_roundtrip_recent_p95_ms como ida/vuelta decontrol, no como latencia de presentación.

Licencias y atribuciónIJG completas en THIRD_PARTY_NOTICES.txt; DLLoficial fijada porhash y cargada desde ruta privada verificada. H264 sigue sin implementarse. Desde0.9 se puede solicitar60FPS.

## Opciones 0.9

FPS solicitados: **10,15,20,30 y60**. CalidadJPEG: **60,70,80,90,95 y100**. Los ajustes anteriores y los valorespredeterminados se conservan; cambia estas opciones con la sesión detenida y vuelve a iniciar.

Para comparar fluidez, prueba primero60FPS manteniendo1024×768 yQ60. Después aumenta calidad a80/90/95/100 de forma independiente. Calidad100 sigue siendoJPEG con pérdidas y no garantiza mayorfluidez; aumenta trabajo y tráfico. 60FPS es objetivo de envío, no garantía de presentación en el iPad. El encoderSIMD y la medición de tasa real se conservan.

La consola acepta `--fps 60 --quality 95`. La implementación de calidadconfigurable se aplica a capturaWindows; no se afirma paridad de captura de otros sistemas. Las opciones nuevas están comprobadas con pruebas de validación/persistencia/negociación y compilaciónWindows; falta probar sus tasas y aspecto en el dispositivo.

## Prueba 0.10: envío y diagnóstico táctil

El envío reutiliza un buffer y agrupa cabecera y JPEG en una escritura habitual, conservando el protocolo y sin añadir una cola de cuadros. Reduce asignaciones y llamadas de escritura; la mejora perceptible frente a 0.9 aún requiere prueba en Windows. Se mantienen 60 FPS y calidad hasta 100.

`last-session.json` incluye el esquema de campos permitidos de los eventos táctiles: sólo nombres conocidos y tipos, sin coordenadas, texto ni contenido de teclado. Esta versión todavía no controla Windows mediante el tacto; falta comprobar la escala y secuencia real de los gestos.
