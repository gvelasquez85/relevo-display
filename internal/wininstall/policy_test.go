package wininstall

import "testing"

func TestInstallerRejectsUnrecognizedOperation(t *testing.T) {
	for _, a := range []string{"", "install", "repair", "uninstall"} {
		if validateAction(a) != nil {
			t.Fatal(a)
		}
	}
	for _, a := range []string{"delete-all", "install --service", "../uninstall"} {
		if validateAction(a) == nil {
			t.Fatal(a)
		}
	}
}
func TestInstallationCommandQuotesExecutable(t *testing.T) {
	if got := installCommand(`C:\Program Files\Relevo Display\Relevo-Windows.exe`, "--uninstall"); got != `"C:\Program Files\Relevo Display\Relevo-Windows.exe" --uninstall` {
		t.Fatal(got)
	}
}

func TestInstallationOwnershipAndPendingRemoval(t *testing.T) {
	if err := validateManifest([]byte(`{"Product":"RelevoDisplay","Version":"0.16"}`)); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{`{}`, `{"Product":"Another app"}`, `{"Product":"RelevoDisplay","pending_removal":true}`, `broken`} {
		if validateManifest([]byte(data)) == nil {
			t.Fatal("accepted unowned or pending installation", data)
		}
	}
}
