//go:build windows

package wintray

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

type labRect struct{ Left, Top, Right, Bottom int32 }
type labPaint struct {
	DC                 uintptr
	Erase              int32
	Rect               labRect
	Restore, IncUpdate int32
	Reserved           [32]byte
}
type labBitmapInfo struct {
	Size                   uint32
	Width, Height          int32
	Planes, BitCount       uint16
	Compression, SizeImage uint32
	XPels, YPels           int32
	Used, Important        uint32
}

// RunLab runs the native-client laboratory in a taskbar window with a notification icon.
// Closing the window hides it; Exit cancels and waits for session cleanup.
func RunLab(o LabOptions) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	title := o.Title
	if title == "" {
		title = "Relevo Display · Pruebas del cliente iPad"
	}
	profiles := o.Profiles
	if len(profiles) == 0 {
		profiles = labProfiles
	}
	if len(profiles) > 6 {
		return fmt.Errorf("se admiten hasta seis pruebas")
	}
	validate := o.Validate
	if validate == nil {
		validate = func(s LabSettings) error { return validateLabSettings(s, profiles) }
	}
	intro := o.Intro
	if intro == "" {
		intro = "Requiere nuestra app Relevo abierta en el iPad por USB. Estas pruebas no funcionan con Yam."
	}
	footer := o.Footer
	if footer == "" {
		footer = "Wi-Fi y cámara/micrófono como dispositivos del sistema: todavía no disponibles. Cámara: sólo vista previa local."
	}
	const class = "RelevoNativeLaboratoryWindow"
	const progress = 0x8101
	const finished = 0x8102
	const tray = 0x8103
	const previewMessage = 0x8104
	k := syscall.NewLazyDLL("kernel32.dll")
	mutex, _, e := k.NewProc("CreateMutexW").Call(0, 0, uintptr(unsafe.Pointer(p(`Local\RelevoDisplay.NativeLaboratory`))))
	if mutex == 0 {
		return fmt.Errorf("abrir pruebas: %v", e)
	}
	defer syscall.CloseHandle(syscall.Handle(mutex))
	if e == syscall.Errno(183) {
		h := call("FindWindowW", uintptr(unsafe.Pointer(p(class))), 0)
		if h != 0 {
			call("ShowWindow", h, 9)
			call("SetForegroundWindow", h)
		}
		return nil
	}
	instance, _, _ := k.NewProc("GetModuleHandleW").Call(0)
	hi := monitorIcon(instance)
	if hi == 0 {
		return fmt.Errorf("crear icono de pruebas")
	}
	defer call("DestroyIcon", hi)
	var hwnd, status, fps, quality, stopButton, checkButton, actionButton uintptr
	var testField uintptr
	var currentProfile string
	var startButtons []uintptr
	var busy, closing, checking, destroyed bool
	var stop chan struct{}
	var workerDone chan struct{}
	var icon nid
	var mu sync.Mutex
	var alive, pending bool
	var pendingText string
	var workerResult error
	var diagnostic string
	var previewH uintptr
	var cameraActive bool
	var previewEpoch uint64
	var previewPending bool
	var latestPixels []byte
	var latestWidth, latestHeight int
	var paintPixels []byte
	var paintWidth, paintHeight int
	previewQuit := make(chan struct{})
	previewDone := make(chan struct{})
	var previewStarted bool
	clearPreview := func() {
		mu.Lock()
		cameraActive = false
		previewEpoch++
		latestPixels = nil
		latestWidth = 0
		latestHeight = 0
		mu.Unlock()
		// Session producers are serialized by Start cleanup. Drain queued frames before the next session.
		for o.Preview != nil {
			select {
			case _, ok := <-o.Preview:
				if !ok {
					goto drained
				}
			default:
				goto drained
			}
		}
	drained:
		paintPixels = nil
		paintWidth = 0
		paintHeight = 0
		if previewH != 0 {
			call("InvalidateRect", previewH, 0, 1)
			call("ShowWindow", previewH, 0)
		}
	}

	// The handle remains live until the worker has returned; reports are coalesced.
	report := func(s string) {
		mu.Lock()
		defer mu.Unlock()
		if !alive {
			return
		}
		pendingText = s
		if !pending {
			pending = true
			call("PostMessageW", hwnd, progress, 0, 0)
		}
	}
	setText := func(h uintptr, s string) { call("SetWindowTextW", h, uintptr(unsafe.Pointer(p(s)))) }
	logText := title + "\r\nListo. Elige una prueba. " + intro
	appendLog := func(s string) {
		s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
		logText += "\r\n" + s
		if len(logText) > 24000 {
			logText = logText[len(logText)-18000:]
			if n := strings.Index(logText, "\r\n"); n >= 0 {
				logText = logText[n+2:]
			}
		}
		setText(status, logText)
		call("SendMessageW", status, 0xb1, ^uintptr(0), ^uintptr(0)) // selection at end
		call("SendMessageW", status, 0xb7, 0, 0)
	}
	update := func() {
		if testField != 0 {
			enabled := uintptr(0)
			if busy && !checking && !closing && currentProfile == "input" && stop != nil {
				enabled = 1
			}
			call("EnableWindow", testField, enabled)
		}

		enabled := uintptr(1)
		if busy || closing {
			enabled = 0
		}
		for _, h := range append(append([]uintptr{}, startButtons...), fps, quality, checkButton) {
			call("EnableWindow", h, enabled)
		}
		en := uintptr(0)
		if busy && !checking && stop != nil {
			en = 1
		}
		call("EnableWindow", stopButton, en)
		actionEnabled := uintptr(0)
		if o.actionEnabled(currentProfile, busy, checking, closing, stop != nil) {
			actionEnabled = 1
		}
		if actionButton != 0 {
			call("EnableWindow", actionButton, actionEnabled)
		}
	}
	show := func() { call("ShowWindow", hwnd, 9); call("SetForegroundWindow", hwnd) }
	destroy := func() {
		mu.Lock()
		alive = false
		cameraActive = false
		latestPixels = nil
		mu.Unlock()
		if previewStarted {
			close(previewQuit)
			<-previewDone
			previewStarted = false
		}
		paintPixels = nil
		if previewH != 0 {
			call("DestroyWindow", previewH)
			previewH = 0
		}
		call("DestroyWindow", hwnd)
	}
	cancel := func() {
		if stop != nil {
			clearPreview()
			if testField != 0 {
				setText(testField, "")
			}
			close(stop)
			stop = nil
			appendLog("Deteniendo… Espera a que se liberen pantalla, entrada y audio.")
			update()
		}
	}
	quit := func() {
		closing = true
		if busy {
			cancel()
			appendLog("Cerrando al terminar la limpieza de la sesión…")
			update()
		} else {
			destroy()
		}
	}
	begin := func(profile string) {
		if busy || closing {
			return
		}
		if o.Start == nil {
			appendLog("No está disponible el motor de pruebas.")
			return
		}
		fi := int(call("SendMessageW", fps, 0x147, 0, 0))
		qi := int(call("SendMessageW", quality, 0x147, 0, 0))
		if fi < 0 || fi >= len(labFPS) || qi < 0 || qi >= len(labQuality) {
			appendLog("Selecciona FPS y calidad.")
			return
		}
		s := LabSettings{Profile: profile, FPS: labFPS[fi], Quality: labQuality[qi]}
		if err := validate(s); err != nil {
			appendLog(err.Error())
			return
		}
		clearPreview()
		if profile == "camera" {
			mu.Lock()
			cameraActive = true
			mu.Unlock()
			call("ShowWindow", previewH, 1)
			call("UpdateWindow", previewH)
		}
		currentProfile = profile
		busy = true
		checking = false
		stop = make(chan struct{})
		ch := stop
		workerDone = make(chan struct{})
		done := workerDone
		for _, p := range profiles {
			if p.ID == profile {
				appendLog("Iniciando: " + p.Label)
			}
		}
		update()
		go func() {
			err := o.Start(s, ch, report)
			mu.Lock()
			workerResult = err
			mu.Unlock()
			close(done)
			call("PostMessageW", hwnd, finished, 0, 0)
		}()
	}
	command := func(id uintptr) {
		if id >= 100 && id < 100+uintptr(len(profiles)) {
			begin(profiles[id-100].ID)
			return
		}
		switch id {
		case 1:
			cancel()
		case 2:
			if busy || closing {
				return
			}
			if o.Check == nil {
				appendLog("Diagnóstico USB no disponible.")
				return
			}
			busy = true
			checking = true
			workerDone = make(chan struct{})
			done := workerDone
			update()
			appendLog("Comprobando conexión USB…")
			go func() {
				s, err := o.Check()
				mu.Lock()
				workerResult = err
				diagnostic = s
				mu.Unlock()
				close(done)
				call("PostMessageW", hwnd, finished, 0, 0)
			}()
		case 3:
			quit()
		case 5:
			if err := SaveDiagnostic(logText); err != nil {
				message(hwnd, err.Error())
			}
		case 6:
			if !o.actionEnabled(currentProfile, busy, checking, closing, stop != nil) {
				return
			}
			if err := o.Action(); err != nil {
				appendLog(err.Error())
			}
		case 4:
			show()
		}
	}
	taskbarCreated := uint32(call("RegisterWindowMessageW", uintptr(unsafe.Pointer(p("TaskbarCreated")))))
	wnd := syscall.NewCallback(func(h uintptr, m uint32, w, l uintptr) uintptr {
		if taskbarCreated != 0 && m == taskbarCreated {
			shell.NewProc("Shell_NotifyIconW").Call(0, uintptr(unsafe.Pointer(&icon)))
			return 0
		}
		switch m {
		case 0x111:
			command(w & 0xffff)
			return 0
		case progress:
			mu.Lock()
			s := pendingText
			pending = false
			mu.Unlock()
			if busy {
				appendLog(s)
			}
			return 0
		case previewMessage:
			mu.Lock()
			pixels := latestPixels
			pw, ph := latestWidth, latestHeight
			latestPixels = nil
			previewPending = false
			allowed := cameraActive
			mu.Unlock()
			if allowed && len(pixels) > 0 {
				paintPixels = pixels
				paintWidth = pw
				paintHeight = ph
				call("InvalidateRect", previewH, 0, 1)
			}
			return 0
		case finished:
			if testField != 0 {
				setText(testField, "")
			}
			clearPreview()
			mu.Lock()
			err := workerResult
			s := diagnostic
			diagnostic = ""
			mu.Unlock()
			wasChecking := checking
			if checking && s != "" {
				appendLog(s)
			}
			busy = false
			stop = nil
			checking = false
			if err != nil {
				appendLog("Error: " + err.Error())
				show()
			} else if wasChecking {
				appendLog("Comprobación USB finalizada.")
			} else {
				appendLog("Prueba finalizada. Recursos liberados.")
			}
			update()
			if closing {
				destroy()
			}
			return 0
		case tray:
			if uint32(l) == 0x202 || uint32(l) == 0x203 {
				show()
				return 0
			}
			if uint32(l) == 0x205 {
				menu := call("CreatePopupMenu")
				defer call("DestroyMenu", menu)
				for _, it := range []struct {
					id   uintptr
					text string
				}{{4, "Abrir pruebas de Relevo"}, {1, "Detener prueba"}, {3, "Salir"}} {
					flags := uintptr(0)
					if it.id == 1 && (!busy || checking || stop == nil) {
						flags = 1
					}
					call("AppendMenuW", menu, flags, it.id, uintptr(unsafe.Pointer(p(it.text))))
				}
				var pt point
				call("GetCursorPos", uintptr(unsafe.Pointer(&pt)))
				call("SetForegroundWindow", h)
				id := call("TrackPopupMenu", menu, 0x102, uintptr(pt.X), uintptr(pt.Y), 0, h, 0)
				call("PostMessageW", h, 0, 0, 0)
				command(id)
			}
			return 0
		case 0x10:
			if o.Embedded {
				quit()
				return 0
			}
			call("ShowWindow", h, 0)
			return 0
		case 2:
			shell.NewProc("Shell_NotifyIconW").Call(2, uintptr(unsafe.Pointer(&icon)))
			destroyed = true
			if !o.Embedded {
				call("PostQuitMessage", 0)
			}
			return 0
		}
		return call("DefWindowProcW", h, uintptr(m), w, l)
	})
	name := p(class)
	c := wc{Proc: wnd, Instance: instance, Name: name, Icon: hi, SmallIcon: hi, Cursor: call("LoadCursorW", 0, 32512), Background: 16}
	c.Size = uint32(unsafe.Sizeof(c))
	if call("RegisterClassExW", uintptr(unsafe.Pointer(&c))) == 0 {
		return fmt.Errorf("registrar ventana de pruebas")
	}
	defer call("UnregisterClassW", uintptr(unsafe.Pointer(name)), instance)
	hwnd = call("CreateWindowExW", 0x40000, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(p(title))), 0x00ca0000, 0x80000000, 0x80000000, 740, 765, 0, 0, instance, 0)
	if hwnd == 0 {
		return fmt.Errorf("crear ventana de pruebas")
	}

	// A native owned preview window shares this UI thread and never writes images to disk.
	previewName := p("RelevoNativeCameraPreview")
	gdi := syscall.NewLazyDLL("gdi32.dll")
	previewProc := syscall.NewCallback(func(h uintptr, m uint32, w, l uintptr) uintptr {
		switch m {
		case 0x10:
			call("ShowWindow", h, 0)
			return 0
		case 2:
			return 0 // An owned preview must not terminate the main message loop.
		case 0x0f:
			var ps labPaint
			dc := call("BeginPaint", h, uintptr(unsafe.Pointer(&ps)))
			defer call("EndPaint", h, uintptr(unsafe.Pointer(&ps)))
			var r labRect
			call("GetClientRect", h, uintptr(unsafe.Pointer(&r)))
			call("FillRect", dc, uintptr(unsafe.Pointer(&r)), uintptr(1))
			if len(paintPixels) > 0 && paintWidth > 0 && paintHeight > 0 {
				width, height := int(r.Right-r.Left), int(r.Bottom-r.Top)
				dw := width
				dh := width * paintHeight / paintWidth
				if dh > height {
					dh = height
					dw = height * paintWidth / paintHeight
				}
				if dw > 0 && dh > 0 {
					info := labBitmapInfo{Size: 40, Width: int32(paintWidth), Height: -int32(paintHeight), Planes: 1, BitCount: 32}
					gdi.NewProc("StretchDIBits").Call(dc, uintptr((width-dw)/2), uintptr((height-dh)/2), uintptr(dw), uintptr(dh), 0, 0, uintptr(paintWidth), uintptr(paintHeight), uintptr(unsafe.Pointer(&paintPixels[0])), uintptr(unsafe.Pointer(&info)), 0, 0x00cc0020)
				}
			} else {
				text := "En el iPad, activa Enviar cámara USB."
				call("DrawTextW", dc, uintptr(unsafe.Pointer(p(text))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), 0x25)
			}
			return 0
		case 5:
			call("InvalidateRect", h, 0, 1)
			return 0
		}
		return call("DefWindowProcW", h, uintptr(m), w, l)
	})
	pc := wc{Proc: previewProc, Instance: instance, Name: previewName, Icon: hi, SmallIcon: hi, Cursor: call("LoadCursorW", 0, 32512), Background: 1}
	pc.Size = uint32(unsafe.Sizeof(pc))
	if call("RegisterClassExW", uintptr(unsafe.Pointer(&pc))) == 0 {
		destroy()
		return fmt.Errorf("registrar vista de cámara")
	}
	defer call("UnregisterClassW", uintptr(unsafe.Pointer(previewName)), instance)
	previewH = call("CreateWindowExW", 0, uintptr(unsafe.Pointer(previewName)), uintptr(unsafe.Pointer(p("Relevo · Cámara del iPad (vista previa)"))), 0x00cf0000, 0x80000000, 0x80000000, 660, 520, hwnd, 0, instance, 0)
	if previewH == 0 {
		destroy()
		return fmt.Errorf("crear vista de cámara")
	}
	hf, _, _ := syscall.NewLazyDLL("gdi32.dll").NewProc("GetStockObject").Call(17)
	child := func(class, text string, style uintptr, x, y, w, h int, id uintptr) uintptr {
		ch := call("CreateWindowExW", 0, uintptr(unsafe.Pointer(p(class))), uintptr(unsafe.Pointer(p(text))), 0x50000000|style, uintptr(x), uintptr(y), uintptr(w), uintptr(h), hwnd, id, instance, 0)
		call("SendMessageW", ch, 0x30, hf, 1)
		return ch
	}
	child("STATIC", title, 0, 24, 18, 680, 24, 0)
	child("STATIC", intro, 0, 24, 49, 680, 40, 0)
	child("STATIC", "Fluidez (FPS)", 0, 24, 95, 150, 20, 0)
	child("STATIC", "Calidad JPEG", 0, 210, 95, 180, 20, 0)
	fps = child("COMBOBOX", "", 0x210003, 24, 119, 150, 130, 20)
	quality = child("COMBOBOX", "", 0x210003, 210, 119, 180, 180, 21)
	for _, f := range labFPS {
		call("SendMessageW", fps, 0x143, 0, uintptr(unsafe.Pointer(p(fmt.Sprint(f)))))
	}
	for _, q := range labQuality {
		call("SendMessageW", quality, 0x143, 0, uintptr(unsafe.Pointer(p(fmt.Sprint(q)))))
	}
	call("SendMessageW", fps, 0x14e, 0, 0)
	call("SendMessageW", quality, 0x14e, 1, 0)
	for i, profile := range profiles {
		startButtons = append(startButtons, child("BUTTON", profile.Label, 0x10000, 24, 164+i*42, 680, 34, 100+uintptr(i)))
	}
	stopButton = child("BUTTON", "&Detener prueba", 0x10000, 24, 424, 200, 36, 1)
	checkButton = child("BUTTON", "&Comprobar USB", 0x10000, 242, 424, 220, 36, 2)
	exitLabel := "&Salir"
	if o.Embedded {
		exitLabel = "Volver a Relevo"
	}
	child("BUTTON", exitLabel, 0x10000, 480, 424, 224, 36, 3)
	child("STATIC", "Estado y resultados", 0, 24, 477, 680, 20, 0)
	statusHeight := 132
	if o.InputField {
		statusHeight = 72
	}
	status = child("EDIT", logText, 0x211844, 24, 503, 680, statusHeight, 30) // multiline, read-only, vertical scroll, tab stop
	if o.InputField {
		child("STATIC", "Campo de prueba: toca y escribe; el contenido se borra al detener.", 0, 24, 579, 680, 20, 0)
		testField = child("EDIT", "", 0x211044, 24, 602, 680, 40, 50)
		call("SendMessageW", testField, 0xc5, 4096, 0) // EM_LIMITTEXT; never read/export typed content.
	}
	child("STATIC", footer, 0, 24, 646, 680, 36, 0)
	child("BUTTON", "Exportar diagnóstico de pruebas", 0x10000, 24, 687, 320, 32, 5)
	if o.Action != nil {
		label := o.ActionLabel
		if label == "" {
			label = "Ejecutar comprobación"
		}
		actionButton = child("BUTTON", label, 0x10000, 368, 687, 336, 32, 6)
	}
	icon = nid{H: hwnd, ID: 1, Flags: 7, Callback: tray, Icon: hi}
	icon.Size = uint32(unsafe.Sizeof(icon))
	copy(icon.Tip[:], syscall.StringToUTF16("Relevo · Pruebas"))
	r, _, _ := shell.NewProc("Shell_NotifyIconW").Call(0, uintptr(unsafe.Pointer(&icon)))
	if r == 0 {
		destroy()
		return fmt.Errorf("crear icono en notificaciones")
	}
	mu.Lock()
	alive = true
	mu.Unlock()

	previewStarted = true
	go func() {
		defer close(previewDone)
		for {
			mu.Lock()
			epoch := previewEpoch
			mu.Unlock()
			select {
			case <-previewQuit:
				return
			case data, ok := <-o.Preview:
				if !ok {
					return
				}
				mu.Lock()
				allowed := alive && cameraActive && epoch == previewEpoch
				mu.Unlock()
				if !allowed {
					continue
				}
				pixels, w, h, err := decodeLabPreview(data)
				if err != nil {
					continue
				}
				mu.Lock()
				if alive && cameraActive && epoch == previewEpoch {
					latestPixels = pixels
					latestWidth = w
					latestHeight = h
					if !previewPending {
						previewPending = true
						call("PostMessageW", hwnd, previewMessage, 0, 0)
					}
				}
				mu.Unlock()
			}
		}
	}()
	update()
	call("ShowWindow", hwnd, 1)
	call("UpdateWindow", hwnd)
	var m msg
	for {
		if o.Embedded && destroyed {
			break
		}
		r, _, err := user.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) == -1 {
			cancel()
			if workerDone != nil {
				<-workerDone
			}
			destroy()
			return fmt.Errorf("interfaz de pruebas: %v", err)
		}
		if r == 0 {
			break
		}
		if call("IsDialogMessageW", hwnd, uintptr(unsafe.Pointer(&m))) == 0 {
			call("TranslateMessage", uintptr(unsafe.Pointer(&m)))
			call("DispatchMessageW", uintptr(unsafe.Pointer(&m)))
		}
	}
	return nil
}
