package colour

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPainterP(t *testing.T) {
	on := Painter{On: true}
	off := Painter{On: false}

	if got, want := on.P(Red, "boom"), Red+"boom"+Reset; got != want {
		t.Errorf("on.P() = %q, want %q", got, want)
	}

	if got, want := off.P(Red, "boom"), "boom"; got != want {
		t.Errorf("off.P() = %q, want %q", got, want)
	}

	// an empty string is left alone, so a blank column carries no escapes
	if got := on.P(Red, ""); got != "" {
		t.Errorf("on.P(\"\") = %q, want empty", got)
	}

	// the zero value has colour off
	if got := (Painter{}).P(Red, "boom"); got != "boom" {
		t.Errorf("zero Painter painted: %q", got)
	}
}

func TestPainterPad(t *testing.T) {
	off := Painter{On: false}
	if got, want := off.Pad(Green, "ok", 9), "ok       "; got != want {
		t.Errorf("Pad() = %q, want %q", got, want)
	}

	// a string already at or over the width is not truncated
	if got, want := off.Pad(Green, "MISMATCH!!", 9), "MISMATCH!!"; got != want {
		t.Errorf("Pad() = %q, want %q", got, want)
	}

	// padding happens before painting, so the escapes sit outside the field
	// and do not count towards its width
	if got, want := (Painter{On: true}).Pad(Green, "ok", 4), Green+"ok  "+Reset; got != want {
		t.Errorf("Pad() = %q, want %q", got, want)
	}
}

func TestIsTerminal(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	t.Setenv("NO_COLOR", "")
	if IsTerminal(f) {
		t.Error("a regular file reported as a terminal")
	}

	// a closed file cannot be statted, which is not a terminal either
	closed, err := os.Create(filepath.Join(t.TempDir(), "closed"))
	if err != nil {
		t.Fatal(err)
	}
	closed.Close()
	if IsTerminal(closed) {
		t.Error("a closed file reported as a terminal")
	}
}

func TestIsTerminalHonoursNoColor(t *testing.T) {
	// /dev/tty is not available in every test environment, so check the
	// NO_COLOR short circuit against a character device that always is
	dev, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()

	os.Unsetenv("NO_COLOR")
	if !IsTerminal(dev) {
		t.Skipf("%s is not a character device here", os.DevNull)
	}

	// an empty NO_COLOR still counts as set, per the convention
	t.Setenv("NO_COLOR", "")
	if IsTerminal(dev) {
		t.Error("NO_COLOR set but colour still on")
	}

	if For(dev).On {
		t.Error("For() returned a painter with colour on despite NO_COLOR")
	}
}
