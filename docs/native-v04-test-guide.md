# Relevo nativo0.4: alcance y prueba física

App propia para iPad2/iOS9.3.5; host independiente Windowsx64. Mantener Yam instalada y el host0.10 como referencia. Esta prueba necesita compilar/firmar/instalar el cliente propio; el ejecutable Windows no funciona con Yam.

## Implementado para ensayo

- Touch de un dedo: clic izquierdo, desplazamiento del puntero mientras arrastras, liberación/cancelación. No equivale a scroll multitáctil ni teclado. Coordenadas relativas a la imagen, validación de pantalla y escritorio de entrada antes de introducirlas en Windows. Sólo -mode desktop/extend con -touch. No usar privilegios elevados ni intentar interactuar con UAC.
- Audio del endpoint de reproducción multimedia predeterminado Windows por WASAPI loopback; no micrófono, sin driver de audio. Conversión a mono22050Hz PCM16, bloques20ms y colas acotadas. Se activa -audio en host y «Escuchar Windows» en iPad. Usar volumen bajo. No se garantiza sincronía audiovisual ni ausencia de cortes hasta medirlo.
- Cámara del iPad con autorización y botón «Enviar cámara USB»; JPEG hasta640×480 y10FPS objetivo, última imagen acotada. El host con -camera muestra URL de preview local y URL para OBS. No se guardan imágenes.

## Webcam para otras aplicaciones

El componente propio todavía no publica un dispositivo virtual de cámara Windows. Ruta opcional para ensayar el vídeo recibido con OBS Studio instalado por el usuario: añadir **Browser Source** con la URL OBS que imprime Relevo, tamaño640×480, y pulsar **Start Virtual Camera**. Elegir **OBS Virtual Camera** en la aplicación destino. Esta integración no fue ejecutada aquí ni se afirma compatibilidad con todas las aplicaciones. Relevo no instala OBS ni un driver de cámara de forma automática.

Fuentes oficiales: [Browser Source](https://obsproject.com/kb/browser-source), [Virtual Camera Guide](https://obsproject.com/kb/virtual-camera-guide). En iPad no se usa Safari; toda cámara/audio viaja entre apps nativas por USB. La página del host sólo presenta la preview o proporciona la fuente para OBS.

## Orden de prueba recomendado

1. Abrir clienteiPad, iniciar enlaceUSB y ejecutar hostsinflags: patrón animado20s. Verificar reconexión/parada.
2. `Relevo-Native-Windows.exe -mode desktop -touch -duration 1m`: tocar imagen, arrastrar una ventana de prueba, soltar y cancelar al desconectar. Repetir en pantalla extendida ya preparada usando -mode extend. No tocar datos importantes durante primerensayo.
3. `Relevo-Native-Windows.exe -audio -duration 1m`: pulsar EscucharWindows; reproducir sonido no privado enWindows y verificar altavoz/auricular del iPad. Parar audio, salir/reabrir app y cambiar endpoint para comprobar fallo visible.
4. `Relevo-Native-Windows.exe -camera -duration 1m`: activar cámaraen iPad y envíoUSB, abrir preview enWindows; probar parar/desconectar. Después ensayar puenteOBS siestádisponible.
5. Combinarflags sólo después de validarporseparado. AnotarFPS y cortes/retardoperceptible; no extrapolar del rendimiento delhostYam.

No hay ejecución física acreditada todavía. Tests simulados, compilación del host o compilación iOS no sustituyen estas comprobaciones.

Actualización 25/10/2026: OBS queda descartado como dependencia del producto. El puente descrito arriba corresponde sólo al prototipo anterior. Diseño de reemplazo integrado, multitáctil y teclado: `native-camera-touch-keyboard.md`. Aún no implementado.
