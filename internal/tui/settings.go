package tui

import (
	"fmt"
	"strings"

	"github.com/cKreymborg/git-treeflow/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

type settingsField struct {
	label string
	key   string
	value string
}

func newSettingsModel(cfg config.Config, repoRoot string) settingsModel {
	ti := textinput.New()

	fields := []settingsField{
		{label: "Worktree Path", key: "worktree_path", value: cfg.WorktreePath},
		{label: "Copy Files", key: "copy_files", value: strings.Join(cfg.CopyFiles, ", ")},
		{label: "Post-Create Hooks", key: "post_create_hooks", value: strings.Join(cfg.PostCreateHooks, ", ")},
	}

	return settingsModel{
		cfg:      cfg,
		repoRoot: repoRoot,
		input:    ti,
		fields:   fields,
	}
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
		m.editing = true
		m.input.SetValue(m.fields[m.cursor].value)
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
	b.WriteString(titleStyle.Render("Settings"))
	b.WriteString("\n\n")

	for i, f := range m.fields {
		cursor := "  "
		if i == m.cursor {
			cursor = "▸ "
		}

		if m.editing && i == m.cursor {
			b.WriteString(fmt.Sprintf("%s%s:\n    %s\n", cursor, f.label, m.input.View()))
		} else {
			label := fmt.Sprintf("%s%s: %s", cursor, f.label, f.value)
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(label))
			} else {
				b.WriteString(dimStyle.Render(label))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	if m.saveMode == 1 {
		b.WriteString("Save to: " + selectedStyle.Render("g") + " global • " + selectedStyle.Render("r") + " repo • esc cancel")
	} else if m.saved {
		b.WriteString(successStyle.Render("Saved!") + "\n")
		b.WriteString(dimStyle.Render("enter edit • w save • esc back"))
	} else {
		b.WriteString(dimStyle.Render("enter edit • w save • esc back"))
	}

	if m.err != nil {
		b.WriteString("\n\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	return b.String()
}
