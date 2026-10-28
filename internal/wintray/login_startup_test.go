package wintray

import "testing"

func TestLoginStartupCommandQuotesPath(t *testing.T) {
	got, err := loginStartupCommand(`C:\Program Files\Relevo\Relevo-Windows.exe`)
	if err != nil || got != `"C:\Program Files\Relevo\Relevo-Windows.exe" --startup` {
		t.Fatal(got, err)
	}
	for _, path := range []string{"", "a\"b", "a\x00b", "a\nb"} {
		if _, err := loginStartupCommand(path); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
}
func TestStartupLaunchFlagIsExact(t *testing.T) {
	for _, args := range [][]string{{"app"}, {"app", "--startup-other"}, {"--startup"}, {"app", "startup"}} {
		if hasStartupFlag(args) {
			t.Fatal(args)
		}
	}
	if !hasStartupFlag([]string{"app", "--startup"}) {
		t.Fatal("startup not recognized")
	}
}

func TestLegacyMigrationOnlyRecognizesOwnFixedCommand(t *testing.T) {
	if !legacyLoginCommand(`"C:\Users\Example\Relevo-Windows.exe" --startup`) {
		t.Fatal("known portable entry")
	}
	for _, s := range []string{`"C:\Other.exe" --startup`, `"C:\Relevo-Windows.exe" --setup`, `"C:\Relevo-Windows.exe" --startup --other`, `C:\Relevo-Windows.exe --startup`} {
		if legacyLoginCommand(s) {
			t.Fatal("unrelated entry", s)
		}
	}
}
