package main

import (
	"fmt"
	"strings"

	"titanic_app/diff"

	bubbletea "github.com/charmbracelet/bubbletea"
	tea      "github.com/charmbracelet/bubbletea"
)

// Model holds application state including directory pairs and their diffs
type Model struct {
	Pairs []DirectoryPair
	Index int
	Diffs [][]diff.Diff
}

// NewModel constructs a Model and precomputes diffs for all pairs
func NewModel(cfg Config) Model {
	m := Model{
		Pairs: cfg.DirectoryPairs,
	}
	m.Diffs = computeAllDiffs(m.Pairs)
	m.Index = 0
	return m
}

// Init is a Bubbletea command initializer (no-op here)
func (m Model) Init() bubbletea.Cmd {
	return nil
}

// Update handles key events and refresh actions
func (m Model) Update(msg bubbletea.Msg) (bubbletea.Model, bubbletea.Cmd) {
	switch msg := msg.(type) {
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
			m.Diffs = computeAllDiffs(m.Pairs)
			return m, nil
		case "s":
			// sync not implemented
			return m, nil
		}
	}
	return m, nil
}

// View renders the current pair's diffs as a formatted string
func (m Model) View() string {
	if len(m.Pairs) == 0 {
		return "No directory pairs\n"
	}
	p := m.Pairs[m.Index]
	head := fmt.Sprintf("Pair %d/%d %s -> %s\n", m.Index+1, len(m.Pairs), p.Source, p.Destination)
	diffs := m.Diffs[m.Index]
	var out strings.Builder
	for _, d := range diffs {
		var status string
		switch d.Status {
		case diff.Match:
			status = "Match"
		case diff.MissingSource:
			status = "Missing source"
		case diff.MissingDestination:
			status = "Missing destination"
		case diff.Mismatch:
			status = "Mismatch"
		}
		out.WriteString(fmt.Sprintf("%-20s %-45s %-33s %-33s\n", status, d.Path, d.SrcHash, d.DstHash))
	}
	return head + out.String() + "\nPress tab to switch, r to refresh, s to sync, q to quit"
}

// computeAllDiffs precomputes diffs for each DirectoryPair
func computeAllDiffs(pairs []DirectoryPair) [][]diff.Diff {
	var all [][]diff.Diff
	for _, p := range pairs {
		var srcList, dstList []diff.FileHash
		var err error
		if strings.Contains(p.Source, ":") {
			srcList, err = diff.ListRemote(p.Source)
		} else {
			srcList, err = diff.ListLocal(p.Source)
		}
		if err != nil {
			all = append(all, nil)
			continue
		}
		dstList, err = diff.ListLocal(p.Destination)
		if err != nil {
			all = append(all, nil)
			continue
		}
		all = append(all, diff.ComputeDiff(srcList, dstList))
	}
	return all
}