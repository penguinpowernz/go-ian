package main

import (
	"testing"
)

// the Args validator is what rejects flag combinations before any manifest is
// touched, so it is checked directly rather than by running the command
func TestAddArgsValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		update  bool
		all     bool
		args    []string
		wantErr bool
	}{
		{"a file given", false, false, []string{"usr/bin/app"}, false},
		{"no file given", false, false, nil, true},
		{"-u alone", true, false, nil, false},
		{"-u with a file", true, false, []string{"usr/bin/app"}, true},
		{"-a with a file", false, true, []string{"usr/bin/app"}, false},
		{"-a with no file", false, true, nil, true},
		// -u re-sums what each manifest already records, which is per arch by
		// definition, so -a has nothing to mean alongside it
		{"-u and -a together", true, true, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// the validator reads the flags off the command rather than
			// taking them as arguments, so they are set on it and reset after
			cmd := addCmd
			if err := cmd.Flags().Set("update", boolStr(tc.update)); err != nil {
				t.Fatal(err)
			}
			if err := cmd.Flags().Set("all-arches", boolStr(tc.all)); err != nil {
				t.Fatal(err)
			}
			defer func() {
				cmd.Flags().Set("update", "false")
				cmd.Flags().Set("all-arches", "false")
			}()

			err := cmd.Args(cmd, tc.args)
			if (err != nil) != tc.wantErr {
				t.Errorf("Args() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
