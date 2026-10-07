package main

import (
	"reflect"
	"testing"
)

func TestAppendDeps(t *testing.T) {
	for _, tc := range []struct {
		name string
		deps []string
		add  []string
		want []string
	}{
		{"into an empty list", nil, []string{"curl"}, []string{"curl"}},
		{"several at once", []string{"curl"}, []string{"jq", "git"}, []string{"curl", "jq", "git"}},
		{"already listed", []string{"curl"}, []string{"curl"}, []string{"curl"}},
		{"with a constraint", []string{"curl"}, []string{"bash (>= 4.0)"}, []string{"curl", "bash (>= 4.0)"}},
		{"same name, different constraint", []string{"bash (>= 4.0)"}, []string{"bash (>= 5.0)"}, []string{"bash (>= 4.0)", "bash (>= 5.0)"}},
		{"empty entries are skipped", []string{"curl"}, []string{"", "jq"}, []string{"curl", "jq"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := appendDeps(tc.deps, tc.add); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("appendDeps() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRemoveDeps(t *testing.T) {
	for _, tc := range []struct {
		name   string
		deps   []string
		remove []string
		want   []string
	}{
		{"the only one", []string{"curl"}, []string{"curl"}, []string{}},
		{"one of several", []string{"curl", "jq", "git"}, []string{"jq"}, []string{"curl", "git"}},
		{"several at once", []string{"curl", "jq", "git"}, []string{"curl", "git"}, []string{"jq"}},
		{"not listed", []string{"curl"}, []string{"jq"}, []string{"curl"}},
		{"with a constraint", []string{"bash (>= 4.0)", "curl"}, []string{"bash (>= 4.0)"}, []string{"curl"}},
		{"the name alone doesn't match a constraint", []string{"bash (>= 4.0)"}, []string{"bash"}, []string{"bash (>= 4.0)"}},
		{"an alternative", []string{"nginx | apache2", "curl"}, []string{"nginx | apache2"}, []string{"curl"}},
		{"empty entries are skipped", []string{"curl", ""}, []string{""}, []string{"curl", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := removeDeps(tc.deps, tc.remove); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("removeDeps() = %q, want %q", got, tc.want)
			}
		})
	}
}
