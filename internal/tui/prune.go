package tui

import (
	"fmt"
	"strings"

	gitpkg "github.com/cKreymborg/git-treeflow/internal/git"

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

	if m.loading {
		b.WriteString(dimStyle.Render("Scanning for stale worktrees and branches…"))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		return b.String()
	}

	hasStale := len(m.staleWorktrees) > 0 || len(m.staleBranches) > 0
	if !hasStale {
		b.WriteString(successStyle.Render("Nothing to prune — all clean!"))
		return b.String()
	}

	if len(m.staleWorktrees) > 0 {
		b.WriteString(dimStyle.Render("Stale worktrees") + "\n\n")
		for _, wt := range m.staleWorktrees {
			b.WriteString("  " + errorStyle.Render(wt.Branch) + "  " + dimStyle.Render(wt.Path) + "\n")
		}
		if len(m.staleBranches) > 0 {
			b.WriteString("\n")
		}
	}

	if len(m.staleBranches) > 0 {
		b.WriteString(dimStyle.Render("Orphan branches") + "\n\n")
		for _, branch := range m.staleBranches {
			b.WriteString("  " + errorStyle.Render(branch) + "\n")
		}
	}

	return b.String()
}

func (m pruneModel) FooterHints() []footerKey {
	if m.loading {
		return nil
	}
	if m.err != nil || (len(m.staleWorktrees) == 0 && len(m.staleBranches) == 0) {
		return []footerKey{{"esc", "back"}}
	}
	return []footerKey{{"y", "prune all"}, {"esc", "cancel"}}
}
