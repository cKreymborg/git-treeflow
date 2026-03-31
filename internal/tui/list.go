package tui

import (
	"fmt"
	"strings"

	"git-treeflow/internal/git"

	tea "github.com/charmbracelet/bubbletea"
)

type listModel struct {
	worktrees []git.Worktree
	cursor    int
	repoRoot  string
	err       error
}

func newListModel(repoRoot string) listModel {
	return listModel{repoRoot: repoRoot}
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

func (m listModel) selectedWorktree() *git.Worktree {
	if len(m.worktrees) == 0 {
		return nil
	}
	return &m.worktrees[m.cursor]
}

func (m listModel) View(width int) string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}
	if len(m.worktrees) == 0 {
		return dimStyle.Render("No worktrees found.")
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Worktrees"))
	b.WriteString("\n")

	for i, wt := range m.worktrees {
		cursor := "  "
		if i == m.cursor {
			cursor = "▸ "
		}

		name := wt.Branch
		if name == "" {
			name = "(detached)"
		}

		label := fmt.Sprintf("%s%s", cursor, name)

		switch {
		case wt.IsCurrent:
			b.WriteString(currentStyle.Render(label + " ●"))
		case i == m.cursor:
			b.WriteString(selectedStyle.Render(label))
		default:
			b.WriteString(dimStyle.Render(label))
		}
		b.WriteString("\n")
	}

	return b.String()
}
