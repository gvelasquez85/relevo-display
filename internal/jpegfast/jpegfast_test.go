package jpegfast

import "testing"

func TestValidate(t *testing.T) {
	for _, tc := range []struct{ w, h, s, q, n int }{{0, 1, 4, 70, 4}, {1, 0, 4, 70, 4}, {4097, 1, 16388, 70, 16388}, {1, 1, 3, 70, 4}, {1, 2, 4, 70, 7}, {1, 1, 4, 0, 4}, {1, 1, 4, 101, 4}, {1, 1, 16385, 70, 4}} {
		if validate(make([]byte, tc.n), tc.w, tc.h, tc.s, tc.q) == nil {
			t.Errorf("accepted %+v", tc)
		}
	}
	if err := validate(make([]byte, 12), 1, 2, 8, 70); err != nil {
		t.Fatal(err)
	}
}
