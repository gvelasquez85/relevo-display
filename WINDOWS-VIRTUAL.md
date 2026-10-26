# Relevo Display 0.10 — Windows e iPad 2

Conserva la app Yam Display instalada en el iPad. El usuario confirmó imagen en Windows con 0.3; esta versión aborda las limitaciones observadas. Sigue siendo una prueba experimental, no un producto terminado.

## Abrir y preparar

1. Extrae el ZIP Windows completo y abre **Relevo-Windows.exe**. Aparece una ventana de control y un icono propio junto al reloj; Windows puede agrupar el icono en la flecha de iconos ocultos. No necesitas ejecutar BAT.
2. En la ventana, selecciona **Preparar controlador** y acepta la elevación normal de Windows. Hazlo también si probaste 0.3: añade los modos del iPad 2. **Reinicia Windows después de preparar el controlador** para que anuncie los modos nuevos.
3. Vuelve a abrir el programa. Conecta un solo iPad por USB, desbloquéalo, acepta Confiar y mantén Yam Display abierto.
4. Selecciona **1024x768**, la resolución nativa del iPad 2, y **Iniciar pantalla**. La sesión permanece activa hasta Detener, Salir o un error.
5. Mueve una ventana y el ratón al monitor virtual situado a la derecha. El puntero sólo aparece en el iPad cuando está dentro de esa pantalla.

No hace falta reinstalar iTunes si el servicio Apple Mobile Device existente funciona. Si falta, el menú **Conexión USB: Apple Devices** abre la guía oficial de Apple. El transporte USB sigue dependiendo de ese servicio; este ejecutable no redistribuye ni instala automáticamente software de Apple. La compatibilidad de Apple Devices con el iPad antiguo y el servicio local requiere comprobación en el equipo.

## Resoluciones y comportamiento

Opciones 4:3 a 60 Hz anunciados por el monitor virtual: **1024×768, 1280×960, 1600×1200 y 2048×1536**. El vídeo JPEG usa exactamente el tamaño seleccionado; la pantalla física del iPad 2 permanece 1024×768, por lo que las resoluciones superiores son escritorios lógicos que se reducen al panel. Se solicitan 20 FPS de transmisión por defecto, ajustables a10/15/20/30/60, distintos de los 60 Hz del modo de Windows.

Si falta el modo elegido, se informa del error: no se sustituye por la resolución principal ni por otra proporción. Para cambiar de resolución, pulsa Detener, elige el nuevo modo en la ventana e inicia otra vez. Para ordenar pantallas, establecer escala u otras preferencias usa **Configuración de pantallas de Windows** con la sesión detenida. Durante la sesión se comprueba la configuración aplicada; los cambios externos pueden detenerla y se conservan al cerrar.

El cursor se incorpora a la captura con su posición y hotspot. Al aparecer el escritorio protegido de UAC, la captura se pausa con una imagen negra, mantiene el envío USB y vuelve a capturar al regresar al escritorio normal. No muestra el cuadro UAC en el iPad ni modifica la protección de Windows. Esto está implementado y probado con simulaciones; falta validarlo en Windows real.

## Controlador y configuración

Usa el proveedor IddCx **VirtualDrivers/MTTVDD 25.7.23**, licencia MIT incluida. No es un controlador nuevo escrito desde cero. El archivo oficial tiene SHA-256:

`e24210692b442b39af763536330ce78b423f19342b7a7792c26de3944e418b3a`

La instalación comprueba el archivo y su catálogo mediante la política Authenticode de Windows. No importa certificados ni cambia Secure Boot/test signing. Sólo admite Windows x64 nativo. El programa gráfico aún no está firmado para distribución comercial.

Los archivos permanecen en `C:\VirtualDisplayDriver`. Se añaden los cuatro modos a la configuración de ejecución, manteniendo intactos DLL, INF y CAT firmados. La configuración exacta suministrada en 0.3 puede migrarse con respaldo. Una configuración personalizada o un archivo ajeno se rechaza para evitar sobrescribirlo. No se reinicia el proveedor en caliente: reinicia Windows después de preparar los modos.

Al detener, restaura el estado previo del monitor virtual si sigue siendo el que aplicó. No cambia la resolución física principal. Si el monitor ya estaba activo antes, seguirá activo después; detener no desinstala el controlador. Los cambios externos no se sobrescriben. Un cierre forzado no permite garantizar restauración.

## Diagnóstico y alcance

El último resultado de la sesión gráfica se guarda en `%APPDATA%\RelevoDisplay\last-session.json`, sin capturas, números de serie ni valores de teclado/touch. Incluye la resolución, estado del protocolo y errores. La compilación completa conserva el ejecutable de consola para diagnósticos:

```powershell
.\relevo-windows-amd64.exe --doctor
.\relevo-windows-amd64.exe --mode extend --width 1024 --height 768 --duration 60s --log resultado.json
```

No se dispone de un Windows ejecutable en este Mac. Las pruebas automáticas validan dimensiones/negociación, políticas del monitor y restauración, configuración, pausa y reanudación simuladas y geometría del cursor. El código nativo se compila y analiza estáticamente. La prueba real pendiente debe cubrir el menú del icono, instalación/migración, imagen 4:3, cursor y varios ciclos de UAC. La confirmación de imagen del usuario corresponde a 0.3 y no valida automáticamente 0.6.

Sin audio, touch/teclado remoto ni rendimiento garantizado. El ZIP gráfico simplifica la prueba; todavía no es un instalador MSI/MSIX con firma, actualizaciones y gestión completa de dependencias.

Referencias: [proveedor oficial](https://github.com/VirtualDrivers/Virtual-Display-Driver/releases/tag/25.7.23), [escritorios de Windows](https://learn.microsoft.com/en-us/windows/win32/winstation/desktops), [Apple Devices](https://support.apple.com/guide/devices-windows/welcome/windows).

## Controles y fluidez 0.6

La ventana incluye resolución, FPS, calidad JPEG60/70/80/90/95/100, Guardar ajustes, Iniciar, Detener, Preparar controlador, Pantallas de Windows y Ayuda USB. Los controles de vídeo se modifican con la sesión detenida; se guardan al iniciar o pulsar Guardar ajustes en `%APPDATA%\RelevoDisplay\settings.json`.

Cerrar la ventana la oculta, sin detener la pantalla. Abre de nuevo desde el icono o ejecutando el programa otra vez. Para terminar usa icono → Salir; espera la restauración del monitor.

Comienza con1024×768,20FPS y calidad70. Puedes probar30FPS; si la tasa medida se queda por debajo o el iPad se retrasa, prueba calidad60 y15/20FPS. Son opciones de ajuste, no garantías de rendimiento. La ventana muestra FPS enviados por el ordenador; no confirma cuándo los presenta el iPad. El resultado JSON incluye FPS solicitados, calidad, FPS enviados y tiempos acumulados de captura/compresión y escritura.

Windows reutiliza los recursos de captura. El codificador evita volver a copiar y escalar un frame que ya tiene el tamaño elegido. El benchmark aislado JPEG1024×768 a igualcalidad80 pasó de49,28a14,91ms en MacM1; falta medir la sesión completa Windows/iPad. No se ha implementado H.264 en esta entrega.

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
