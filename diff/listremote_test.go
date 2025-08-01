package diff_test

import (
	"os/exec"
	"testing"

	"titanic_app/diff"
)

func TestListRemote(t *testing.T) {
	// Override ExecCommand to simulate ssh output
	orig := diff.ExecCommand
	defer func() { diff.ExecCommand = orig }()

	diff.ExecCommand = func(name string, args ...string) *exec.Cmd {
		// ignore name and args; print two md5sum entries
		return exec.Command("sh", "-c", `printf "aa111 fileA.txt\nbb222 sub/fileB.log\n"`)
	}

	list, err := diff.ListRemote("dummyHost:/some/path")
	if err != nil {
		t.Fatalf("ListRemote returned error: %v", err)
	}
	// Convert to map for assertion
	got := make(map[string]string)
	for _, fh := range list {
		got[fh.Path] = fh.Hash
	}

	expected := map[string]string{
		"fileA.txt":     "aa111",
		"sub/fileB.log": "bb222",
	}
	if len(got) != len(expected) {
		t.Fatalf("expected %d entries, got %d", len(expected), len(got))
	}
	for path, hash := range expected {
		if gotHash, ok := got[path]; !ok {
			t.Errorf("missing path %s in result", path)
		} else if gotHash != hash {
			t.Errorf("hash mismatch for %s: expected %s, got %s", path, hash, gotHash)
		}
	}
}