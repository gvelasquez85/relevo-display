package session

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestInputSchemaOnlyApprovedTypes(t *testing.T) {
	event := map[string]interface{}{"eventName": "KeyboardEvent", "x": 12.5, "y": int64(-20), "phase": "private phase text", "mouseButton": uint64(1), "modifierFlags": uint32(4), "privateText": "secret keyboard", "private-key-name": "secret"}
	got, ok := InputEventSchema(event)
	want := map[string]string{"eventName": "string", "x": "number", "y": "number", "phase": "string", "mouseButton": "number", "modifierFlags": "number"}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("schema = %v, %v", got, ok)
	}
	b, _ := json.Marshal(got)
	for _, secret := range []string{"secret", "private", "KeyboardEvent", "12.5"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("leaked %q: %s", secret, b)
		}
	}
}

func TestInputSchemaRejectsUnknownAndInvalidCoordinates(t *testing.T) {
	for _, event := range []map[string]interface{}{
		nil, {}, {"eventName": "private event"}, {"eventName": []byte("SingleTap")},
		{"eventName": "SingleTap", "x": math.NaN()}, {"eventName": "SingleTap", "y": math.Inf(1)},
		{"eventName": "DeviceResize", "width": math.Inf(-1)}, {"eventName": "DeviceResize", "height": "768"},
		{"eventName": "Scrolled", "deltaX": true}, {"eventName": "Scrolled", "deltaY": nil},
	} {
		if got, ok := InputEventSchema(event); ok || got != nil {
			t.Fatalf("accepted invalid event schema %v", got)
		}
	}
}

func TestInputSchemaKnownNamesAndBounds(t *testing.T) {
	for _, name := range []string{"SingleTap", "TwoFingerSingleTap", "SingleTap2nd", "Scrolled", "Dragged", "HoverEvent", "KeyboardEvent", "KeyboardPressEvent", "vKeyboardEvent", "vKeyboardSourceEvent", "Zoomed", "TwoFingerPanned", "TwoFingerRotated", "SwipeToChangeSpaceEvent", "DeviceResize"} {
		event := map[string]interface{}{"eventName": name, "x": float32(-1.5), "y": uint(99), "width": int32(1024), "height": uint16(768), "deltaX": int8(-1), "deltaY": uint8(2), "phase": map[string]interface{}{"secret": "private"}}
		got, ok := InputEventSchema(event)
		if !ok || len(got) != 8 || got["phase"] != "object" {
			t.Fatalf("name %s schema %v %v", name, got, ok)
		}
	}
	event := map[string]interface{}{"eventName": "SingleTap", "phase": []interface{}{"secret"}, "mouseButton": nil, "modifierFlags": true}
	got, ok := InputEventSchema(event)
	if !ok || got["phase"] != "array" || got["mouseButton"] != "null" || got["modifierFlags"] != "boolean" {
		t.Fatalf("types %v %v", got, ok)
	}
}

func TestGestureAndKeyboardSchemaNeverStoresValues(t *testing.T) {
	event := map[string]interface{}{"eventName": "KeyboardEvent", "inputString": "private password", "modifiers": "private modifiers", "keyCode": uint64(4), "pressed": true, "startOfDrag": true, "endOfDrag": false, "scale": 1.12, "zoom": 1}
	schema, ok := InputEventSchema(event)
	if !ok || schema["inputString"] != "string" || schema["pressed"] != "boolean" || schema["scale"] != "number" {
		t.Fatalf("schema %v", schema)
	}
	b, _ := json.Marshal(schema)
	if strings.Contains(string(b), "private") || strings.Contains(string(b), "1.12") {
		t.Fatal("values leaked")
	}
}
