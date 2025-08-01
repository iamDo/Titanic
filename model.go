package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"titanic_app/diff"

	bubbletea "github.com/charmbracelet/bubbletea"
	tea      "github.com/charmbracelet/bubbletea"
)

// Styles for diff rows and messages
var (
	matchStyle   = lipgloss.NewStyle().Background(lipgloss.Color("#D4EDDA")).Foreground(lipgloss.Color("#155724"))
	missingStyle = lipgloss.NewStyle().Background(lipgloss.Color("#F8D7DA")).Foreground(lipgloss.Color("#721C24"))
	syncingStyle = lipgloss.NewStyle().Background(lipgloss.Color("#D1ECF1")).Foreground(lipgloss.Color("#0C5460")).Bold(true)
	refreshStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#0C5460")).Bold(true)
)

// Message types for async updates
type diffsMsg [][]diff.Diff

type syncStartMsg struct {
	Index int
	Path  string
}

type syncDoneMsg struct {
	Index   int
	Path    string
	Err     error
	SrcHash string
}

// Model holds state: pairs, current index, computed diffs, loading and syncing flags
type Model struct {
	Pairs   []DirectoryPair
	Index   int
	Diffs   [][]diff.Diff
	Loading bool
	Syncing map[string]struct{}
}

// NewModel initializes Model and precomputes diffs
func NewModel(cfg Config) Model {
	m := Model{
		Pairs:   cfg.DirectoryPairs,
		Syncing: make(map[string]struct{}),
	}
	m.Diffs = computeAllDiffs(m.Pairs)
	return m
}

// computeAllDiffs runs ListLocal/Remote and ComputeDiff for each pair
func computeAllDiffs(pairs []DirectoryPair) [][]diff.Diff {
	var all [][]diff.Diff
	for _, p := range pairs {
		src, err1 := getFileHashes(p.Source)
		dst, err2 := diff.ListLocal(p.Destination)
		if err1 != nil || err2 != nil {
			all = append(all, nil)
		} else {
			all = append(all, diff.ComputeDiff(src, dst))
		}
	}
	return all
}

// computeDiffsCmd returns a Cmd to recompute all diffs asynchronously
func computeDiffsCmd(pairs []DirectoryPair) tea.Cmd {
	return func() tea.Msg {
		return diffsMsg(computeAllDiffs(pairs))
	}
}

// syncStartCmd returns a Cmd that indicates a file sync has started
func syncStartCmd(index int, path string) tea.Cmd {
	return func() tea.Msg { return syncStartMsg{Index: index, Path: path} }
}

// syncFileCmd returns a Cmd that performs rsync and returns a syncDoneMsg
func syncFileCmd(index int, path string, pr DirectoryPair) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		// build source path
		if strings.Contains(pr.Source, ":") {
			host := strings.SplitN(pr.Source, ":", 2)[0]
			hostPath := fmt.Sprintf("%s:%s/%s", host, strings.TrimRight(strings.SplitN(pr.Source, ":", 2)[1], "/"), path)
			cmd = exec.Command("rsync", "-avz", hostPath, filepath.Join(pr.Destination, path))
		} else {
			srcPath := filepath.Join(pr.Source, path)
			cmd = exec.Command("rsync", "-avz", srcPath, filepath.Join(pr.Destination, path))
		}
		err := cmd.Run()
		// recalc src hash
		src, _ := getFileHashes(pr.Source)
		var newHash string
		for _, fh := range src {
			if fh.Path == path {
				newHash = fh.Hash
				break
			}
		}
		return syncDoneMsg{Index: index, Path: path, Err: err, SrcHash: newHash}
	}
}

// Init does nothing initially (diffs already computed)
func (m Model) Init() tea.Cmd { return nil }

// Update handles incoming messages and key events
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case diffsMsg:
		m.Diffs = [][]diff.Diff(msg)
		m.Loading = false
		return m, nil

	case syncStartMsg:
		if msg.Index == m.Index {
			m.Syncing[msg.Path] = struct{}{}
		}
		return m, nil

	case syncDoneMsg:
		if msg.Index == m.Index {
			delete(m.Syncing, msg.Path)
			if msg.Err == nil {
				for i, d := range m.Diffs[msg.Index] {
					if d.Path == msg.Path {
						m.Diffs[msg.Index][i].DstHash = msg.SrcHash
						m.Diffs[msg.Index][i].Status = diff.Match
						break
					}
				}
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, bubbletea.Quit

		case "tab":
			if m.Index < len(m.Pairs)-1 {
				m.Index++
			} else {
				m.Index = 0
			}
			return m, nil

		case "r":
			m.Loading = true
			return m, computeDiffsCmd(m.Pairs)

		case "s":
			idx := m.Index
			pair := m.Pairs[idx]
			var cmds []tea.Cmd
			for _, d := range m.Diffs[idx] {
				if d.Status == diff.MissingDestination || d.Status == diff.Mismatch {
					cmds = append(cmds, syncStartCmd(idx, d.Path))
					cmds = append(cmds, syncFileCmd(idx, d.Path, pair))
				}
			}
			return m, tea.Batch(cmds...)
		}
	}
	return m, nil
}

// View renders the UI, including syncing indicators
func (m Model) View() string {
	if len(m.Pairs) == 0 {
		return "No directory pairs\n"
	}
	p := m.Pairs[m.Index]
	head := fmt.Sprintf("Pair %d/%d %s -> %s\n", m.Index+1, len(m.Pairs), p.Source, p.Destination)
	var out strings.Builder
	for _, d := range m.Diffs[m.Index] {
		// Determine status label
		var statusLabel string
		if _, syncing := m.Syncing[d.Path]; syncing {
			statusLabel = "Syncing"
		} else {
			switch d.Status {
			case diff.Match:
				statusLabel = "Match"
			case diff.MissingSource:
				statusLabel = "Missing source"
			case diff.MissingDestination:
				statusLabel = "Missing destination"
			case diff.Mismatch:
				statusLabel = "Mismatch"
			}
		}
		// Build and style the line
		line := fmt.Sprintf("%-20s %-45s %-33s %-33s", statusLabel, d.Path, d.SrcHash, d.DstHash)
		var styled string
		if _, syncing := m.Syncing[d.Path]; syncing {
			styled = syncingStyle.Render(line)
		} else if d.Status == diff.Match {
			styled = matchStyle.Render(line)
		} else {
			styled = missingStyle.Render(line)
		}
		out.WriteString(styled + "\n")
	}
	if m.Loading {
		out.WriteString(refreshStyle.Render("Refreshing...") + "\n")
	}
	return head + out.String() + "\nPress tab to switch, r to refresh, s to sync, q to quit"
}
