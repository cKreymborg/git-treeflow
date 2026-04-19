package tui

import (
	"fmt"
	"os"

	"github.com/cKreymborg/git-treeflow/internal/config"
	gitpkg "github.com/cKreymborg/git-treeflow/internal/git"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewState int

const (
	viewList viewState = iota
	viewCreate
	viewFastCreate
	viewDelete
	viewSettings
	viewPrune
)

// repoNamePlacement controls where the repo name appears on the home screen.
// Flip repoNameDisplayPlacement to try the alternative layout.
type repoNamePlacement int

const (
	repoNameInlineTitle repoNamePlacement = iota
	repoNameDedicatedLine
)

const repoNameDisplayPlacement = repoNameInlineTitle

type AppModel struct {
	view       viewState
	list       listModel
	create     createModel
	fastCreate fastCreateModel
	del        deleteModel
	settings   settingsModel
	prune      pruneModel
	showHelp   bool
	width      int
	height     int
	repoRoot   string
	repoName   string
	cwd        string
	cfg        config.Config
	version    string
	hasWrapper bool
	SwitchPath string
	err        error
}

func NewApp(repoRoot, cwd, version string, hasWrapper bool, cfg config.Config) AppModel {
	repoName, _ := gitpkg.RepoDisplayName(repoRoot)
	return AppModel{
		view:       viewList,
		list:       newListModel(repoRoot),
		repoRoot:   repoRoot,
		repoName:   repoName,
		cwd:        cwd,
		version:    version,
		hasWrapper: hasWrapper,
		cfg:        cfg,
	}
}

func (m AppModel) Init() tea.Cmd {
	return loadWorktrees(m.repoRoot, m.cwd)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case worktreesLoadedMsg:
		m.list, _ = m.list.Update(msg)
		return m, nil

	case createDoneMsg:
		m.view = viewList
		if msg.err != nil {
			m.err = msg.err
		}
		return m, loadWorktrees(m.repoRoot, m.cwd)

	case deleteDoneMsg:
		m.view = viewList
		if msg.err != nil {
			m.err = msg.err
		}
		return m, loadWorktrees(m.repoRoot, m.cwd)

	case pruneDoneMsg:
		m.view = viewList
		return m, loadWorktrees(m.repoRoot, m.cwd)

	case stashDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		wt := m.list.selectedWorktree()
		if wt != nil {
			m.SwitchPath = wt.Path
		}
		return m, tea.Quit
	}

	var cmd tea.Cmd
	switch m.view {
	case viewCreate:
		m.create, cmd = m.create.Update(msg)
	case viewFastCreate:
		m.fastCreate, cmd = m.fastCreate.Update(msg)
	case viewDelete:
		m.del, cmd = m.del.Update(msg)
	case viewSettings:
		m.settings, cmd = m.settings.Update(msg)
	case viewPrune:
		m.prune, cmd = m.prune.Update(msg)
	}
	return m, cmd
}

type stashDoneMsg struct{ err error }

func (m AppModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "?" && m.view == viewList {
		m.showHelp = !m.showHelp
		return m, nil
	}

	if m.showHelp {
		m.showHelp = false
		return m, nil
	}

	if m.view != viewList {
		var cmd tea.Cmd
		switch m.view {
		case viewCreate:
			m.create, cmd = m.create.Update(msg)
		case viewFastCreate:
			m.fastCreate, cmd = m.fastCreate.Update(msg)
		case viewDelete:
			m.del, cmd = m.del.Update(msg)
		case viewSettings:
			if msg.String() == "esc" && !m.settings.editing && m.settings.saveMode == 0 {
				m.view = viewList
				return m, loadWorktrees(m.repoRoot, m.cwd)
			}
			m.settings, cmd = m.settings.Update(msg)
		case viewPrune:
			m.prune, cmd = m.prune.Update(msg)
		}
		return m, cmd
	}

	switch {
	case key.Matches(msg, listKeys.Quit):
		return m, tea.Quit
	case key.Matches(msg, listKeys.Up):
		m.list = m.list.moveUp()
	case key.Matches(msg, listKeys.Down):
		m.list = m.list.moveDown()
	case key.Matches(msg, listKeys.Top):
		m.list = m.list.moveToTop()
	case key.Matches(msg, listKeys.Bottom):
		m.list = m.list.moveToBottom()
	case key.Matches(msg, listKeys.Select):
		return m.handleSwitch()
	case key.Matches(msg, listKeys.Create):
		m.err = nil
		m.view = viewCreate
		m.create = newCreateModel(m.repoRoot, m.cfg)
		return m, m.create.Init()
	case key.Matches(msg, listKeys.FastCreate):
		m.err = nil
		m.view = viewFastCreate
		m.fastCreate = newFastCreateModel(m.repoRoot, m.cfg)
		return m, m.fastCreate.Init()
	case key.Matches(msg, listKeys.Delete):
		m.err = nil
		wt := m.list.selectedWorktree()
		if wt != nil && !wt.IsMain {
			m.view = viewDelete
			m.del = newDeleteModel(*wt, m.repoRoot)
		}
		return m, nil
	case key.Matches(msg, listKeys.Prune):
		m.err = nil
		m.view = viewPrune
		m.prune = newPruneModel(m.repoRoot)
		return m, m.prune.Init()
	case key.Matches(msg, listKeys.ToggleView):
		m.list = m.list.cycleDisplayMode()
	case key.Matches(msg, listKeys.Settings):
		m.err = nil
		m.view = viewSettings
		m.settings = newSettingsModel(m.cfg, m.repoRoot)
		return m, nil
	}

	return m, nil
}

func (m AppModel) handleSwitch() (tea.Model, tea.Cmd) {
	m.err = nil
	wt := m.list.selectedWorktree()
	if wt == nil {
		return m, nil
	}
	if wt.IsCurrent {
		return m, nil
	}

	dirty, err := gitpkg.HasUncommittedChanges(m.cwd)
	if err != nil {
		m.err = err
		return m, nil
	}
	if dirty {
		return m, func() tea.Msg {
			err := gitpkg.Stash(m.cwd, "git-treeflow: auto-stash before switch")
			return stashDoneMsg{err: err}
		}
	}

	m.SwitchPath = wt.Path
	return m, tea.Quit
}

func (m AppModel) View() string {
	var content string
	var title string
	var footer []footerKey

	if m.showHelp {
		content = m.helpContent()
		title = "Keybindings"
		footer = []footerKey{{"any key", "close"}}
	} else {
		switch m.view {
		case viewList:
			content = m.list.View(m.panelInnerWidth())
			title = m.listTitle()
			footer = []footerKey{
				{"enter", "switch"}, {"c", "create"}, {"f", "quick create"},
				{"d", "delete"}, {"?", "help"}, {"q", "quit"},
			}
		case viewCreate:
			content = m.create.View()
			title = "Create Worktree"
			footer = m.create.FooterHints()
		case viewFastCreate:
			content = m.fastCreate.View()
			title = "Quick Create"
			footer = m.fastCreate.FooterHints()
		case viewDelete:
			content = m.del.View(m.panelInnerWidth())
			title = "Delete Worktree"
			footer = m.del.FooterHints()
		case viewSettings:
			content = m.settings.View()
			title = "Settings"
			footer = m.settings.FooterHints()
		case viewPrune:
			content = m.prune.View()
			title = "Prune"
			footer = m.prune.FooterHints()
		}
	}

	if m.err != nil {
		content += "\n\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	pw := m.panelWidth()

	var sections []string
	if m.view == viewList && !m.showHelp {
		versionLine := dimStyle.Render("  v" + m.version)
		sections = append(sections, "", renderLogo(), versionLine, "")
		if repoLine := m.renderRepoNameLine(pw); repoLine != "" {
			sections = append(sections, repoLine, "")
		}
	} else {
		sections = append(sections, "")
	}
	sections = append(sections, renderPanel(title, content, pw))
	sections = append(sections, renderFooter(footer, pw))

	if !m.hasWrapper && m.view == viewList && !m.showHelp {
		hint := accentStyle.Render("  Tip: ") +
			dimStyle.Render("Add ") +
			normalStyle.Render("eval \"$(gtf --init zsh)\"") +
			dimStyle.Render(" to your shell config to enable worktree switching")
		sections = append(sections, "", hint)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m AppModel) helpContent() string {
	keys := []struct{ key, desc string }{
		{"j / ↓", "Move down"},
		{"k / ↑", "Move up"},
		{"g", "Jump to top"},
		{"G", "Jump to bottom"},
		{"enter", "Switch to worktree"},
		{"c", "Create worktree"},
		{"f", "Quick create (off main)"},
		{"d", "Delete worktree"},
		{"p", "Prune stale worktrees"},
		{"s", "Settings"},
		{"v", "Toggle view (name / branch / both)"},
		{"?", "Toggle help"},
		{"q / esc", "Quit / back"},
	}

	var lines []string
	for _, k := range keys {
		lines = append(lines,
			accentStyle.Render(fmt.Sprintf("  %-12s", k.key))+normalStyle.Render(k.desc))
	}

	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

// listTitle returns the home-panel title, optionally annotated with the repo name.
func (m AppModel) listTitle() string {
	const base = "Existing Worktrees"
	if m.repoName == "" || repoNameDisplayPlacement != repoNameInlineTitle {
		return base
	}
	// Panel title sits inside "╭─ <title> ──╮" — budget = panelWidth - 5 - 1
	// (borders + gaps + at least one trailing dash).
	budget := m.panelWidth() - 6
	separator := " — "
	prefix := base + separator
	remaining := budget - lipgloss.Width(prefix)
	if remaining < 2 {
		return base
	}
	name := truncateTail(m.repoName, remaining)
	return prefix + name
}

// renderRepoNameLine renders the repo name as a standalone line above the
// panel. Returns "" when the inline-title placement is active or no name is known.
func (m AppModel) renderRepoNameLine(panelWidth int) string {
	if m.repoName == "" || repoNameDisplayPlacement != repoNameDedicatedLine {
		return ""
	}
	budget := panelWidth - 4
	if budget < 2 {
		return ""
	}
	name := truncateTail(m.repoName, budget)
	return accentStyle.Render("  " + name)
}

func (m AppModel) panelWidth() int {
	w := m.width
	if w == 0 {
		w = 80
	}
	return w
}

func (m AppModel) panelInnerWidth() int {
	return m.panelWidth() - 6
}

func RunApp(repoRoot, cwd, version string, hasWrapper bool, cfg config.Config) (string, error) {
	app := NewApp(repoRoot, cwd, version, hasWrapper, cfg)
	p := tea.NewProgram(app, tea.WithOutput(os.Stderr), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}
	return finalModel.(AppModel).SwitchPath, nil
}
