package str

import "strings"

// CleanStrings will run strings.TrimSpace over a whole
// string array
func CleanStrings(strs []string) []string {
	for i, s := range strs {
		strs[i] = strings.TrimSpace(s)
	}

	return strs
}

// Lines splits up the given string by newline
func Lines(s string) []string {
	return strings.Split(s, "\n")
}
