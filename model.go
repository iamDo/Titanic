package main

import (
	"fmt"
	bubbletea "github.com/charmbracelet/bubbletea"
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	Pairs []DirectoryPair
	Index int
}

func NewModel(cfg Config) Model {
	return Model{Pairs: cfg.DirectoryPairs, Index: 0}
}

func (m Model) Init() bubbletea.Cmd {
	return nil
}

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
		case "r":
			return m, nil
		case "s":
			syncDifferences(m.Pairs[m.Index])
			return m, nil
		}
	}
	return m, nil
}

func (m Model) View() string {
	if len(m.Pairs) == 0 {
		return "No directory pairs\n"
	}
	p := m.Pairs[m.Index]
	head := fmt.Sprintf("Pair %d/%d %s -> %s\n", m.Index+1, len(m.Pairs), p.Source, p.Destination)
	body := highlightDifferences(p)
	return head + body + "\nPress tab to switch, r to refresh, s to sync, q to quit"
}
