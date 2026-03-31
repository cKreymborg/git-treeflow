package tui

import (
	"fmt"
	"strings"

	gitpkg "git-treeflow/internal/git"

	tea "github.com/charmbracelet/bubbletea"
)

type pruneModel struct {
	staleWorktrees []gitpkg.Worktree
	staleBranches  []string
	repoRoot       string
	loading        bool
	err            error
}

type pruneDoneMsg struct{}

type pruneDataLoadedMsg struct {
	staleWorktrees []gitpkg.Worktree
	staleBranches  []string
	err            error
}

func newPruneModel(repoRoot string) pruneModel {
	return pruneModel{
		repoRoot: repoRoot,
		loading:  true,
	}
}

func (m pruneModel) Init() tea.Cmd {
	return func() tea.Msg {
		staleWt, err := gitpkg.StaleWorktrees(m.repoRoot)
		if err != nil {
			return pruneDataLoadedMsg{err: err}
		}
		staleBr, err := gitpkg.StaleBranches(m.repoRoot)
		if err != nil {
			return pruneDataLoadedMsg{err: err}
		}
		return pruneDataLoadedMsg{
			staleWorktrees: staleWt,
			staleBranches:  staleBr,
		}
	}
}

func (m pruneModel) Update(msg tea.Msg) (pruneModel, tea.Cmd) {
	switch msg := msg.(type) {
	case pruneDataLoadedMsg:
		m.loading = false
		m.staleWorktrees = msg.staleWorktrees
		m.staleBranches = msg.staleBranches
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "y":
			return m, m.doPrune()
		case "esc", "q", "n":
			return m, func() tea.Msg { return pruneDoneMsg{} }
		}
	}
	return m, nil
}

func (m pruneModel) doPrune() tea.Cmd {
	return func() tea.Msg {
		gitpkg.PruneWorktrees(m.repoRoot)
		for _, branch := range m.staleBranches {
			gitpkg.DeleteBranch(m.repoRoot, branch, true)
		}
		return pruneDoneMsg{}
	}
}

func (m pruneModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Prune"))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString("Scanning for stale worktrees and branches...")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n" + dimStyle.Render("esc to go back"))
		return b.String()
	}

	hasStale := len(m.staleWorktrees) > 0 || len(m.staleBranches) > 0
	if !hasStale {
		b.WriteString(successStyle.Render("Nothing to prune — all clean!"))
		b.WriteString("\n\n" + dimStyle.Render("esc to go back"))
		return b.String()
	}

	if len(m.staleWorktrees) > 0 {
		b.WriteString("Stale worktrees (directory missing):\n")
		for _, wt := range m.staleWorktrees {
			b.WriteString(fmt.Sprintf("  • %s (%s)\n", errorStyle.Render(wt.Branch), dimStyle.Render(wt.Path)))
		}
		b.WriteString("\n")
	}

	if len(m.staleBranches) > 0 {
		b.WriteString("Branches with deleted remote:\n")
		for _, branch := range m.staleBranches {
			b.WriteString(fmt.Sprintf("  • %s\n", errorStyle.Render(branch)))
		}
		b.WriteString("\n")
	}

	b.WriteString(dimStyle.Render("y prune all • esc cancel"))
	return b.String()
}
