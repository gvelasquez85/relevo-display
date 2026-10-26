//go:build linux

package desktop

import (
	"context"
	"fmt"
	"github.com/godbus/dbus/v5"
	"strings"
	"sync/atomic"
	"time"
)

const portalName = "org.freedesktop.portal.Desktop"
const portalIface = "org.freedesktop.portal.ScreenCast"
const portalPath = dbus.ObjectPath("/org/freedesktop/portal/desktop")

var portalCounter atomic.Uint64

func portalSourceType(mode string, available uint32) (uint32, error) {
	var typ uint32 = 1
	if mode == "extend" {
		typ = 4
	} else if mode != "desktop" {
		return 0, fmt.Errorf("unsupported Linux capture mode %q", mode)
	}
	if available&typ == 0 {
		return 0, fmt.Errorf("desktop portal does not support %s (AvailableSourceTypes=%d); virtual extension requires a compositor with VIRTUAL support", mode, available)
	}
	return typ, nil
}
func portalResponse(body []interface{}) (map[string]dbus.Variant, error) {
	if len(body) != 2 {
		return nil, fmt.Errorf("malformed portal response")
	}
	code, ok := body[0].(uint32)
	if !ok {
		return nil, fmt.Errorf("invalid portal response code")
	}
	if code != 0 {
		return nil, fmt.Errorf("portal request declined or failed (response %d)", code)
	}
	out, ok := body[1].(map[string]dbus.Variant)
	if !ok {
		return nil, fmt.Errorf("invalid portal response results")
	}
	return out, nil
}
func portalNode(v dbus.Variant) (uint32, error) {
	var streams []struct {
		Node  uint32
		Props map[string]dbus.Variant
	}
	if err := dbus.Store([]interface{}{v.Value()}, &streams); err != nil {
		return 0, fmt.Errorf("decode portal streams: %w", err)
	}
	if len(streams) != 1 || streams[0].Node == 0 {
		return 0, fmt.Errorf("portal returned no unique stream")
	}
	return streams[0].Node, nil
}
func closePortalObject(conn *dbus.Conn, path dbus.ObjectPath, iface string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = conn.Object(portalName, path).CallWithContext(ctx, iface+".Close", 0).Err
}

// Subscribe to the predictable request path before making the method call: the
// portal may emit Response before the method reply arrives.
func portalRequest(ctx context.Context, conn *dbus.Conn, method string, args ...interface{}) (map[string]dbus.Variant, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	token := fmt.Sprintf("relevo%d_%d", time.Now().UnixNano(), portalCounter.Add(1))
	options := args[len(args)-1].(map[string]dbus.Variant)
	options["handle_token"] = dbus.MakeVariant(token)
	sender := strings.ReplaceAll(strings.TrimPrefix(conn.Names()[0], ":"), ".", "_")
	path := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sender + "/" + token)
	match := []dbus.MatchOption{dbus.WithMatchObjectPath(path), dbus.WithMatchInterface("org.freedesktop.portal.Request"), dbus.WithMatchMember("Response"), dbus.WithMatchSender(portalName)}
	signals := make(chan *dbus.Signal, 8)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)
	if err := conn.AddMatchSignalContext(ctx, match...); err != nil {
		return nil, err
	}
	defer conn.RemoveMatchSignal(match...)
	var returned dbus.ObjectPath
	if err := conn.Object(portalName, portalPath).CallWithContext(ctx, portalIface+"."+method, 0, args...).Store(&returned); err != nil {
		closePortalObject(conn, path, "org.freedesktop.portal.Request")
		return nil, fmt.Errorf("portal %s: %w", method, err)
	}
	if returned != path {
		closePortalObject(conn, returned, "org.freedesktop.portal.Request")
		return nil, fmt.Errorf("portal returned unexpected request path")
	}
	for {
		select {
		case <-ctx.Done():
			closePortalObject(conn, path, "org.freedesktop.portal.Request")
			return nil, ctx.Err()
		case s, ok := <-signals:
			if !ok {
				return nil, fmt.Errorf("portal connection closed")
			}
			if s != nil && s.Path == path {
				return portalResponse(s.Body)
			}
		}
	}
}
