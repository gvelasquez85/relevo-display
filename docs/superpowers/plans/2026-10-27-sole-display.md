# Pantalla virtual principal y única — 0.15.4

Problema reportado: al elegir «Mostrar sólo» en la pantalla Relevo, termina la sesión y la limpieza PnP elimina la única salida. Causas verificadas: rechazo de PRIMARY_DEVICE en captureDisplay, comparación exacta con geometría inicial en captura/entrada y retirada de adaptador sin salida alternativa.

Cambios: captura e input siguen identidad verificada con origen/tamaño actuales; principal válido; recursos GDI se recrean al cambiar geometría/principal; pausa/reintento acotados durante reconfiguración. Adquisición reutiliza virtual principal/única; sin otra pantalla adjunta inicia en0,0. Se mantiene frame negociado con ajuste proporcional. Contactos activos se liberan por el validador existente al cambiar geometría.

Seguridad de salida: EnsureSafeDetach comprueba salida alternativa activa, intenta recuperar topología extendida sólo si CCD confirma destino conectado, y verifica resultado. Si no puede recuperar conserva adaptador. Se aplica en PnP Close, aborto de apertura, desconexión explícita y restauraciones de Lease/backend. GUI Stop/Exit comprueba antes de cancelar vídeo; si no hay otra salida se mantiene sesión y explica motivo. Se ignoran resultados de comprobaciones de sesiones anteriores.

Pruebas: principal+reposicionamiento, bounds vivos, tolerancia a transición, adquisición/reutilización sin monitor físico, cierre sin retirar única salida, recuperación real frente a retorno falso, cancelación/solicitudes repetidas/stale en GUI; race/vet y buildWindows. El test de QR Vision macOS usa CPU para evitar dependencia de GPU durante cambios de pantallas; mismos píxeles y validación.

Límites: no se ejecutó físicamente Windows desde este Mac. No se certifica arranque antes de inicio de sesión ni escritorio seguro con iPad, ni auto-reconexión tras quitar el cable cuando es única pantalla. Si no queda receptor y no existe pantalla física, conservar adaptador evita retirada destructiva pero no permite mostrar vídeo sin receptor.
