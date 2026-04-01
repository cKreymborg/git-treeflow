package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cKreymborg/git-treeflow/internal/git"

	tea "github.com/charmbracelet/bubbletea"
)

type displayMode int

const (
	displayBoth displayMode = iota
	displayName
	displayBranch
)

type listModel struct {
	worktrees   []git.Worktree
	cursor      int
	repoRoot    string
	displayMode displayMode
	err         error
}

func newListModel(repoRoot string) listModel {
	return listModel{repoRoot: repoRoot, displayMode: displayBoth}
}

type worktreesLoadedMsg struct {
	worktrees []git.Worktree
	err       error
}

func loadWorktrees(repoRoot, cwd string) tea.Cmd {
	return func() tea.Msg {
		trees, err := git.ListWorktrees(repoRoot)
		if err != nil {
			return worktreesLoadedMsg{err: err}
		}
		git.MarkCurrent(trees, cwd)
		return worktreesLoadedMsg{worktrees: trees}
	}
}

func (m listModel) Update(msg tea.Msg) (listModel, tea.Cmd) {
	switch msg := msg.(type) {
	case worktreesLoadedMsg:
		m.worktrees = msg.worktrees
		m.err = msg.err
	}
	return m, nil
}

func (m listModel) moveUp() listModel {
	if m.cursor > 0 {
		m.cursor--
	}
	return m
}

func (m listModel) moveDown() listModel {
	if m.cursor < len(m.worktrees)-1 {
		m.cursor++
	}
	return m
}

func (m listModel) moveToTop() listModel {
	m.cursor = 0
	return m
}

func (m listModel) moveToBottom() listModel {
	if len(m.worktrees) > 0 {
		m.cursor = len(m.worktrees) - 1
	}
	return m
}

func (m listModel) cycleDisplayMode() listModel {
	m.displayMode = (m.displayMode + 1) % 3
	return m
}

func (m listModel) selectedWorktree() *git.Worktree {
	if len(m.worktrees) == 0 {
		return nil
	}
	return &m.worktrees[m.cursor]
}

func worktreeName(wt git.Worktree) string {
	return filepath.Base(wt.Path)
}

func (m listModel) itemLabel(wt git.Worktree) string {
	name := worktreeName(wt)
	branch := wt.Branch
	if branch == "" {
		branch = "(detached)"
	}

	switch m.displayMode {
	case displayName:
		return name
	case displayBranch:
		return branch
	default: // displayBoth
		if name == branch {
			return name
		}
		return name + "  " + dimStyle.Render(branch)
	}
}

func (m listModel) displayModeLabel() string {
	switch m.displayMode {
	case displayName:
		return "name"
	case displayBranch:
		return "branch"
	default:
		return "both"
	}
}

func (m listModel) View(width int) string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}
	if len(m.worktrees) == 0 {
		return dimStyle.Render("No worktrees found.")
	}

	var b strings.Builder
	for i, wt := range m.worktrees {
		if i > 0 {
			b.WriteString("\n")
		}

		label := m.itemLabel(wt)
		isSelected := i == m.cursor

		switch {
		case isSelected:
			line := " ▸ " + label
			b.WriteString(activeItemStyle.Render(line))
			if wt.IsCurrent {
				b.WriteString(currentStyle.Render("  ●"))
			}
		case wt.IsCurrent:
			b.WriteString(currentStyle.Render("   " + label + "  ●"))
		default:
			b.WriteString(normalStyle.Render("   " + label))
		}
	}

	return b.String()
}
