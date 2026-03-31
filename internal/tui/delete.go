package tui

import (
	"fmt"
	"strings"

	gitpkg "github.com/cKreymborg/git-treeflow/internal/git"

	tea "github.com/charmbracelet/bubbletea"
)

type deleteModel struct {
	worktree     gitpkg.Worktree
	deleteBranch bool
	repoRoot     string
	confirmed    bool
	err          error
}

type deleteDoneMsg struct {
	err error
}

func newDeleteModel(wt gitpkg.Worktree, repoRoot string) deleteModel {
	return deleteModel{
		worktree: wt,
		repoRoot: repoRoot,
	}
}

func (m deleteModel) Update(msg tea.Msg) (deleteModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y":
			m.confirmed = true
			return m, m.doDelete()
		case "tab", " ":
			m.deleteBranch = !m.deleteBranch
			return m, nil
		case "esc", "n", "q":
			return m, func() tea.Msg { return deleteDoneMsg{} }
		}
	case deleteDoneMsg:
		return m, func() tea.Msg { return msg }
	}
	return m, nil
}

func (m deleteModel) doDelete() tea.Cmd {
	return func() tea.Msg {
		// Try non-force first; fall back to force
		err := gitpkg.RemoveWorktree(m.repoRoot, m.worktree.Path, false)
		if err != nil {
			err = gitpkg.RemoveWorktree(m.repoRoot, m.worktree.Path, true)
			if err != nil {
				return deleteDoneMsg{err: err}
			}
		}
		if m.deleteBranch && m.worktree.Branch != "" {
			// Try non-force first for branch too
			err = gitpkg.DeleteBranch(m.repoRoot, m.worktree.Branch, false)
			if err != nil {
				err = gitpkg.DeleteBranch(m.repoRoot, m.worktree.Branch, true)
			}
		}
		return deleteDoneMsg{err: err}
	}
}

func (m deleteModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Delete Worktree"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Worktree: %s\n", errorStyle.Render(m.worktree.Branch)))
	b.WriteString(fmt.Sprintf("  Path:     %s\n", dimStyle.Render(m.worktree.Path)))
	b.WriteString("\n")

	toggle := "[ ]"
	if m.deleteBranch {
		toggle = "[x]"
	}
	b.WriteString(fmt.Sprintf("  %s Also delete branch\n", toggle))

	b.WriteString("\n" + dimStyle.Render("y confirm • tab toggle branch deletion • esc cancel"))

	if m.err != nil {
		b.WriteString("\n\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	return b.String()
}
