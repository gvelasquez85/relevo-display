package wintray

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestLabSettingsValidation(t *testing.T) {
	for _, id := range []string{"pattern", "extend", "input", "audio", "microphone", "camera"} {
		if err := (LabSettings{Profile: id, FPS: 30, Quality: 70}).Validate(); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	for _, s := range []LabSettings{{Profile: "unknown", FPS: 30, Quality: 70}, {Profile: "pattern", FPS: 45, Quality: 70}, {Profile: "audio", FPS: 60, Quality: 99}, {Profile: "camera", FPS: 0, Quality: 70}} {
		if s.Validate() == nil {
			t.Fatalf("accepted invalid settings %+v", s)
		}
	}
}

func TestLabPreviewRejectsInvalidAndOversizedFrames(t *testing.T) {
	if _, _, _, err := decodeLabPreview([]byte("not jpeg")); err == nil {
		t.Fatal("accepted invalid JPEG")
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 641, 480)), nil); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := decodeLabPreview(b.Bytes()); err == nil {
		t.Fatal("accepted oversized JPEG")
	}
	b.Reset()
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{R: 255, A: 255})
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	pixels, w, h, err := decodeLabPreview(b.Bytes())
	if err != nil || w != 2 || h != 1 || len(pixels) != 8 {
		t.Fatalf("unexpected frame %dx%d %d %v", w, h, len(pixels), err)
	}
	if pixels[2] < 200 || pixels[3] != 255 {
		t.Fatalf("expected opaque BGRA red pixel, got %v", pixels[:4])
	}
}

func TestYamProfilesAreIndependentAndValidateOnlyCompatibleChecks(t *testing.T) {
	profiles := YamProfiles()
	if len(profiles) != 6 {
		t.Fatal("expected six GUI checks")
	}
	for _, p := range profiles {
		if err := ValidateYamSettings(LabSettings{Profile: p.ID, FPS: 60, Quality: 100}); err != nil {
			t.Fatal(err)
		}
	}
	profiles[0].ID = "mutated"
	if YamProfiles()[0].ID != "performance30" {
		t.Fatal("shared mutable profile list")
	}
	for _, s := range []LabSettings{{Profile: "camera", FPS: 30, Quality: 70}, {Profile: "performance30", FPS: 45, Quality: 70}, {Profile: "display", FPS: 30, Quality: 101}} {
		if ValidateYamSettings(s) == nil {
			t.Fatalf("accepted invalid check %+v", s)
		}
	}
	if (LabSettings{Profile: "performance30", FPS: 30, Quality: 70}).Validate() == nil {
		t.Fatal("native profiles accept Yam check")
	}
}

func TestLabActionRequiresMatchingRunningTrial(t *testing.T) {
	o := LabOptions{Action: func() error { return nil }, ActionProfile: "uac"}
	if !o.actionEnabled("uac", true, false, false, true) {
		t.Fatal("running UAC action disabled")
	}
	for _, c := range []struct {
		profile                            string
		busy, checking, closing, stoppable bool
	}{{"display", true, false, false, true}, {"uac", false, false, false, true}, {"uac", true, true, false, true}, {"uac", true, false, true, true}, {"uac", true, false, false, false}} {
		if o.actionEnabled(c.profile, c.busy, c.checking, c.closing, c.stoppable) {
			t.Fatalf("enabled invalid state %+v", c)
		}
	}
	o.Action = nil
	if o.actionEnabled("uac", true, false, false, true) {
		t.Fatal("enabled absent action")
	}
}
