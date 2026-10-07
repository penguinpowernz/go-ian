package str

import (
	"os/exec"
	"reflect"
	"testing"
)

func TestCleanStrings(t *testing.T) {
	in := []string{"  a", "b  ", "\tc\n", "", "  "}
	want := []string{"a", "b", "c", "", ""}

	got := CleanStrings(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CleanStrings() = %q, want %q", got, want)
	}

	// it trims in place, so the caller's slice is updated too
	if !reflect.DeepEqual(in, want) {
		t.Errorf("input not trimmed in place: %q", in)
	}
}

func TestLines(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []string
	}{
		{"several lines", "a\nb\nc", []string{"a", "b", "c"}},
		{"trailing newline leaves an empty last line", "a\n", []string{"a", ""}},
		{"no newline", "a", []string{"a"}},
		{"empty string", "", []string{""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Lines(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Lines(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCommandString(t *testing.T) {
	cmd := exec.Command("/bin/echo", "hello", "world")
	if got, want := CommandString(cmd), "/bin/echo hello world"; got != want {
		t.Errorf("CommandString() = %q, want %q", got, want)
	}
}
