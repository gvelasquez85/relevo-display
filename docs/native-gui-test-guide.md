# Relevo: pruebas desde Windows 0.7

Abre Relevo-Pruebas-Windows-0.7.exe con doble clic. La ventana y el icono junto al reloj permiten controlar las pruebas sin comandos ni archivos BAT.

Este laboratorio requiere nuestra app Relevo 0.6 abierta en el iPad por USB. No funciona con el cliente Yam. El IPA incluido está sin firmar; la verificación de Apple en el iPad sigue pendiente. Esta versión experimental es independiente de la aplicación existente compatible con Yam.

1. Conecta y desbloquea el iPad. Pulsa **Comprobar USB**. El diagnóstico distingue USB de red; detectar el dispositivo no confirma que la app esté instalada o verificada.
2. Elige 30 o 60 FPS y calidad JPEG entre 60 y 100.
3. Pulsa una prueba:
   - **Pantalla de prueba**: patrón de vídeo para comprobar recepción y fluidez.
   - **Pantalla extendida + entrada**: escritorio adicional, multitouch, teclado y mouse. Requiere el controlador de pantalla virtual ya instalado.
   - **Touch/teclado/mouse**: prueba de entrada sobre la pantalla actual. El teclado automático sólo reconoce campos nativos Edit/RichEdit de Windows; los modos dedicados del iPad permiten activarlo manualmente. Los campos personalizados de navegador u otras apps requieren trabajo adicional.
   - **Escuchar Windows en iPad**: salida de audio del computador hacia el iPad.
   - **Escuchar micrófono del iPad**: monitor de escucha local. Usa auriculares para evitar acople. No crea un micrófono seleccionable en otras aplicaciones.
   - **Cámara del iPad**: abre una ventana de vista previa dentro de Relevo. Activa la cámara y el envío USB en el iPad; acepta su permiso de cámara. No crea una webcam seleccionable por otras aplicaciones.
4. Observa el estado y los contadores en la misma ventana. Pulsa **Detener prueba** antes de cambiar de función. La interfaz espera la limpieza de la sesión; cada prueba tiene un límite de cinco minutos.
5. **Salir** detiene la sesión y cierra. Cerrar la ventana la oculta; el icono junto al reloj permite volver a abrirla.

Micrófono y reproducción USB se prueban por separado. Wi-Fi, cámara y micrófono como dispositivos del sistema y el instalador unificado siguen pendientes. No requiere OBS.

Verificación realizada: pruebas automatizadas con detector de carreras, análisis estático y compilación Windows amd64. Incluye una prueba de dos sesiones consecutivas con un dispositivo simulado para comprobar cancelación, cierre y reutilización. La interfaz y los medios todavía requieren pruebas físicas en Windows y el iPad; la compilación no acredita ese funcionamiento.
