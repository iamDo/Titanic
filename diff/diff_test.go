package diff_test

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"titanic_app/diff"
)

// helper to compute md5 hex of given data
func hashData(data []byte) string {
	h := md5.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func TestListLocal(t *testing.T) {
	tmp := t.TempDir()
	// prepare files
	files := map[string][]byte{
		"a.txt":          []byte("hello world"),
		"subdir/binary.bin": []byte{0x00, 0x01, 0x02},
	}
	for rel, content := range files {
		full := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("failed to mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, content, 0644); err != nil {
			t.Fatalf("failed to write file %s: %v", rel, err)
		}
	}

	list, err := diff.ListLocal(tmp)
	if err != nil {
		t.Fatalf("ListLocal returned error: %v", err)
	}

	// convert to map for easy lookup
	got := make(map[string]string)
	for _, fh := range list {
		got[fh.Path] = fh.Hash
	}

	// check expected entries
	if len(got) != len(files) {
		t.Errorf("expected %d files, got %d", len(files), len(got))
	}
	for rel, content := range files {
		exHash := hashData(content)
		if h, ok := got[rel]; !ok {
			t.Errorf("missing entry for %s", rel)
		} else if h != exHash {
			t.Errorf("hash mismatch for %s: expected %s, got %s", rel, exHash, h)
		}
	}
}

func TestComputeDiff(t *testing.T) {
	testCases := []struct {
		name     string
		src      []diff.FileHash
		dst      []diff.FileHash
		expected []diff.Diff
	}{
		{
			name: "match and missing",
			src: []diff.FileHash{
				{Path: "foo.txt", Hash: "h1"},
				{Path: "bar.txt", Hash: "h2"},
			},
			dst: []diff.FileHash{
				{Path: "foo.txt", Hash: "h1"},
			},
			expected: []diff.Diff{
				{Path: "bar.txt", SrcHash: "h2", DstHash: "", Status: diff.MissingDestination},
				{Path: "foo.txt", SrcHash: "h1", DstHash: "h1", Status: diff.Match},
			},
		},
		{
			name: "missing source and mismatch",
			src: []diff.FileHash{
				{Path: "a", Hash: "x"},
			},
			dst: []diff.FileHash{
				{Path: "a", Hash: "y"},
				{Path: "b", Hash: "z"},
			},
			expected: []diff.Diff{
				{Path: "a", SrcHash: "x", DstHash: "y", Status: diff.Mismatch},
				{Path: "b", SrcHash: "", DstHash: "z", Status: diff.MissingSource},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			diffs := diff.ComputeDiff(tc.src, tc.dst)
			// compare lengths
			if len(diffs) != len(tc.expected) {
				t.Fatalf("%s: expected %d diffs, got %d", tc.name, len(tc.expected), len(diffs))
			}
			for i, exp := range tc.expected {
				got := diffs[i]
				if got.Path != exp.Path || got.SrcHash != exp.SrcHash || got.DstHash != exp.DstHash || got.Status != exp.Status {
					t.Errorf("%s[%d]: expected %+v, got %+v", tc.name, i, exp, got)
				}
			}
		})
	}
}
