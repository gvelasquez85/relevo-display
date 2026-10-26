package desktop

import (
	"context"
	"testing"
)

func TestMacRejectsExtendBeforeCapture(t *testing.T) {
	if _, err := Open(context.Background(), Options{Mode: "extend", Display: 1}); err == nil {
		t.Fatal("extend must require a native virtual-display implementation")
	}
}
func TestMacRejectsInvalidDisplay(t *testing.T) {
	if _, err := Open(context.Background(), Options{Mode: "desktop", Display: 0}); err == nil {
		t.Fatal("display must be positive")
	}
}
