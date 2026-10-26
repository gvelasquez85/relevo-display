# Relevo: conexión Wi-Fi, propuesta v0.6

Fecha: 25 de octubre de 2026. Estado: investigación y selección de transporte implementada; conexión inalámbrica de Relevo todavía no implementada ni probada.

## Qué comprobamos

El diagnóstico local `dist/relevo-macos-arm64 --doctor` enumera simultáneamente conexiones `USB` y `Network`. El diagnóstico sólo consulta el servicio local: no abrió una sesión con el iPad ni transmitió imagen, teclado, audio o cámara. No se guardan aquí identificadores personales.

El host actual usa `internal/usbmux/client.go`: `Devices` conserva tipo e ID efímero, descarta la dirección de red; `Connect` siempre envía una solicitud USBMux `Connect`. El receptor propio `clients/ios/Sources/RVConnection.m` escucha únicamente `INADDR_LOOPBACK:52742`.

En la implementación primaria de [inetcat](https://github.com/libimobiledevice/libusbmuxd/blob/master/tools/inetcat.c), la rama Network convierte la dirección descubierta y abre un socket TCP directo al puerto solicitado; la rama USB usa `usbmuxd_connect`. Por tanto, listar Network no demuestra que el receptor de Relevo sea alcanzable ni que exista un túnel cifrado equivalente a USB. Cambiar sólo la selección de ID no habilita Wi-Fi.

La [documentación de inetcat](https://github.com/libimobiledevice/libusbmuxd/blob/master/docs/inetcat.1) requiere emparejamiento con el computador y sincronización Wi-Fi para descubrimiento Network; advierte posibles cierres para ahorrar energía. Apple documenta la [sincronización Wi-Fi con iTunes desde iOS 5](https://support.apple.com/en-gb/guide/itunes/itns3751d862/windows). Esto acredita descubrimiento/sincronización, no transporte protegido de nuestras funciones.

## Dos opciones

| Ruta | Ventaja | Límite y decisión |
| --- | --- | --- |
| Descubrimiento Apple con sincronización Wi-Fi | Aprovecha dispositivos ya emparejados | Depende de servicios Apple y plataforma. El socket directo de una app propia necesita autenticación/cifrado independientes. No activar como atajo. |
| Transporte propio autenticado | Misma lógica para Windows, Linux y Raspberry Pi; independiente de sincronización Apple | Requiere TLS, emparejamiento y pruebas de iOS 9. Es la ruta recomendada para producto. |

## Diseño recomendado, todavía pendiente

1. USB sigue siendo la opción por defecto. Wi-Fi se activa explícitamente en ambos extremos. No hay fallback automático de USB a LAN.
2. El computador ofrece un servidor TLS sólo mientras el usuario habilita Wi-Fi. El iPad inicia la conexión saliente: no hace falta abrir un listener LAN en el iPad. Sin nube, UPnP ni publicación en internet. Escuchar sólo en la interfaz local seleccionada; el instalador debe explicar el eventual permiso de firewall.
3. Emparejamiento preferido: intercambiar la identidad pública del computador y una credencial aleatoria por la conexión USB ya autorizada. No reutilizar los secretos ni registros de emparejamiento de Apple. Guardar credenciales en Keychain/almacenamiento protegido del host y permitir «Olvidar computador».
4. Alternativa sin cable: QR local con dirección, huella criptográfica del servidor y secreto aleatorio de un solo uso de al menos 192 bits, caducidad corta (propuesta: 120 segundos) y aprobación visible en el computador. La cámara puede leer QR en iOS 9; esto requiere su permiso y no implica compartir webcam. Validar la huella **antes** de enviar el secreto dentro de TLS. No usar un PIN corto como única autenticación ni registrar el QR/secreto. La entrada manual necesita un flujo equivalente diseñado y revisado, no rebajar la comprobación.
5. TLS 1.2 como mínimo compatible con iOS 9.3.5, con suites modernas compartidas y comprobación estricta de identidad fijada al emparejar. No aceptar cualquier certificado, no SSL/TLS 1.0/1.1, no excepción global ATS. Apple documenta [TLS hasta 1.2 en Secure Transport](https://developer.apple.com/documentation/security/secure-transport); las APIs están obsoletas para sistemas modernos, pero son candidatas para este cliente heredado. Compilar y probar el handshake en el iPad real antes de afirmar compatibilidad. Implementar transporte de forma reemplazable para sistemas nuevos.
6. Tras TLS y autenticación, negociar capacidades RVL1. Nunca procesar input, pantalla, cámara o micrófono de un peer no autorizado. Identidad emparejada no equivale a consentimiento permanente: cámara y micrófono mantienen sus interruptores e indicadores visibles, apagados al desconectar/background. Límites de intentos, plazos y tamaño antes de reservar memoria.
7. Descubrimiento Bonjour/mDNS opcional después: sólo anuncia disponibilidad y puerto, sin nombre personal, UDID, credenciales ni funciones sensibles. Descubrir no concede acceso; verificar la identidad emparejada siempre. La primera prueba puede usar la dirección del QR para evitar otra dependencia de descubrimiento en Windows.
8. Conexiones separadas o colas prioritarias para control, vídeo y audio; colas acotadas y descarte de vídeo antiguo. Medir latencia extremo a extremo, no sólo FPS enviados. Probar congestión de Wi-Fi y pausar vídeo en modo teclado/trackpad para reducir tráfico.
9. Desconexión/revocación: soltar todas las teclas/contactos, apagar medios, retirar la pantalla virtual y reorganizar el escritorio. Reautenticar en cada conexión; no reactivar automáticamente cámara o micrófono. No migrar una sesión a un transporte distinto sin acción del usuario.

## Código entregado

`internal/usbmux/transport.go`: `ChooseTransport(devices, id, "USB"|"Network")` selecciona sólo metadatos, obliga a tipo exacto, rechaza ambigüedad y nunca hace fallback. Pruebas de ID/tipo incompatibles, tipo desconocido y ausencia del transporte solicitado. No cambia `Choose`, no abre conexiones y no habilita Wi-Fi en CLI.

## Criterios antes de distribuir Wi-Fi

- iPad 2 con iOS 9.3.5: handshake TLS real, identidad correcta, credencial revocada/caducada, certificado distinto, host falso y cambio de red.
- Peer no autenticado: cero eventos de entrada o acceso a cámara, micrófono/pantalla; reintentos acotados y logs sin texto escrito ni secretos.
- Captura de tráfico de una sesión sintética autorizada: ausencia de contenido legible; validar identidad además de cifrado.
- Windows 11 y Linux/RPi: conexión local y controles de firewall acotados; aislamiento de clientes Wi-Fi debe generar un error comprensible.
- Latencia, pérdida, reconexión y suspensión/background: sin teclas pulsadas, pantalla fantasma ni grabación residual.

No se modificaron sincronización Apple, firewall, red, certificados de confianza ni permisos. No se habilitó un listener LAN ni se transmitieron datos personales.
