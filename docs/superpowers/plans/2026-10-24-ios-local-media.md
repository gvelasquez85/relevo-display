# Prueba nativa local de medios

Autorización: German acepta crear app de desarrollo con cámara local y sonido antes de conectarla a Windows. Mínimo iOS9.3.5, iPad2armv7. Conservar Yam.

1. Implementar controlador UIKit/AVFoundation, inicio explícito, cámara local y tono finito, sin grabación/red/micrófono (worker ios_media_local).
2. Enlazar AVFoundation, propósito de cámara y versión0.2, actualizar script/documentación (Hannibal).
3. Revisar permisos, lifecycle, interrupciones y compatibilidad declarada; validar plist/script. No equiparar validación estructural a compilación.
4. Obtener SDK oficial armv7. Revisión local no encontró SDK/archivo Xcode y pestaña oficial sigue en login; se informó al usuario por pregunta asíncrona, sin solicitar contraseña.
5. Compilar, verificar arquitectura/minOS, firmar e instalar sólo cuando herramientas/perfil estén disponibles. Cámara/sonido físicos constituyen aceptación; mientras tanto entregar fuente revisable, no IPA falso.

La prueba de Windows0.10 se conserva. El laboratorio navegador no forma parte de esta app.
