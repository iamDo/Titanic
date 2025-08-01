// Package diff provides file hashing and diff computation utilities.
package diff

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// FileHash holds a file path and its MD5 hash.
type FileHash struct {
	Path string
	Hash string
}

// DiffStatus represents the status of a file compared between source and destination.
type DiffStatus int

const (
	// Match indicates the file hashes are identical.
	Match DiffStatus = iota
	// MissingSource indicates the file is missing in the source.
	MissingSource
	// MissingDestination indicates the file is missing in the destination.
	MissingDestination
	// Mismatch indicates the file hashes differ.
	Mismatch
)

// Diff represents the comparison result for a single file.
type Diff struct {
	Path    string
	SrcHash string
	DstHash string
	Status  DiffStatus
}

// ExecCommand is the function used to invoke external commands (e.g., ssh). Can be overridden in tests.
var ExecCommand = exec.Command

// ListLocal walks the given directory and returns MD5 hashes for all files.
func ListLocal(dir string) ([]FileHash, error) {
	var results []FileHash
	dir = strings.TrimRight(dir, string(os.PathSeparator))
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		h := md5.New()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		hash := hex.EncodeToString(h.Sum(nil))
		results = append(results, FileHash{Path: rel, Hash: hash})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// ListRemote connects to a remote host via SSH, runs md5sum, and parses the results.
// The addr should be in the form "host:/absolute/path".
func ListRemote(addr string) ([]FileHash, error) {
	parts := strings.SplitN(addr, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid remote address %s", addr)
	}
	host, base := parts[0], parts[1]
	base = strings.TrimRight(base, "/")
	cmd := ExecCommand("ssh", host, fmt.Sprintf("cd %s && find . -type f -exec md5sum {} +", base))
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ssh error: %w", err)
	}

	var results []FileHash
	s := bufio.NewScanner(&out)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 2 {
			continue
		}
		h := fields[0]
		file := fields[1]
		rel := strings.TrimPrefix(file, "./")
		results = append(results, FileHash{Path: rel, Hash: h})
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// ComputeDiff compares two file-hash lists and returns a sorted slice of Diff.
func ComputeDiff(srcList, dstList []FileHash) []Diff {
	srcMap := make(map[string]string, len(srcList))
	dstMap := make(map[string]string, len(dstList))
	for _, fh := range srcList {
		srcMap[fh.Path] = fh.Hash
	}
	for _, fh := range dstList {
		dstMap[fh.Path] = fh.Hash
	}

	// Collect unique paths
	pathSet := make(map[string]struct{})
	for path := range srcMap {
		pathSet[path] = struct{}{}
	}
	for path := range dstMap {
		pathSet[path] = struct{}{}
	}

	// Sort paths
	paths := make([]string, 0, len(pathSet))
	for path := range pathSet {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	// Build diff slice
	var diffs []Diff
	for _, path := range paths {
		sHash, sOk := srcMap[path]
		dHash, dOk := dstMap[path]
		var status DiffStatus
		switch {
		case !sOk:
			status = MissingSource
		case !dOk:
			status = MissingDestination
		case sHash != dHash:
			status = Mismatch
		default:
			status = Match
		}
		diffs = append(diffs, Diff{Path: path, SrcHash: sHash, DstHash: dHash, Status: status})
	}
	return diffs
}
