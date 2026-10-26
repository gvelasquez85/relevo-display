# Relevo Native 0.6 — teclado contextual y modos dedicados

Cliente propio iPad2/iOS9.3.5+ y host Windowsamd64 experimental de consola. IPA sin firma: sigue pendiente resolver la verificación de desarrollador del iPad. No conecta con la app Yam ni sustituye el ejecutableGUIlegacy.

## Modos

- Pantalla: teclado oculto inicialmente. Un toque breve de un dedo sobre campo editable nativo enfocado permite abrirlo; un toque sobrecontrolnoeditable lo oculta. La respuesta se correlaciona con la secuencia del toque para descartar respuestas antiguas. Arrastre/pinch no solicitan teclado. No se abre por la mera existencia de foco ni al conectarse.
- Teclado: elección explícita del usuario para teclado completo sin imagencompartida. Escape, Tab, Enter, Ctrl/Alt/Shift/Windows y atajos en barraauxiliar. Salir libera teclas y contactos.
- Mouse: touchpad relativo sin imagencompartida, clicizquierdo/derecho, dosdedosscroll y mantenerpulsado paraarrastrar. Salir libera botones.

EnTeclado/Mouse el host pausa capturas y nuevosJPEG; una imagen ya enviada puede completarse/confirmarse. Pingperiódico mantiene la sesión sin vídeo. Cámara/audio/micrófono son funciones independientes con consentimientoexplícito y no se detienen implícitamente por cambiar de modo de entrada.

## Limitación de apertura automática

Esta versión detecta campos Windows nativos Edit/RichEdit mediante metadatos, sin leertexto ni contraseñas. Navegadores, Electron, Office y controles personalizados no están cubiertos: usar modoTeclado. También puede requerirse un segundo toque si la aplicación aún no actualizó el foco. No se promete compatibilidad universal. No se ha probado físicamente.

## Prueba en Windows

```
Relevo-Native-Windows-0.6.exe -mode extend -multitouch -keyboard -pointer -duration 5m
```

Abrir cliente0.6 y enlaceUSB; probar campoeditable nativo, botón, sololecutra, pinch y arrastre. ElegirTeclado/Mouse, esperar másde5s y volveraPantalla: no debe desconectarse ni enviar imágenes mientras tanto. Probarcambio de modo durante arrastre/modificadorpulsado, cámara/micfonoindependientes y desconexión. Teclas y botones deben liberarse antes de retirar la pantalla.

El flag `-touch` anterior conserva mouseabsoluto, es alternativo a `-multitouch`. `-keyboard` solicita foco contextual. `-pointer` habilita touchpad dedicado. Modos no negociados se deshabilitan en iPad.

Audio/microfono: `-audio` envía salidaWindows; `-microphone` mide recepcióniPad; `-microphone-monitor` permite escucha explícita en altavocesWindows con riesgo de realimentación. Enelcliente actual reproducciónUSB y microfono se activan porseparado. No es un micrófono virtualseleccionable. `-camera` sigue siendo previsualizaciónlocal, no dispositivo listo paraotrasapps.

## Wi-Fi

Revisado en docs/wifi-connection-design.md. Detectar Network no habilita sesiones: el receptorUSB actual sóloescucha loopback y la red deApple usaTCPdirecto. Se propone transportepropio TLS1.2autenticado, iPadcliente saliente y emparejamientoUSB/QR. Enesta versión no se abren puertosLAN ni se transmite teclado/cámara/micrófono porWiFi.

## Verificación

SuiteGo -race, vetMac/Windows y crossbuildWindowsamd64; compilacióniOSARMv7 conSDK15.2 y mínimo9.3.5. Tests cubrenpausavídeo, inactividadmayor5s conheartbeat yretornoaPantalla, focoposterioratoque, contactoúltiplenoabre teclado, ratónrelativo yliberación al cerrar. Falta validaciónWindows/iPad, firma iOS, proveedoresdemedia completos e instaladorúnico.
