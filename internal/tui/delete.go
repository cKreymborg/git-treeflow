package tui

import (
	"fmt"
	"strings"

	gitpkg "github.com/cKreymborg/git-treeflow/internal/git"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type deleteModel struct {
	worktree     gitpkg.Worktree
	deleteBranch bool
	repoRoot     string
	confirmed    bool
	spinner      spinner.Model
	err          error
}

type deleteDoneMsg struct {
	err error
}

func newDeleteModel(wt gitpkg.Worktree, repoRoot string) deleteModel {
	return deleteModel{
		worktree: wt,
		repoRoot: repoRoot,
		spinner:  newDefaultSpinner(),
	}
}

func (m deleteModel) Update(msg tea.Msg) (deleteModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.confirmed {
			return m, nil
		}
		switch msg.String() {
		case "y":
			m.confirmed = true
			return m, tea.Batch(m.doDelete(), m.spinner.Tick)
		case "tab", " ":
			m.deleteBranch = !m.deleteBranch
			return m, nil
		case "esc", "n", "q":
			return m, func() tea.Msg { return deleteDoneMsg{} }
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
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

func (m deleteModel) View(width int) string {
	var b strings.Builder

	if m.confirmed && m.err == nil {
		b.WriteString(m.spinner.View() + " " + dimStyle.Render("Deleting worktree…"))
		return b.String()
	}

	// "Path    " label is 8 chars; truncate path to fit remaining width
	pathMaxWidth := width - 8
	if pathMaxWidth < 20 {
		pathMaxWidth = 20
	}
	path := truncatePath(m.worktree.Path, pathMaxWidth)

	b.WriteString(dimStyle.Render("Branch") + "  " + errorStyle.Render(m.worktree.Branch) + "\n")
	b.WriteString(dimStyle.Render("Path") + "    " + dimStyle.Render(path) + "\n\n")

	if m.deleteBranch {
		b.WriteString(successStyle.Render("◆") + normalStyle.Render(" Also delete branch"))
	} else {
		b.WriteString(dimStyle.Render("◇ Also delete branch"))
	}

	if m.err != nil {
		b.WriteString("\n\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	return b.String()
}

func (m deleteModel) FooterHints() []footerKey {
	if m.confirmed {
		return nil
	}
	return []footerKey{{"y", "confirm"}, {"tab", "toggle branch"}, {"esc", "cancel"}}
}
