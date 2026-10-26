// Package medialab provides an isolated, opt-in LAN audio diagnostic.
package medialab

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// New returns a token-protected handler and its per-run URL token.
func New() (http.Handler, string, error) {
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return nil, "", err
	}
	token := hex.EncodeToString(secret)
	return Handler(token), token, nil
}

// Handler serves only the diagnostic page and generated tone under /TOKEN/.
func Handler(token string) http.Handler {
	wav := Tone()
	var mu sync.Mutex
	var snapshot []byte
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; media-src 'self'; img-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
		if len(parts) != 2 || len(token) != 48 || subtle.ConstantTimeCompare([]byte(parts[0]), []byte(token)) != 1 {
			http.NotFound(w, r)
			return
		}
		if parts[1] == "snapshot" && r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
			mr, err := r.MultipartReader()
			if err != nil {
				http.Error(w, "Formulario inválido", 400)
				return
			}
			part, err := mr.NextPart()
			if err != nil || part.FormName() != "photo" {
				http.Error(w, "Falta foto", 400)
				return
			}
			data, err := io.ReadAll(part)
			if err != nil {
				http.Error(w, "Foto demasiado grande", 413)
				return
			}
			if _, err = mr.NextPart(); err != io.EOF {
				http.Error(w, "Envía una sola foto", 400)
				return
			}
			config, format, err := image.DecodeConfig(bytes.NewReader(data))
			if err != nil || (format != "jpeg" && format != "png") || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 4000000 {
				http.Error(w, "Sólo JPEG/PNG de hasta 4 millones de píxeles", 400)
				return
			}
			img, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				http.Error(w, "Imagen inválida", 400)
				return
			}
			var normalized bytes.Buffer
			if err := jpeg.Encode(&normalized, img, &jpeg.Options{Quality: 80}); err != nil {
				http.Error(w, "No se pudo convertir la foto", 400)
				return
			}
			mu.Lock()
			snapshot = normalized.Bytes()
			mu.Unlock()
			http.Redirect(w, r, "./", http.StatusSeeOther)
			return
		}
		if parts[1] == "clear" && r.Method == http.MethodPost {
			mu.Lock()
			snapshot = nil
			mu.Unlock()
			http.Redirect(w, r, "./", http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		switch parts[1] {
		case "":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeContent(w, r, "index.html", time.Time{}, strings.NewReader(page))
		case "snapshot.jpg":
			mu.Lock()
			data := append([]byte(nil), snapshot...)
			mu.Unlock()
			if len(data) == 0 {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "image/jpeg")
			http.ServeContent(w, r, "snapshot.jpg", time.Time{}, bytes.NewReader(data))
		case "tone.wav":
			w.Header().Set("Content-Type", "audio/wav")
			http.ServeContent(w, r, "tone.wav", time.Time{}, bytes.NewReader(wav))
		default:
			http.NotFound(w, r)
		}
	})
}

// Tone generates five seconds of quiet 440 Hz PCM16 mono at 22050 Hz.
func Tone() []byte {
	const rate = 22050
	const samples = rate * 5
	b := make([]byte, 44+samples*2)
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], rate)
	binary.LittleEndian.PutUint32(b[28:], rate*2)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], samples*2)
	for i := 0; i < samples; i++ {
		fade := math.Min(1, math.Min(float64(i)/2205, float64(samples-1-i)/2205))
		sample := int16(math.Sin(2*math.Pi*440*float64(i)/rate) * 3276 * fade)
		binary.LittleEndian.PutUint16(b[44+i*2:], uint16(sample))
	}
	return b
}

// PrivateURLs lists only private IPv4 interfaces; it does not change firewall rules.
func PrivateURLs(port, token string) ([]string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	var urls []string
	seen := map[string]bool{}
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil || ip.To4() == nil || !ip.IsPrivate() || ip.IsLoopback() {
			continue
		}
		host := ip.String()
		if seen[host] {
			continue
		}
		seen[host] = true
		urls = append(urls, fmt.Sprintf("http://%s/%s/", net.JoinHostPort(host, port), token))
	}
	return urls, nil
}

const page = `<!doctype html><html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Relevo · Laboratorio</title><style>body{font:18px sans-serif;max-width:650px;margin:32px auto;padding:0 20px;line-height:1.5}h1{font-size:28px}audio{width:100%}#result{background:#eee;padding:16px}</style></head><body><h1>Relevo · Laboratorio multimedia</h1><p>Prueba de transporte de audio desde el computador a Safari por la red local. Usa la misma red Wi-Fi privada. Al poner Safari en primer plano, Yam puede pausar la pantalla.</p><h2>1. Tono de prueba</h2><p>Baja el volumen y pulsa reproducir. Debes oír un tono suave de 5 segundos, generado en el computador. Puedes repetirlo con el control de audio.</p><audio controls preload="none" src="tone.wav">Este navegador no permite reproducir audio HTML.</audio><p>Esto verifica un archivo WAV por Wi-Fi. Todavía no transmite el sonido del escritorio, ni usa USB, ni comprueba latencia en tiempo real. Puedes seleccionar una salida Bluetooth desde iOS para comprobarla manualmente.</p><h2>2. Foto de prueba (no webcam en vivo)</h2><p>Safari puede ofrecer cámara o fototeca; el comportamiento en iOS 9 está pendiente de verificar. Usa un objeto sin datos personales. La foto viaja por HTTP sin cifrar y se conserva únicamente en RAM hasta otra foto, borrarla o cerrar el programa. No se guarda en disco; se eliminan metadatos al convertirla a JPEG. Máximo 5 MiB y 4 millones de píxeles.</p><form method="post" enctype="multipart/form-data" action="snapshot"><input type="file" name="photo" accept="image/*" capture="camera"><button type="submit">Enviar foto de prueba</button></form><p><a href="snapshot.jpg">Ver última foto en este navegador (actualizar manualmente)</a></p><form method="post" action="clear"><button type="submit">Borrar foto de RAM</button></form><h2>3. Capacidades de cámara del navegador</h2><div id="result">Comprobando APIs disponibles…</div><p>iOS 9.3.5 no dispone de la API web estándar necesaria para capturar la cámara. Esta prueba no abre la cámara ni solicita permisos. Una webcam de Windows requiere un cliente compatible en el iPad y un componente receptor en Windows.</p><p>No se envían datos a servicios externos. El enlace contiene una clave temporal: no lo compartas y cierra el laboratorio al terminar. Esta conexión HTTP local no está cifrada.</p><script>(function(){var modern=!!(navigator.mediaDevices&&navigator.mediaDevices.getUserMedia);var legacy=!!(navigator.getUserMedia||navigator.webkitGetUserMedia||navigator.mozGetUserMedia);document.getElementById('result').textContent='API estándar de cámara: '+(modern?'detectada':'no detectada')+'. API antigua: '+(legacy?'detectada':'no detectada')+'. Detección solamente; no confirma acceso a la cámara.';}());</script></body></html>`
