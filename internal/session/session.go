// Package session runs a bounded synthetic-video compatibility probe.
package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"net"
	"runtime"
	"time"

	"howett.net/plist"
	"relevo/internal/pattern"
	"relevo/internal/wire"
)

type Options struct {
	Duration         time.Duration
	Continuous       bool
	Width, Height    int
	FPS              int
	Log              func(string)
	HandshakeTimeout time.Duration
	PeerTimeout      time.Duration
	Progress         func(Stats)
	// Frame supplies a JPEG at the negotiated size. Nil uses the test pattern.
	Frame func(context.Context, int) ([]byte, error)
}
type Stats struct {
	InputEventSchemas      map[string]map[string]string `json:"input_event_schemas,omitempty"`
	PingP95Millis          float64                      `json:"control_roundtrip_recent_p95_ms"`
	InputEventCounts       map[string]int               `json:"input_event_counts,omitempty"`
	PeerInfo               bool                         `json:"peer_info"`
	Ready                  bool                         `json:"ready_to_receive"`
	FramesSent             int                          `json:"frames_sent"`
	BytesSent              int                          `json:"bytes_sent"`
	Pongs                  int                          `json:"pongs"`
	Events                 int                          `json:"events"`
	DecoderErrors          int                          `json:"decoder_errors"`
	ElapsedSeconds         float64                      `json:"video_elapsed_seconds"`
	ActualFPS              float64                      `json:"sender_fps"`
	CaptureMillis          float64                      `json:"capture_total_ms"`
	WriteMillis            float64                      `json:"video_write_total_ms"`
	CaptureP95Millis       float64                      `json:"capture_recent_p95_ms"`
	WriteP95Millis         float64                      `json:"write_recent_p95_ms"`
	FrameIntervalP95Millis float64                      `json:"send_interval_recent_p95_ms"`
}

func Run(ctx context.Context, c net.Conn, opt Options) (st Stats, err error) {
	defer c.Close()
	var captureTimes, writeTimes, intervalTimes, pingTimes recentTimings
	pingSent := make(map[uint32]time.Time)
	var videoWriter wire.VideoWriter
	var lastFrameSent time.Time
	var videoStarted time.Time
	var videoEnded time.Time
	updateStats := func() {
		st.PingP95Millis = pingTimes.p95()
		st.CaptureP95Millis = captureTimes.p95()
		st.WriteP95Millis = writeTimes.p95()
		st.FrameIntervalP95Millis = intervalTimes.p95()
		if !videoStarted.IsZero() {
			end := videoEnded
			if end.IsZero() {
				end = time.Now()
			}
			st.ElapsedSeconds = end.Sub(videoStarted).Seconds()
			if st.ElapsedSeconds > 0 {
				st.ActualFPS = float64(st.FramesSent) / st.ElapsedSeconds
			}
		}
	}
	defer updateStats()
	if (!opt.Continuous && opt.Duration <= 0) || opt.FPS < 1 || opt.FPS > 60 {
		return st, errors.New("duration must be positive and FPS must be 1..60")
	}
	if opt.Width == 0 && opt.Height == 0 {
		opt.Width, opt.Height = pattern.Width, pattern.Height
	}
	if opt.Width < 1 || opt.Height < 1 || opt.Width > 4096 || opt.Height > 4096 {
		return st, errors.New("invalid negotiated dimensions")
	}
	if opt.Frame == nil && (opt.Width != pattern.Width || opt.Height != pattern.Height) {
		return st, errors.New("custom dimensions require an image source")
	}
	log := opt.Log
	if log == nil {
		log = func(string) {}
	}
	frame := opt.Frame
	if frame == nil {
		frame = func(ctx context.Context, index int) ([]byte, error) { return pattern.JPEG(index) }
	}
	var cancel context.CancelFunc
	if opt.Continuous {
		ctx, cancel = context.WithCancel(ctx)
	} else {
		ctx, cancel = context.WithTimeout(ctx, opt.Duration+15*time.Second)
	}
	defer cancel()
	if opt.HandshakeTimeout <= 0 {
		opt.HandshakeTimeout = 10 * time.Second
	}
	if opt.PeerTimeout <= 0 {
		opt.PeerTimeout = 5 * time.Second
	}
	peerTimer := time.NewTimer(opt.PeerTimeout)
	defer peerTimer.Stop()
	var peerDeadline <-chan time.Time
	handshakeTimer := time.NewTimer(opt.HandshakeTimeout)
	defer handshakeTimer.Stop()
	var handshakeDeadline <-chan time.Time = handshakeTimer.C
	received := make(chan wire.Frame, 8)
	failed := make(chan error, 1)
	go func() {
		for {
			f, e := wire.Read(c)
			if e != nil {
				select {
				case failed <- e:
				default:
				}
				return
			}
			select {
			case received <- f:
			case <-ctx.Done():
				return
			}
		}
	}()
	send := func(f wire.Frame) error { c.SetWriteDeadline(time.Now().Add(2 * time.Second)); return wire.Write(c, f) }
	text := func(s string) error {
		log("TX control: " + s)
		return send(wire.Frame{Type: 101, Payload: wire.Text(s)})
	}
	ping := time.NewTicker(time.Second)
	defer ping.Stop()
	var tick *time.Ticker
	var frames <-chan time.Time
	var finish <-chan time.Time
	var finishTimer *time.Timer
	defer func() {
		if tick != nil {
			tick.Stop()
		}
		if finishTimer != nil {
			finishTimer.Stop()
		}
	}()
	hello := false
	settling := false
	tag := uint32(1)
	for {
		select {
		case <-ctx.Done():
			return st, fmt.Errorf("session ended: %w", ctx.Err())
		case <-peerDeadline:
			return st, errors.New("iPad not responding; display session disconnected")
		case <-handshakeDeadline:
			return st, errors.New("iPad handshake timed out: keep the compatible iPad app visible and close other desktop servers")
		case e := <-failed:
			if errors.Is(e, io.EOF) {
				return st, errors.New("iPad closed the display session")
			}
			return st, fmt.Errorf("iPad read: %w", e)
		case <-finish:
			if settling {
				// Catch already queued read failures before reporting transport completion.
				select {
				case e := <-failed:
					return st, fmt.Errorf("iPad final read: %w", e)
				default:
				}
				return st, nil
			}
			if st.FramesSent == 0 {
				return st, errors.New("no video frames sent")
			}
			if e := text("stopStream"); e != nil {
				return st, e
			}
			videoEnded = time.Now()
			frames = nil
			if tick != nil {
				tick.Stop()
			}
			ping.Stop()
			settling = true
			finishTimer = time.NewTimer(500 * time.Millisecond)
			finish = finishTimer.C
		case <-ping.C:
			if st.PeerInfo {
				if e := send(wire.Frame{Type: 102, Tag: tag}); e != nil {
					return st, e
				}
				if len(pingSent) >= 8 {
					var oldest uint32
					var first time.Time
					for k, t := range pingSent {
						if first.IsZero() || t.Before(first) {
							oldest = k
							first = t
						}
					}
					delete(pingSent, oldest)
				}
				pingSent[tag] = time.Now()
				tag++
			}
		case <-frames:
			captureStarted := time.Now()
			frameCtx, stopFrame := context.WithTimeout(ctx, 3*time.Second)
			b, e := frame(frameCtx, st.FramesSent)
			stopFrame()
			captureElapsed := time.Since(captureStarted)
			captureTimes.add(captureElapsed)
			st.CaptureMillis += float64(captureElapsed) / float64(time.Millisecond)
			if e != nil {
				return st, fmt.Errorf("capture source: %w", e)
			}
			if len(b) == 0 || len(b) > wire.MaxPayload-13 {
				return st, errors.New("invalid JPEG frame size")
			}
			config, decodeErr := jpeg.DecodeConfig(bytes.NewReader(b))
			if decodeErr != nil || config.Width != opt.Width || config.Height != opt.Height {
				return st, fmt.Errorf("JPEG dimensions do not match negotiated %dx%d", opt.Width, opt.Height)
			}
			writeStarted := time.Now()
			c.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if e = videoWriter.WriteJPEG(c, b); e != nil {
				return st, fmt.Errorf("iPad video write: %w", e)
			}
			writeElapsed := time.Since(writeStarted)
			writeTimes.add(writeElapsed)
			st.WriteMillis += float64(writeElapsed) / float64(time.Millisecond)
			sentAt := time.Now()
			if !lastFrameSent.IsZero() {
				intervalTimes.add(sentAt.Sub(lastFrameSent))
			}
			lastFrameSent = sentAt
			st.FramesSent++
			st.BytesSent += 13 + len(b)
			if st.FramesSent == 1 || st.FramesSent%opt.FPS == 0 {
				updateStats()
				log(fmt.Sprintf("TX JPEG video: %d frames, %.1f FPS sent", st.FramesSent, st.ActualFPS))
				if opt.Progress != nil {
					opt.Progress(st)
				}
			}
		case f := <-received:
			switch f.Type {
			case 100:
				var info map[string]interface{}
				if _, e := plist.Unmarshal(f.Payload, &info); e != nil {
					return st, fmt.Errorf("peer info plist: %w", e)
				}
				st.PeerInfo = true
				log("RX device information (personal fields omitted)")
				if !hello {
					hello = true
					for _, s := range []string{fmt.Sprintf("Version:0.1 OS:%s Hardware:YamTrial Protocol:1.0.20 H264:0", runtime.GOOS), "keyCodeLevel:0", "waitASec", "stopStream", fmt.Sprintf("size:%dx%d", opt.Width, opt.Height)} {
						if e := text(s); e != nil {
							return st, e
						}
					}
				}
			case 101:
				if _, e := wire.ParseText(f.Payload); e != nil {
					return st, fmt.Errorf("peer text: %w", e)
				}
				log("RX text message (content omitted)")
			case 102:
				if e := send(wire.Frame{Type: 103, Tag: f.Tag}); e != nil {
					return st, e
				}
			case 103:
				if sent, ok := pingSent[f.Tag]; ok {
					pingTimes.add(time.Since(sent))
					if !peerTimer.Stop() {
						select {
						case <-peerTimer.C:
						default:
						}
					}
					peerTimer.Reset(opt.PeerTimeout)
					delete(pingSent, f.Tag)
				}
				st.Pongs++
				log("RX pong")
			case 104:
				var event map[string]interface{}
				if _, e := plist.Unmarshal(f.Payload, &event); e != nil {
					return st, fmt.Errorf("event plist: %w", e)
				}
				st.Events++
				name, _ := event["eventName"].(string)
				switch name {
				case "ReadyToReceive":
					if !st.PeerInfo {
						return st, errors.New("ready event received before peer information")
					}
					if !st.Ready {
						handshakeTimer.Stop()
						handshakeDeadline = nil
						st.Ready = true
						peerTimer.Reset(opt.PeerTimeout)
						peerDeadline = peerTimer.C
						log("RX ReadyToReceive")
						if e := text("startStream:0"); e != nil {
							return st, e
						}
						videoStarted = time.Now()
						tick = time.NewTicker(time.Second / time.Duration(opt.FPS))
						frames = tick.C
						if !opt.Continuous {
							finishTimer = time.NewTimer(opt.Duration)
							finish = finishTimer.C
						}
					}
				case "ErrorDecoding":
					st.DecoderErrors++
					return st, errors.New("iPad reported ErrorDecoding: experimental payload rejected")
				case "DeviceTerminate", "DeviceLocked", "DeviceBackground":
					return st, errors.New("iPad app stopped or entered background; keep the compatible iPad app visible")
				case "SingleTap", "TwoFingerSingleTap", "SingleTap2nd", "Scrolled", "Dragged", "HoverEvent", "KeyboardEvent", "KeyboardPressEvent", "vKeyboardEvent", "vKeyboardSourceEvent", "Zoomed", "TwoFingerPanned", "TwoFingerRotated", "SwipeToChangeSpaceEvent":
					if st.InputEventCounts == nil {
						st.InputEventCounts = make(map[string]int)
					}
					st.InputEventCounts[name]++
					if schema, valid := InputEventSchema(event); valid {
						if st.InputEventSchemas == nil {
							st.InputEventSchemas = make(map[string]map[string]string)
						}
						if _, exists := st.InputEventSchemas[name]; !exists {
							st.InputEventSchemas[name] = schema
						}
					}
					if st.InputEventCounts[name] == 1 {
						log("RX input event: " + name + " (values omitted; not injected)")
					}
				case "DeviceResize":
					log("RX DeviceResize")
				default:
					log("RX input/lifecycle event (values omitted; not injected into computer)")
				}
			default:
				log(fmt.Sprintf("RX frame type %d (%d bytes)", f.Type, len(f.Payload)))
			}
		}
	}
}
