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
	"github.com/sahilm/fuzzy"
)

type createStep int

const (
	stepName createStep = iota
	stepBranchMode
	stepBranchName
	stepBranchSelect
	stepConfirm
	stepCreating
)

type branchMode int

const (
	branchNew branchMode = iota
	branchLocal
	branchRemote
)

type createModel struct {
	step               createStep
	nameInput          textinput.Model
	branchInput        textinput.Model
	searchInput        textinput.Model
	branchMode         branchMode
	modeCursor         int
	branches           []string
	filtered           []string
	branchCursor       int
	repoRoot           string
	cfg                config.Config
	err                error
	worktreeName       string
	branchName         string
	baseBranch         string // resolved default branch, or "" if detection failed/not applicable
	baseLoading        bool   // true while DefaultBranch is being resolved
	basePickerOpen     bool   // true while the base-branch picker overlay is shown on stepConfirm
	baseBranchExplicit bool   // true once the user picks a base via the 'b' picker
	branchesGen        int    // increments on each loadBranches dispatch so stale results can be ignored
}

type createDoneMsg struct {
	err    error
	wtPath string
}

type branchesLoadedMsg struct {
	branches []string
	err      error
	gen      int
}

type defaultBranchLoadedMsg struct {
	branch string
	err    error
}

func newCreateModel(repoRoot string, cfg config.Config) createModel {
	ni := textinput.New()
	ni.Placeholder = "worktree-name"
	ni.Focus()

	bi := textinput.New()
	bi.Placeholder = "branch-name"

	si := textinput.New()
	si.Placeholder = "filter branches..."

	return createModel{
		step:        stepName,
		nameInput:   ni,
		branchInput: bi,
		searchInput: si,
		repoRoot:    repoRoot,
		cfg:         cfg,
	}
}

func (m createModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m createModel) Update(msg tea.Msg) (createModel, tea.Cmd) {
	switch msg := msg.(type) {
	case branchesLoadedMsg:
		if msg.gen != m.branchesGen {
			return m, nil
		}
		m.branches = msg.branches
		query := m.searchInput.Value()
		if query == "" {
			m.filtered = msg.branches
		} else {
			matches := fuzzy.Find(query, msg.branches)
			m.filtered = make([]string, len(matches))
			for i, match := range matches {
				m.filtered[i] = match.Str
			}
		}
		m.branchCursor = 0
		m.err = msg.err
		return m, nil

	case defaultBranchLoadedMsg:
		m.baseLoading = false
		if !m.baseBranchExplicit {
			m.baseBranch = msg.branch // "" if err != nil
		}
		return m, nil

	case createDoneMsg:
		return m, func() tea.Msg { return msg }

	case tea.KeyMsg:
		if m.basePickerOpen {
			return m.handleBasePickerKey(msg)
		}
		if msg.String() == "esc" {
			if m.step == stepName {
				return m, func() tea.Msg { return createDoneMsg{} }
			}
			m.err = nil
			m.step--
			if m.step == stepBranchMode {
				m.branchInput.Blur()
				m.searchInput.Blur()
				m.baseLoading = false
				m.baseBranchExplicit = false
			}
			if m.step == stepName {
				m.nameInput.Focus()
			}
			return m, nil
		}
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	switch m.step {
	case stepName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case stepBranchName:
		m.branchInput, cmd = m.branchInput.Update(msg)
	case stepBranchSelect:
		m.searchInput, cmd = m.searchInput.Update(msg)
	}
	return m, cmd
}

func (m createModel) handleKey(msg tea.KeyMsg) (createModel, tea.Cmd) {
	switch m.step {
	case stepName:
		switch msg.String() {
		case "enter":
			val := strings.TrimSpace(m.nameInput.Value())
			if val == "" {
				return m, nil
			}
			m.worktreeName = val
			m.step = stepBranchMode
			m.nameInput.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			if val := m.nameInput.Value(); strings.Contains(val, " ") {
				pos := m.nameInput.Position()
				m.nameInput.SetValue(strings.ReplaceAll(val, " ", "-"))
				m.nameInput.SetCursor(pos)
			}
			return m, cmd
		}

	case stepBranchMode:
		switch msg.String() {
		case "j", "down":
			if m.modeCursor < 2 {
				m.modeCursor++
			}
		case "k", "up":
			if m.modeCursor > 0 {
				m.modeCursor--
			}
		case "enter":
			m.branchMode = branchMode(m.modeCursor)
			switch m.branchMode {
			case branchNew:
				m.step = stepBranchName
				m.branchInput.Focus()
				m.baseLoading = true
				return m, tea.Batch(textinput.Blink, m.loadDefaultBranch())
			case branchLocal:
				m.step = stepBranchSelect
				m.searchInput.Focus()
				m.branchesGen++
				return m, tea.Batch(textinput.Blink, m.loadBranches(false))
			case branchRemote:
				m.step = stepBranchSelect
				m.searchInput.Focus()
				m.branchesGen++
				return m, tea.Batch(textinput.Blink, m.loadBranches(true))
			}
		}
		return m, nil

	case stepBranchName:
		switch msg.String() {
		case "enter":
			val := strings.TrimSpace(m.branchInput.Value())
			if val == "" {
				return m, nil
			}
			if err := gitpkg.ValidateBranchName(val); err != nil {
				m.err = err
				return m, nil
			}
			m.err = nil
			m.branchName = val
			m.step = stepConfirm
			m.branchInput.Blur()
			return m, nil
		default:
			m.err = nil
			var cmd tea.Cmd
			m.branchInput, cmd = m.branchInput.Update(msg)
			if val := m.branchInput.Value(); strings.Contains(val, " ") {
				pos := m.branchInput.Position()
				m.branchInput.SetValue(strings.ReplaceAll(val, " ", "-"))
				m.branchInput.SetCursor(pos)
			}
			return m, cmd
		}

	case stepBranchSelect:
		switch msg.String() {
		case "enter":
			if len(m.filtered) > 0 {
				m.branchName = m.filtered[m.branchCursor]
				m.step = stepConfirm
				m.searchInput.Blur()
			}
			return m, nil
		case "ctrl+j", "ctrl+n", "down":
			if m.branchCursor < len(m.filtered)-1 {
				m.branchCursor++
			}
			return m, nil
		case "ctrl+k", "ctrl+p", "up":
			if m.branchCursor > 0 {
				m.branchCursor--
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			query := m.searchInput.Value()
			if query == "" {
				m.filtered = m.branches
			} else {
				matches := fuzzy.Find(query, m.branches)
				m.filtered = make([]string, len(matches))
				for i, match := range matches {
					m.filtered[i] = match.Str
				}
			}
			m.branchCursor = 0
			return m, cmd
		}

	case stepConfirm:
		switch msg.String() {
		case "enter", "y":
			// Silent swallow: detection is sub-perceptual (<50ms) and
			// the (detecting…) indicator on the confirm view gives visual feedback.
			if m.baseLoading && m.branchMode == branchNew {
				return m, nil
			}
			m.step = stepCreating
			return m, m.doCreate()
		case "b":
			if m.branchMode != branchNew {
				return m, nil
			}
			m.basePickerOpen = true
			m.searchInput.SetValue("")
			m.branchCursor = 0
			m.branches = nil
			m.filtered = nil
			m.searchInput.Focus()
			m.branchesGen++
			return m, tea.Batch(textinput.Blink, m.loadBranches(false))
		}
		return m, nil
	}

	return m, nil
}

func (m createModel) handleBasePickerKey(msg tea.KeyMsg) (createModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.basePickerOpen = false
		m.searchInput.Blur()
		return m, nil
	case "enter":
		if len(m.filtered) > 0 {
			m.baseBranch = m.filtered[m.branchCursor]
			m.baseBranchExplicit = true
			m.basePickerOpen = false
			m.searchInput.Blur()
		}
		return m, nil
	case "ctrl+j", "ctrl+n", "down":
		if m.branchCursor < len(m.filtered)-1 {
			m.branchCursor++
		}
		return m, nil
	case "ctrl+k", "ctrl+p", "up":
		if m.branchCursor > 0 {
			m.branchCursor--
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		query := m.searchInput.Value()
		if query == "" {
			m.filtered = m.branches
		} else {
			matches := fuzzy.Find(query, m.branches)
			m.filtered = make([]string, len(matches))
			for i, match := range matches {
				m.filtered[i] = match.Str
			}
		}
		m.branchCursor = 0
		return m, cmd
	}
}

func (m createModel) loadBranches(remote bool) tea.Cmd {
	gen := m.branchesGen
	return func() tea.Msg {
		var branches []string
		var err error
		if remote {
			branches, err = gitpkg.ListRemoteBranches(m.repoRoot)
		} else {
			branches, err = gitpkg.ListLocalBranches(m.repoRoot)
		}
		return branchesLoadedMsg{branches: branches, err: err, gen: gen}
	}
}

func (m createModel) loadDefaultBranch() tea.Cmd {
	return func() tea.Msg {
		branch, err := gitpkg.DefaultBranch(m.repoRoot)
		return defaultBranchLoadedMsg{branch: branch, err: err}
	}
}

func (m createModel) doCreate() tea.Cmd {
	return func() tea.Msg {
		mainRoot, err := gitpkg.MainWorktreeRoot(m.repoRoot)
		if err != nil {
			return createDoneMsg{err: err}
		}

		repoName := filepath.Base(mainRoot)

		vars := map[string]string{
			"repoName":     repoName,
			"worktreeName": m.worktreeName,
			"branchName":   m.branchName,
			"date":         time.Now().Format("2006-01-02"),
		}

		relPath := config.ExpandPath(m.cfg.WorktreePath, vars)
		wtPath := filepath.Join(mainRoot, relPath)
		wtPath, _ = filepath.Abs(wtPath)

		isNew := m.branchMode == branchNew
		branch := m.branchName
		if m.branchMode == branchRemote {
			parts := strings.SplitN(branch, "/", 2)
			if len(parts) == 2 {
				branch = parts[1]
			}
		}

		base := ""
		if isNew {
			base = m.baseBranch
		}
		err = gitpkg.CreateWorktree(m.repoRoot, wtPath, branch, base, isNew)
		if err != nil {
			return createDoneMsg{err: err}
		}

		var warnings []string
		warnings = append(warnings, copyFiles(m.repoRoot, wtPath, m.cfg.CopyFiles)...)
		warnings = append(warnings, runHooks(wtPath, m.cfg.PostCreateHooks)...)

		var warnErr error
		if len(warnings) > 0 {
			warnErr = fmt.Errorf("warnings: %s", strings.Join(warnings, "; "))
		}
		return createDoneMsg{wtPath: wtPath, err: warnErr}
	}
}

func (m createModel) View() string {
	var b strings.Builder

	switch m.step {
	case stepName:
		b.WriteString(dimStyle.Render("Worktree name") + "\n\n")
		b.WriteString(m.nameInput.View())

	case stepBranchMode:
		b.WriteString(dimStyle.Render("Worktree") + "  " + accentStyle.Render(m.worktreeName) + "\n\n")
		b.WriteString(dimStyle.Render("Branch mode") + "\n\n")
		modes := []string{"Create new branch", "Checkout local branch", "Checkout remote branch"}
		for i, mode := range modes {
			if i == m.modeCursor {
				b.WriteString(selectedStyle.Render(" ◉ " + mode))
			} else {
				b.WriteString(dimStyle.Render(" ○ " + mode))
			}
			b.WriteString("\n")
		}

	case stepBranchName:
		b.WriteString(dimStyle.Render("Worktree") + "  " + accentStyle.Render(m.worktreeName) + "\n\n")
		b.WriteString(dimStyle.Render("New branch name") + "\n\n")
		b.WriteString(m.branchInput.View())

	case stepBranchSelect:
		b.WriteString(dimStyle.Render("Worktree") + "  " + accentStyle.Render(m.worktreeName) + "\n\n")
		b.WriteString(dimStyle.Render("Select branch") + "\n\n")
		b.WriteString(m.searchInput.View())
		b.WriteString("\n\n")
		if len(m.filtered) == 0 {
			b.WriteString(dimStyle.Render("No branches found"))
		} else {
			limit := 10
			if len(m.filtered) < limit {
				limit = len(m.filtered)
			}
			for i := 0; i < limit; i++ {
				if i == m.branchCursor {
					b.WriteString(selectedStyle.Render(" ▸ " + m.filtered[i]))
				} else {
					b.WriteString(dimStyle.Render("   " + m.filtered[i]))
				}
				b.WriteString("\n")
			}
			if len(m.filtered) > limit {
				b.WriteString(dimStyle.Render(fmt.Sprintf("   … and %d more", len(m.filtered)-limit)))
			}
		}

	case stepConfirm:
		if m.basePickerOpen {
			b.WriteString(dimStyle.Render("Worktree") + "  " + accentStyle.Render(m.worktreeName) + "\n\n")
			b.WriteString(dimStyle.Render("Select base branch") + "\n\n")
			b.WriteString(m.searchInput.View())
			b.WriteString("\n\n")
			if len(m.filtered) == 0 {
				b.WriteString(dimStyle.Render("No branches found"))
			} else {
				limit := 10
				if len(m.filtered) < limit {
					limit = len(m.filtered)
				}
				for i := 0; i < limit; i++ {
					if i == m.branchCursor {
						b.WriteString(selectedStyle.Render(" ▸ " + m.filtered[i]))
					} else {
						b.WriteString(dimStyle.Render("   " + m.filtered[i]))
					}
					b.WriteString("\n")
				}
				if len(m.filtered) > limit {
					b.WriteString(dimStyle.Render(fmt.Sprintf("   … and %d more", len(m.filtered)-limit)))
				}
			}
		} else {
			b.WriteString(dimStyle.Render("Worktree") + "   " + accentStyle.Render(m.worktreeName) + "\n")
			b.WriteString(dimStyle.Render("Branch") + "     " + accentStyle.Render(m.branchName) + "\n")
			if m.branchMode == branchNew {
				var baseStr string
				if m.baseLoading {
					baseStr = dimStyle.Render("(detecting…)")
				} else if m.baseBranch != "" {
					baseStr = accentStyle.Render(m.baseBranch)
				} else {
					baseStr = dimStyle.Render("(current HEAD)")
				}
				b.WriteString(dimStyle.Render("Base") + "       " + baseStr + "\n")
			}
			modeStr := "new"
			if m.branchMode == branchLocal {
				modeStr = "local"
			} else if m.branchMode == branchRemote {
				modeStr = "remote"
			}
			b.WriteString(dimStyle.Render("Mode") + "       " + dimStyle.Render(modeStr))
		}

	case stepCreating:
		b.WriteString(dimStyle.Render("Creating worktree…"))
	}

	if m.err != nil {
		b.WriteString("\n\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	return b.String()
}

func (m createModel) FooterHints() []footerKey {
	switch m.step {
	case stepName:
		return []footerKey{{"enter", "continue"}, {"esc", "cancel"}}
	case stepBranchMode:
		return []footerKey{{"j/k", "navigate"}, {"enter", "select"}, {"esc", "back"}}
	case stepBranchName:
		return []footerKey{{"enter", "continue"}, {"esc", "back"}}
	case stepBranchSelect:
		return []footerKey{{"↑/↓", "navigate"}, {"enter", "select"}, {"esc", "back"}}
	case stepConfirm:
		if m.basePickerOpen {
			return []footerKey{{"↑/↓", "navigate"}, {"enter", "select"}, {"esc", "close"}}
		}
		if m.branchMode == branchNew {
			return []footerKey{{"enter", "confirm"}, {"b", "change base"}, {"esc", "back"}}
		}
		return []footerKey{{"enter", "confirm"}, {"esc", "back"}}
	default:
		return nil
	}
}
