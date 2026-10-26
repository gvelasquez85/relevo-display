package winfocus

import "testing"

func TestEditablePolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    metadata
		want bool
	}{
		{"edit", metadata{"Edit", true, false, true, true}, true},
		{"rich", metadata{"RichEdit50W", true, false, true, true}, true},
		{"button", metadata{"Button", true, false, true, true}, false},
		{"browser", metadata{"Chrome_RenderWidgetHostHWND", true, false, true, true}, false},
		{"readonly", metadata{"Edit", true, true, true, true}, false},
		{"disabled", metadata{"Edit", false, false, true, true}, false},
		{"inactive", metadata{"Edit", true, false, false, true}, false},
		{"other point", metadata{"Edit", true, false, true, false}, false},
		{"class lookalike", metadata{"RichEditFake", true, false, true, true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := editable(tc.m); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
