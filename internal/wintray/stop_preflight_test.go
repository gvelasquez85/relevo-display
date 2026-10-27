package wintray

import "testing"

func TestStopPreflightKeepsOneRequestAndCanUpgradeToExit(t *testing.T) {
	var guard stopPreflight
	guard.newSession()
	token, ok := guard.begin(false)
	if !ok {
		t.Fatal("first stop refused")
	}
	again, ok := guard.begin(true)
	if ok || again != token {
		t.Fatal("parallel stop was launched")
	}
	exit, ok := guard.complete(token)
	if !ok || !exit {
		t.Fatal("exit request lost")
	}
	if _, ok := guard.complete(token); ok {
		t.Fatal("duplicate completion may close twice")
	}
}
func TestStopPreflightIgnoresPreviousSessionCompletion(t *testing.T) {
	var guard stopPreflight
	guard.newSession()
	old, _ := guard.begin(true)
	guard.finishSession()
	guard.newSession()
	current, _ := guard.begin(false)
	if _, ok := guard.complete(old); ok {
		t.Fatal("old stop could cancel new session")
	}
	if exit, ok := guard.complete(current); !ok || exit {
		t.Fatal("current stop lost")
	}
}

func TestExitIntentSurvivesSessionEndingDuringCheck(t *testing.T) {
	var guard stopPreflight
	guard.newSession()
	token, _ := guard.begin(true)
	if !guard.finishSession() {
		t.Fatal("exit request lost when session ended first")
	}
	if _, ok := guard.complete(token); ok {
		t.Fatal("finished session accepted late stop")
	}
}
