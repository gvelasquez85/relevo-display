package winvirtual

import (
	"context"
	"errors"
	"testing"
)

func TestAcquireReusesPrimaryVirtualWithoutChangingTopology(t *testing.T) {
	f := fixture()
	f.displays[0].Mode.Attached = false
	f.displays[1].Primary = true
	f.displays[1].Mode.X = 0
	old := f.displays[1]
	l, e := Acquire(context.Background(), f, 1024, 768)
	if e != nil {
		t.Fatal(e)
	}
	if l.Mode != old.Mode || len(f.sets) != 0 {
		t.Fatal("primary topology rewritten")
	}
	if e = l.Close(); e != nil {
		t.Fatal(e)
	}
	if f.displays[1] != old || len(f.sets) != 0 {
		t.Fatal("borrowed primary restored")
	}
}
func TestAcquireHeadlessAttachesAtOrigin(t *testing.T) {
	f := fixture()
	f.displays[0].Mode.Attached = false
	f.displays[1].Mode.Attached = false
	l, e := Acquire(context.Background(), f, 1024, 768)
	if e != nil {
		t.Fatal(e)
	}
	if l.Mode.X != 0 || l.Mode.Y != 0 || !l.Mode.Attached {
		t.Fatal(l.Mode)
	}
}
func TestClosePreservesPrimaryPromotion(t *testing.T) {
	f := fixture()
	l, e := Acquire(context.Background(), f, 1024, 768)
	if e != nil {
		t.Fatal(e)
	}
	f.displays[1].Primary = true
	f.displays[1].Mode.X = 0
	old := f.displays[1]
	if e = l.Close(); e != nil {
		t.Fatal(e)
	}
	if f.displays[1] != old || len(f.sets) != 1 {
		t.Fatal("external primary overwritten")
	}
}

type recoveringBackend struct {
	*fakeBackend
	recover    func()
	recoverErr error
	recoveries int
}

func (f *recoveringBackend) RestoreOtherOutput(context.Context) error {
	f.recoveries++
	if f.recover != nil {
		f.recover()
	}
	return f.recoverErr
}
func TestSafeDetachRequiresAnotherVisibleOutput(t *testing.T) {
	f := fixture()
	f.displays[0].Mode.Attached = false
	f.displays[1].Primary = true
	err := EnsureSafeDetach(context.Background(), f)
	var keep *KeepDisplayActiveError
	if !errors.As(err, &keep) {
		t.Fatal("sole output not protected", err)
	}
	if len(f.sets) != 0 || !f.displays[1].Mode.Attached {
		t.Fatal("sole output changed")
	}
}
func TestSafeDetachVerifiesRecoveryRatherThanTrustingReturn(t *testing.T) {
	base := fixture()
	base.displays[0].Mode.Attached = false
	f := &recoveringBackend{fakeBackend: base}
	if EnsureSafeDetach(context.Background(), f) == nil {
		t.Fatal("accepted recovery without visible output")
	}
	f.recover = func() { base.displays[0].Mode.Attached = true }
	if err := EnsureSafeDetach(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	if f.recoveries != 2 {
		t.Fatal(f.recoveries)
	}
}
func TestSafeDetachDoesNotRecoverNormalExtension(t *testing.T) {
	f := &recoveringBackend{fakeBackend: fixture()}
	if err := EnsureSafeDetach(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	if f.recoveries != 0 {
		t.Fatal("unnecessary topology change")
	}
}
func TestSafeDetachProtectsTransientAllInactiveTopology(t *testing.T) {
	f := fixture()
	for i := range f.displays {
		f.displays[i].Mode.Attached = false
	}
	if err := EnsureSafeDetach(context.Background(), f); err == nil {
		t.Fatal("all-inactive transition allowed adapter retirement")
	}
}
func TestSafeDetachAllowsAlreadyInactiveVirtualWithVisiblePhysical(t *testing.T) {
	f := fixture()
	f.displays[1].Mode.Attached = false
	if err := EnsureSafeDetach(context.Background(), f); err != nil {
		t.Fatal(err)
	}
}
func TestHeadlessLeaseCloseNeverRemovesOnlyOutput(t *testing.T) {
	f := fixture()
	f.displays[0].Mode.Attached = false
	f.displays[1].Mode.Attached = false
	l, err := Acquire(context.Background(), f, 1024, 768)
	if err != nil {
		t.Fatal(err)
	}
	err = l.Close()
	var keep *KeepDisplayActiveError
	if !errors.As(err, &keep) {
		t.Fatal("headless lease allowed restore to no outputs", err)
	}
	if !f.displays[1].Mode.Attached || len(f.sets) != 1 {
		t.Fatal("headless output removed")
	}
}
func TestBorrowedPrimaryDetachedCloseDoesNotFalselyReportRemoval(t *testing.T) {
	f := fixture()
	f.displays[1].Primary = true
	l, err := Acquire(context.Background(), f, 1024, 768)
	if err != nil {
		t.Fatal(err)
	}
	if err = l.CloseDetached(); err == nil && f.displays[1].Mode.Attached {
		t.Fatal("borrowed primary falsely reported detached")
	}
}

// Cold boot may enumerate no physical display path at all, rather than a
// disconnected path. The sole virtual target must still attach at the origin.
func TestAcquireWithNoPhysicalDisplayEnumerated(t *testing.T) {
	f := fixture()
	f.displays = f.displays[1:]
	f.displays[0].Mode.Attached = false
	l, err := Acquire(context.Background(), f, 1024, 768)
	if err != nil {
		t.Fatal(err)
	}
	if l.Mode.X != 0 || l.Mode.Y != 0 || !l.Mode.Attached || l.Mode.Width != 1024 || l.Mode.Height != 768 {
		t.Fatal(l.Mode)
	}
	if err = l.Close(); err == nil {
		t.Fatal("removed the only output")
	}
	if !f.displays[0].Mode.Attached {
		t.Fatal("sole output detached")
	}
}
