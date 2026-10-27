package session

import "math"

// InputEventSchema returns only approved field names and fixed type labels for
// recognized input events. It never returns field values, arbitrary key names,
// nested content or reflected type names. A false result must not be logged as
// the original event. Coordinates must be finite numbers when present; their
// units, origin and meaning remain deliberately unspecified.
func InputEventSchema(event map[string]interface{}) (map[string]string, bool) {
	name, ok := event["eventName"].(string)
	if !ok {
		return nil, false
	}
	switch name {
	case "SingleTap", "TwoFingerSingleTap", "SingleTap2nd", "Scrolled", "Dragged", "HoverEvent", "KeyboardEvent", "KeyboardPressEvent", "vKeyboardEvent", "vKeyboardSourceEvent", "Zoomed", "TwoFingerPanned", "TwoFingerRotated", "SwipeToChangeSpaceEvent", "DeviceResize":
	default:
		return nil, false
	}
	schema := make(map[string]string, 11)
	for _, key := range [...]string{"eventName", "x", "y", "width", "height", "mouseButton", "modifierFlags", "deltaX", "deltaY", "phase", "startOfDrag", "endOfDrag", "startOfScroll", "endOfScroll", "startOfPan", "endOfPan", "startOfZoom", "endOfZoom", "scale", "zoom", "keyCode", "pressed", "inputString", "modifiers", "keyboardLayout", "language", "touchType"} {
		value, present := event[key]
		if !present {
			continue
		}
		switch key {
		case "x", "y", "width", "height", "deltaX", "deltaY":
			if !finiteInputNumber(value) {
				return nil, false
			}
		}
		schema[key] = inputTypeLabel(value)
	}
	return schema, true
}

func finiteInputNumber(value interface{}) bool {
	switch n := value.(type) {
	case float64:
		return !math.IsNaN(n) && !math.IsInf(n, 0)
	case float32:
		return !math.IsNaN(float64(n)) && !math.IsInf(float64(n), 0)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	default:
		return false
	}
}

func inputTypeLabel(value interface{}) string {
	switch value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return "number"
	case map[string]interface{}:
		return "object"
	case []interface{}, []byte:
		return "array"
	default:
		return "other"
	}
}
