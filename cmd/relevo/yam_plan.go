package main

import (
	"relevo/internal/wintray"
	"time"
)

func yamPlan(base wintray.Settings, request wintray.LabSettings) (wintray.Settings, time.Duration, error) {
	if e := base.Validate(); e != nil {
		return base, 0, e
	}
	if e := wintray.ValidateYamSettings(request); e != nil {
		return base, 0, e
	}
	base.FPS = request.FPS
	base.Quality = request.Quality
	duration := 45 * time.Second
	switch request.Profile {
	case "performance30":
		base.FPS = 30
		duration = 20 * time.Second
	case "performance60":
		base.FPS = 60
		duration = 20 * time.Second
	case "display":
		duration = 20 * time.Second
	}
	return base, duration, nil
}
