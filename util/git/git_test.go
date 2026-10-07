package git

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repo makes a git repo with one commit and returns its path
func repo(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	dir := t.TempDir()

	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "first"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %s: %s", strings.Join(args, " "), err, out)
		}
	}

	return dir
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %s: %s", strings.Join(args, " "), err, out)
	}
}

func TestDescribeUsesTheTag(t *testing.T) {
	dir := repo(t)
	git(t, dir, "tag", "v1.2.3")

	got, err := Describe(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "v1.2.3" {
		t.Errorf("Describe() = %q, want %q", got, "v1.2.3")
	}
}

func TestDescribeUntaggedRepoGivesTheCommitHash(t *testing.T) {
	dir := repo(t)

	// --always means a repo with no tags still yields a version string
	got, err := Describe(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Error("Describe() on an untagged repo returned nothing")
	}
	if strings.ContainsAny(got, " \n") {
		t.Errorf("Describe() = %q, want a bare hash with no whitespace", got)
	}
}

func TestDescribeDirtyTreeIsMarked(t *testing.T) {
	dir := repo(t)
	git(t, dir, "tag", "v1.0.0")

	// an uncommitted change must show in the version, so a package built
	// from a dirty tree is not mistaken for the release
	if err := exec.Command("touch", filepath.Join(dir, "changed")).Run(); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "changed")

	got, err := Describe(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "-dirty") {
		t.Errorf("Describe() = %q, want a -dirty suffix", got)
	}
}

func TestDescribeOutsideARepoErrors(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	if _, err := Describe(t.TempDir()); err == nil {
		t.Error("expected an error describing a directory that isn't a repo")
	}
}
