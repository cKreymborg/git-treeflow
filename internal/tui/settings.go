package tui

import (
	"fmt"
	"strings"

	"github.com/cKreymborg/git-treeflow/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type settingsModel struct {
	cfg      config.Config
	repoRoot string
	cursor   int
	editing  bool
	input    textinput.Model
	fields   []settingsField
	saveMode int
	err      error
	saved    bool
}

type settingsFieldKind int

const (
	fieldText settingsFieldKind = iota
	fieldBool
)

type settingsField struct {
	label string
	key   string
	value string
	kind  settingsFieldKind
}

func newSettingsModel(cfg config.Config, repoRoot string) settingsModel {
	ti := textinput.New()

	fields := []settingsField{
		{label: "Worktree Path", key: "worktree_path", value: cfg.WorktreePath, kind: fieldText},
		{label: "Copy Files", key: "copy_files", value: strings.Join(cfg.CopyFiles, ", "), kind: fieldText},
		{label: "Post-Create Hooks", key: "post_create_hooks", value: strings.Join(cfg.PostCreateHooks, ", "), kind: fieldText},
		{label: "Auto-fetch Remote Branches", key: config.KeyAutoFetchRemoteBranches, value: boolDisplay(cfg.AutoFetchRemoteBranches), kind: fieldBool},
	}

	return settingsModel{
		cfg:      cfg,
		repoRoot: repoRoot,
		input:    ti,
		fields:   fields,
	}
}

func boolDisplay(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func renderFieldValue(f settingsField, textStyle lipgloss.Style) string {
	if f.kind == fieldBool {
		if f.value == "on" {
			return successStyle.Render("[on]")
		}
		return dimStyle.Render("[off]")
	}
	return textStyle.Render(f.value)
}

func (m settingsModel) Update(msg tea.Msg) (settingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.saveMode == 1 {
			return m.handleSaveChoice(msg)
		}
		if m.editing {
			return m.handleEditing(msg)
		}
		return m.handleNavigation(msg)
	}

	if m.editing {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m settingsModel) handleNavigation(msg tea.KeyMsg) (settingsModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.fields)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		f := m.fields[m.cursor]
		if f.kind == fieldBool {
			m.fields[m.cursor].value = boolDisplay(f.value != "on")
			m.applyFields()
			m.saved = false
			return m, nil
		}
		m.editing = true
		m.input.SetValue(f.value)
		m.input.Focus()
		return m, textinput.Blink
	case "w":
		m.saveMode = 1
	}
	return m, nil
}

func (m settingsModel) handleEditing(msg tea.KeyMsg) (settingsModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.fields[m.cursor].value = m.input.Value()
		m.editing = false
		m.input.Blur()
		m.applyFields()
		return m, nil
	case "esc":
		m.editing = false
		m.input.Blur()
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m settingsModel) handleSaveChoice(msg tea.KeyMsg) (settingsModel, tea.Cmd) {
	switch msg.String() {
	case "g":
		err := config.Save(m.cfg, config.GlobalConfigPath())
		m.err = err
		m.saveMode = 0
		if err == nil {
			m.saved = true
		}
	case "r":
		err := config.Save(m.cfg, config.RepoConfigPath(m.repoRoot))
		m.err = err
		m.saveMode = 0
		if err == nil {
			m.saved = true
		}
	case "esc":
		m.saveMode = 0
	}
	return m, nil
}

func (m *settingsModel) applyFields() {
	for _, f := range m.fields {
		switch f.key {
		case "worktree_path":
			m.cfg.WorktreePath = f.value
		case "copy_files":
			m.cfg.CopyFiles = splitAndTrim(f.value)
		case "post_create_hooks":
			m.cfg.PostCreateHooks = splitAndTrim(f.value)
		case config.KeyAutoFetchRemoteBranches:
			m.cfg.AutoFetchRemoteBranches = f.value == "on"
		}
	}
}

func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func (m settingsModel) View() string {
	var b strings.Builder

	for i, f := range m.fields {
		if i > 0 {
			b.WriteString("\n")
		}

		if m.editing && i == m.cursor {
			b.WriteString(accentStyle.Render(f.label) + "\n")
			b.WriteString("  " + m.input.View())
		} else if i == m.cursor {
			b.WriteString(selectedStyle.Render("▸ " + f.label) + "\n")
			b.WriteString("  " + renderFieldValue(f, normalStyle))
		} else {
			b.WriteString(dimStyle.Render("  " + f.label) + "\n")
			b.WriteString("  " + renderFieldValue(f, dimStyle))
		}
		b.WriteString("\n")
	}

	if m.saveMode == 1 {
		b.WriteString("\n" + normalStyle.Render("Save to: ") +
			accentStyle.Render("g") + normalStyle.Render(" global  ") +
			accentStyle.Render("r") + normalStyle.Render(" repo"))
	} else if m.saved {
		b.WriteString("\n" + successStyle.Render("Saved!"))
	}

	if m.err != nil {
		b.WriteString("\n\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	return b.String()
}

func (m settingsModel) FooterHints() []footerKey {
	if m.saveMode == 1 {
		return []footerKey{{"g", "global"}, {"r", "repo"}, {"esc", "cancel"}}
	}
	if m.editing {
		return []footerKey{{"enter", "save"}, {"esc", "cancel"}}
	}
	return []footerKey{{"enter", "edit"}, {"w", "save"}, {"esc", "back"}}
}
