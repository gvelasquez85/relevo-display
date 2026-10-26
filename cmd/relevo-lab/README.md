# Relevo Lab: audio local y capacidades del navegador

Ejecutable de diagnóstico separado de Relevo Display. No cambia la instalación ni la app Yam del iPad.

Abre **Relevo-Lab-Windows.exe** con doble clic y pulsa Enter para iniciar la prueba LAN. Se abre el navegador de Windows y la consola muestra el enlace para Safari del iPad. No necesitas BAT ni instalar nada. También puedes abrir una terminal en la carpeta del ejecutable y ejecutar:

```powershell
.\Relevo-Lab-Windows.exe -lan
```

Para un acceso directo de Windows, establece como destino `"C:\ruta\Relevo-Lab-Windows.exe" -lan`. Mantén abierta su consola. Al ejecutarlo desde terminal, sin `-lan` (por ejemplo con `-listen 127.0.0.1:8765`) sólo escucha en este computador (`127.0.0.1:8765`). Para otro puerto, usa `-lan -listen 127.0.0.1:8877`.

1. Conecta Windows y el iPad a la misma Wi-Fi privada.
2. Abre en Safari del iPad uno de los enlaces privados que muestra la consola. Contiene una clave aleatoria nueva cada ejecución; no lo compartas.
3. Baja el volumen y pulsa reproducir. El audio es un WAV PCM16 mono, 22050 Hz, con un tono suave de cinco segundos generado en el computador.
4. Si tienes auriculares Bluetooth emparejados con el iPad, puedes seleccionarlos desde iOS y repetir la reproducción. La compatibilidad de esos auriculares con iOS es independiente.
5. Cierra con Ctrl+C. Si Windows pide una regla de firewall, usa sólo la red privada si corresponde; el programa no modifica el firewall. Redes con aislamiento de clientes pueden bloquear esta prueba.

Al abrir Safari en primer plano, Yam puede pausar su pantalla. La prueba no demuestra audio simultáneo con Yam, no captura audio del escritorio ni micrófono, no usa USB y no mide latencia continua. El informe de cámara detecta exclusivamente la presencia de APIs: no solicita permisos ni captura imágenes. Safari de iOS 9.3.5 carece de la API estándar web para capturar cámara. Crear una webcam de Windows exige otro cliente compatible en el iPad y un receptor/componente de cámara en Windows.

La conexión HTTP local no está cifrada. El servidor entrega página/tono/foto mediante GET/HEAD y permite POST sólo para la fotografía manual y su borrado. No sirve archivos arbitrarios ni usa servicios externos ni registra URLs de las solicitudes. Con `-lan` escucha en todas las interfaces IPv4; anuncia únicamente direcciones IPv4 privadas. Usa una red de confianza y cierra al terminar.

Compilar desde el repositorio:

```sh
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/Relevo-Lab-Windows.exe ./cmd/relevo-lab
```

## Foto estática experimental

La página ofrece un selector de cámara/fototeca y un botón explícito para enviar una foto de prueba. No es vídeo ni una webcam de Windows. El comportamiento del selector en Safari de iOS 9.3.5 requiere prueba real. Usa un objeto sin datos personales: HTTP no cifra la transferencia. Sólo se permite una imagen JPEG/PNG de hasta 5 MiB y 4 millones de píxeles. El servidor decodifica y vuelve a codificar JPEG, descartando metadatos; conserva únicamente la última foto en RAM. Desde el navegador de Windows, abre el mismo enlace temporal y pulsa «Ver última foto»; actualiza manualmente. «Borrar foto de RAM» elimina la copia del servidor; también desaparece al cerrar. La foto original y el historial/descargas del navegador quedan bajo control del usuario.
