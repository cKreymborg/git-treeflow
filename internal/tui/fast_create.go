package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cKreymborg/git-treeflow/internal/config"
	gitpkg "github.com/cKreymborg/git-treeflow/internal/git"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type fastCreateStep int

const (
	fastStepName fastCreateStep = iota
	fastStepCreating
)

type fastCreateModel struct {
	step        fastCreateStep
	nameInput   textinput.Model
	repoRoot    string
	cfg         config.Config
	baseBranch  string
	baseLoading bool
	err         error
}

func newFastCreateModel(repoRoot string, cfg config.Config) fastCreateModel {
	ni := textinput.New()
	ni.Placeholder = "worktree-name"
	ni.Focus()

	return fastCreateModel{
		step:        fastStepName,
		nameInput:   ni,
		repoRoot:    repoRoot,
		cfg:         cfg,
		baseLoading: true,
	}
}

func (m fastCreateModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.loadDefaultBranch())
}

func (m fastCreateModel) Update(msg tea.Msg) (fastCreateModel, tea.Cmd) {
	switch msg := msg.(type) {
	case defaultBranchLoadedMsg:
		m.baseLoading = false
		m.baseBranch = msg.branch // "" if err != nil; falls back to current HEAD
		return m, nil

	case createDoneMsg:
		return m, func() tea.Msg { return msg }

	case tea.KeyMsg:
		if msg.String() == "esc" {
			return m, func() tea.Msg { return createDoneMsg{} }
		}
		if m.step == fastStepName && msg.String() == "enter" {
			val := strings.TrimSpace(m.nameInput.Value())
			if val == "" {
				return m, nil
			}
			if err := gitpkg.ValidateBranchName(val); err != nil {
				m.err = err
				return m, nil
			}
			if m.baseLoading {
				return m, nil
			}
			m.err = nil
			m.step = fastStepCreating
			return m, m.doCreate(val)
		}
	}

	if m.step == fastStepName {
		var cmd tea.Cmd
		m.nameInput, cmd = m.nameInput.Update(msg)
		if val := m.nameInput.Value(); strings.Contains(val, " ") {
			pos := m.nameInput.Position()
			m.nameInput.SetValue(strings.ReplaceAll(val, " ", "-"))
			m.nameInput.SetCursor(pos)
		}
		m.err = nil
		return m, cmd
	}
	return m, nil
}

func (m fastCreateModel) loadDefaultBranch() tea.Cmd {
	return func() tea.Msg {
		branch, err := gitpkg.DefaultBranch(m.repoRoot)
		return defaultBranchLoadedMsg{branch: branch, err: err}
	}
}

func (m fastCreateModel) doCreate(name string) tea.Cmd {
	repoRoot := m.repoRoot
	cfg := m.cfg
	base := m.baseBranch
	return func() tea.Msg {
		mainRoot, err := gitpkg.MainWorktreeRoot(repoRoot)
		if err != nil {
			return createDoneMsg{err: err}
		}
		vars := map[string]string{
			"repoName":     filepath.Base(mainRoot),
			"worktreeName": name,
			"branchName":   name,
			"date":         time.Now().Format("2006-01-02"),
		}
		relPath := config.ExpandPath(cfg.WorktreePath, vars)
		wtPath := filepath.Join(mainRoot, relPath)
		wtPath, _ = filepath.Abs(wtPath)

		if err := gitpkg.CreateWorktree(repoRoot, wtPath, name, base, true); err != nil {
			return createDoneMsg{err: err}
		}

		var warnings []string
		warnings = append(warnings, copyFiles(repoRoot, wtPath, cfg.CopyFiles)...)
		warnings = append(warnings, runHooks(wtPath, cfg.PostCreateHooks)...)

		var warnErr error
		if len(warnings) > 0 {
			warnErr = fmt.Errorf("warnings: %s", strings.Join(warnings, "; "))
		}
		return createDoneMsg{wtPath: wtPath, err: warnErr}
	}
}

func (m fastCreateModel) View() string {
	var b strings.Builder
	switch m.step {
	case fastStepName:
		header := dimStyle.Render("Quick create")
		if m.baseLoading {
			header += "  " + dimStyle.Render("(detecting base…)")
		} else if m.baseBranch != "" {
			header += "  " + dimStyle.Render("off ") + accentStyle.Render(m.baseBranch)
		} else {
			header += "  " + dimStyle.Render("off current HEAD")
		}
		b.WriteString(header + "\n\n")
		b.WriteString(dimStyle.Render("Worktree / branch name") + "\n\n")
		b.WriteString(m.nameInput.View())
	case fastStepCreating:
		b.WriteString(dimStyle.Render("Creating worktree…"))
	}
	if m.err != nil {
		b.WriteString("\n\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}
	return b.String()
}

func (m fastCreateModel) FooterHints() []footerKey {
	if m.step == fastStepName {
		return []footerKey{{"enter", "create"}, {"esc", "cancel"}}
	}
	return nil
}
