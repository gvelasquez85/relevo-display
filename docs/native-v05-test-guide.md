# Relevo Native 0.5 — prueba de entrada, micrófono y audio

Requiere nuestro cliente iPad 0.5 (IPA entregado sin firma), iPad2/iOS9.3.5 y Windows64bit. La verificación del desarrollador en el iPad sigue bloqueando la prueba física; compilar no demuestra funcionamiento en hardware. No se conecta a la app Yam. El host de esta entrega sigue siendo una consola experimental.

## Implementado y compilado

- Touch directo real hasta10contactos con InjectTouchInput, coordenadas de pantalla extendida, DPI, contactos estacionarios y cancelación al desconectar/cambiar pantalla. Windows y cada aplicación deciden los gestos disponibles; no se promete equivalencia con todos los gestos de macOS ni capturar gestos reservados de iOS.
- Teclado nativo iOS con textoUnicode confirmado/composición, retroceso, Enter, Tab, flechas, controles y modificadores. Pulsaciones no se registran en logs. Ctrl+C/V/A usa teclas físicas separadas del texto.
- Micrófono iPad→Windows por USB, sólo después de permiso iOS y activación explícita. PCM22050Hz/mono/16bits, paquetes hasta20ms, colas acotadas. Contador de paquetes en host; `-microphone-monitor` reproduce en altavoces Windows para diagnóstico. Esto no crea un micrófono seleccionable por Teams/Zoom.
- Audio Windows→iPad con WASAPIloopback ya existente. Activar «Escuchar Windows» en iPad. En0.5 micrófono y reproducciónUSB se prueban por separado, con exclusión explícita de controles para evitar conflictos AVAudioSession; dúplex simultáneo aún no validado.
- Cámara iPad→previsualización local del host. Componentes para cámara virtual nativa bajo `drivers/windows-media/`, aún separados de este ejecutable. OBS no se requiere ni se instala.

## Pruebas propuestas

1. Abrir clientepropio y «Iniciar enlaceUSB». En Windows ejecutar:

```
Relevo-Native-Windows-0.5.exe -mode extend -multitouch -keyboard -duration 5m
```

Probar dos dedos en una app que acepte touch (zoom/scroll), tres o más contactos, pulsación larga/arrastre, teclado con `ñ á 😀`, borrar, Enter, Ctrl+A/C/V. Desconectar mientras dedos/modificador estén pulsados: no deben quedar activos y se debe retirar la pantalla extendida. No usar UAC/escritorioseguro como objetivo de entrada.

2. Micrófono (empezar con altavoces bajos o auriculares para evitar realimentación):

```
Relevo-Native-Windows-0.5.exe -mode pattern -microphone-monitor -duration 1m
```

Activar «Enviar micrófono» en iPad y autorizar micrófono. Probar hablar, parar, segundo plano y desconectar. Sin `-microphone-monitor`, `-microphone` sólo mide recepción y no reproduce ni guarda audio.

3. Audio de Windows:

```
Relevo-Native-Windows-0.5.exe -mode pattern -audio -duration 1m
```

Activar «Escuchar Windows». Al activar micrófono, la reproducciónUSB se detiene. No confundir esta ruta con un altavoz virtual del sistema ni con captura del micrófonoWindows.

4. Cámara:

```
Relevo-Native-Windows-0.5.exe -mode pattern -camera -duration 1m
```

Activar cámara y envíoUSB en iPad. Abrir URLlocal impresa. No se anuncia compatibilidad de cámara virtual con otras apps mientras falte proveedor/instalación/pruebas.

## Estado de dispositivos e instalación

La cámara virtualWindows11 necesita proveedorCOM; la fuente no equivale a un driverUVC genérico. Micrófono seleccionable requiere endpointdeaudio y driveralimentadorfirmado. El instaladorúnico y esos dispositivos aún no están entregados. No desactivarSecureBoot/verificaciónfirmas ni instalar driversunsigned para esta prueba.

Verificaciones ejecutadas: suiteGo con detectorcarreras, vetMac/Windows y compilaciónWindowsamd64; compilaciónObjectiveC ARMv7 con SDKoficial15.2 y mínimo9.3.5; cola/protocoloC++ portable con ASan/UBSan. Estas verificaciones no sustituyen pruebas físicas.
