# Teclado operativo con Yam — 0.15.1

Solicitud: conservar pantalla, desconexión, touch/arrastre y gestos de varios dedos en la sesión normal y resolver entrada de teclado sin depender de la firma iOS.

Implementación: teclado Relevo dibujado en el monitor Windows enviado al iPad. Ventana no activable para conservar el campo de destino. Apertura tras un SingleTap inyectado correctamente y foco Edit/RichEdit confirmado, o botón manual «Abrir teclado». Cierre vinculado a sesión antes de retirar el monitor. Teclas Unicode españolas, controles y Mayús; sin almacenamiento de texto. No es el teclado nativo de iOS.

Conservar: controlador y protocolo de vídeo, scroll/zoom y arrastre existentes. No convertir gestos agregados de Yam en afirmación de contactos multitáctiles independientes. Cliente propio conserva su implementación separada.

Verificación: regresión de disparador tras toque correcto, cancelación de apertura, foco no editable, cierre idempotente de sesión, geometría y eventos del teclado; suite race y vet portátiles/Windows y compilación Windows x64. Foco y síntesis touch→clic de la ventana requieren validación física Windows/iPad.
