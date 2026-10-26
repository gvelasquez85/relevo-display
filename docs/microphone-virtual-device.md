# Relevo Microphone: contrato para el dispositivo virtual

Estado (25 octubre 2026): conversión y cola portables implementadas y probadas; **todavía no existe un paquete de controlador Relevo instalable ni un micrófono seleccionable en Windows**. La escucha del micrófono en los altavoces sigue siendo una prueba distinta. El código de este documento no registra endpoints y no cambia dispositivos predeterminados.

## Camino de datos

1. Cliente iPad entrega PCM16 LE, mono, 22050 Hz, por RVL1 tipo 13.
2. El host valida formato y consentimiento; `MicrophoneFeeder` acepta el contenido PCM con cabecera de ocho bytes, convierte a 48000 Hz mono PCM16 por interpolación lineal y conserva fase entre paquetes. No normaliza, comprime ni graba.
3. Su cola fija conserva como máximo 5760 muestras (120 ms); descarta las más antiguas al llenarse. Lecturas sin datos producen silencio. Tras 250 ms sin paquetes, o un reloj retrocedido, descarta audio pendiente y reinicia la conversión. Disconnect borra audio y estado inmediatamente.
4. El futuro servicio escribe bloques de hasta 960 muestras (20 ms) al controlador con el protocolo RVM2 descrito abajo.
5. El futuro pin WaveRT de captura entrega los bloques a Windows, completa con ceros en underrun y sigue avanzando su reloj de captura. Las aplicaciones seleccionarán «Relevo Microphone».

No debe existir una cola de 120 ms en cada salto: el límite portable es una protección, no un objetivo de precarga. El servicio deberá vaciarla a cada periodo y el controlador conservar únicamente su presupuesto pendiente; las mediciones de extremo a extremo fijarán el límite final. La interpolación lineal es la primera implementación funcional; calidad de voz y comportamiento ante deriva de relojes requieren medición real. No hay aún compensación adaptativa de deriva.

## Frontera C preparada para kernel

`drivers/windows-media/src/microphone_endpoint.h` no utiliza STL, asignación, mutex, handles ni llamadas al sistema. Contiene cola circular de tamaño fijo, lectura con silencio y decodificador de escritura con validación completa antes de mutar estado. El llamador debe serializar open/ingest/read con el mismo bloqueo apropiado para el contexto WaveRT; las funciones no son atómicas por sí solas.

RVM2, little endian, sin padding ni punteros:

| Offset | Longitud | Valor |
|---|---:|---|
| 0 | 4 | ASCII RVM2 |
| 4 | 2 | Versión 1 |
| 6 | 2 | Operación: 1 PCM, 2 disconnect |
| 8 | 8 | Generación de sesión |
| 16 | 4 | Número de muestras, 1–960 para PCM; 0 para disconnect |
| 20 | 4 | Reservado, cero |
| 24 | 2 × muestras | PCM16 LE mono 48000 Hz |

`relevo_mic_open` recibe la generación asignada por el código confiable de apertura. Un paquete no puede cambiarla. Disconnect cierra la generación; escrituras posteriores y generaciones antiguas se rechazan. La futura apertura exclusiva debe asignar una generación nueva, monotónica durante la vida de la instancia. El servicio no puede reutilizar la generación de otra sesión.

No se han asignado IOCTL ni creado un device interface: hacerlo exige integrar seguridad, apertura y cierre de archivos en el controlador. El contrato previsto es METHOD_BUFFERED, WRITE_DATA, acceso de servicio dedicado/SYSTEM exclusivamente, límites de longitud comprobados antes del acceso al búfer. Nada de memoria compartida escribible libremente por usuarios. Cleanup, sorpresa PnP, parada y timeout del servicio deben limpiar la cola aun cuando el proceso termine sin enviar disconnect. El timeout final también debe existir en kernel: el del feeder de usuario no protege de una caída del servicio.

## Integración que falta para tener el dispositivo real

La base Microsoft SYSVAD es un controlador WDM/PortCls que expone endpoints y simula captura generando un tono: copiar o compilar la muestra no conecta el iPad automáticamente. La implementación pendiente debe:

- Fijar una revisión concreta y conservar su licencia, crear proyecto WDK y exponer únicamente el endpoint de captura mono 48000 PCM16 Relevo, sin los endpoints de ejemplo.
- Sustituir el generador de captura por la cola C; copiar al búfer WaveRT en su temporizador/posición, manejar wrap y notificaciones, y garantizar avance correcto de posición incluso con silencio. No hacer IO bloqueante, conversión ni asignación en DPC.
- Añadir interfaz de servicio y sus IOCTL, ACL, apertura exclusiva, generación, timeout monotónico, y limpieza en file cleanup/PnP. El lado usuario no debe recibir punteros del kernel.
- Implementar feeder Windows que reciba el PCM del host, abra el dispositivo y transporte RVM2. Enviar silence/disconnect y cerrar sesión cuando acaba el consentimiento o el enlace.
- Compilar con Visual Studio/WDK y ejecutar validación INF, Driver Verifier y pruebas HLK de captura/energía/desconexión, incluyendo Teams/Zoom/grabadora y varios lectores.
- Obtener la firma Microsoft apropiada para distribución y añadir INF/SYS/CAT al único instalador Relevo. El instalador requiere elevación para el driver; no se propone desactivar Secure Boot ni comprobación de firmas.

La documentación Microsoft indica que los drivers virtuales tienen los mismos requisitos de firma que el hardware y que código kernel debe estar firmado. Los requisitos de publicación se deben validar en Hardware Dev Center al preparar la distribución; esta fase no asume que una firma local de desarrollo sea suficiente.

## Evidencia y fuentes

Prueba `tests/microphone_feeder_test.cpp`: equivalencia de paquetes divididos, cuenta exacta de muestras, rango PCM, entrada inválida sin mutación, límite de cola, silencio por timeout/disconnect, reconexión, productor/lector concurrentes y rechazo RVM2 por generación/reservados/longitud. Ejecutada con clang C++17, AddressSanitizer y UndefinedBehaviorSanitizer. La cabecera también compiló con clang C11 y warnings como errores. Esto prueba el componente portable, no un driver ni su ejecución kernel.

- [Microsoft SYSVAD sample](https://learn.microsoft.com/en-us/samples/microsoft/windows-driver-samples/sysvad-virtual-audio-device-driver-sample/)
- [Microsoft SYSVAD fuente](https://github.com/microsoft/Windows-driver-samples/tree/main/audio/sysvad)
- [Microsoft: firma de drivers kernel](https://learn.microsoft.com/en-us/windows-hardware/drivers/install/kernel-mode-code-signing-requirements--windows-vista-and-later-)
- [Microsoft: tutorial de firma](https://learn.microsoft.com/en-us/windows-hardware/drivers/install/windows-driver-signing-tutorial)
