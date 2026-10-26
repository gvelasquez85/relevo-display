# Desarrollo paralelo del enlace nativo

Usuario autoriza avanzar código mientras se descarga SDK; no implica que compilar elimine las pruebas de firma/instalación.

1. Fijar protocolo propio RVL1 acotado y puerto52742, sin dependencia Yam (specnative-transport).
2. Implementar hostGo CLI independiente con usbmux existente, patrón1024x768 y captura opcional, handshake+ack y una sola imagen pendiente; pruebas net.Pipe y límites (native_host).
3. Implementar listener iOSloopback, lectura/escritura robusta, validaciones y presentaciónJPEG, botonesUSB y cleanup conservando prueba localmedios (ios_media_local).
4. Integrar fuentes en proyecto/script, verificar estructuras y empaquetar fuentes+hostWindows. Revisar contrato cruzado y lifecycle.
5. Con SDKoficial completo/verificado: extraer SDK, compilararmv7, comprobarbinario. Firma e instalación requieren perfil válido; pruebasfísicas pendientes.

Fuera de este hito: inyecciónWindows de touch, audioWindows continuo, camvirtualWindows, transporte cámara/audio; tiposreservados no se anuncian como soporte. No sustituir hostYamfuncional0.10.
