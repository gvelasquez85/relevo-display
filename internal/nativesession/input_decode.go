package nativesession

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"relevo/internal/remoteinput"
	"unicode/utf8"
)

func strictJSON(b []byte, dest any) error {
	if !utf8.Valid(b) {
		return fmt.Errorf("invalid input UTF8")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e := d.Decode(dest); e != nil {
		return e
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return fmt.Errorf("trailing input JSON")
	}
	return nil
}
func decodeContacts(b []byte) ([]remoteinput.Contact, error) {
	var in struct {
		Contacts []struct {
			ID    uint32   `json:"id"`
			Phase string   `json:"phase"`
			X     *float64 `json:"x"`
			Y     *float64 `json:"y"`
		} `json:"contacts"`
	}
	if e := strictJSON(b, &in); e != nil {
		return nil, e
	}
	if len(in.Contacts) == 0 || len(in.Contacts) > remoteinput.MaxContacts {
		return nil, fmt.Errorf("invalid contact count")
	}
	out := make([]remoteinput.Contact, 0, len(in.Contacts))
	for _, c := range in.Contacts {
		if c.X == nil || c.Y == nil {
			return nil, fmt.Errorf("missing contact coordinates")
		}
		out = append(out, remoteinput.Contact{ID: c.ID, Phase: c.Phase, X: *c.X, Y: *c.Y})
	}
	return out, nil
}
func decodeKey(b []byte) (remoteinput.Key, error) {
	var k remoteinput.Key
	if e := strictJSON(b, &k); e != nil {
		return k, e
	}
	return k, k.Validate()
}
