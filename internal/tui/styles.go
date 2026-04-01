package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() {
	// The TUI renders to stderr (via bubbletea), so point lipgloss's
	// default renderer at stderr with TrueColor so hex colors work.
	lipgloss.DefaultRenderer().SetOutput(
		termenv.NewOutput(os.Stderr, termenv.WithProfile(termenv.TrueColor)),
	)
}

// Purple neon color palette
var (
	colorPurple  = lipgloss.Color("#BD93F9")
	colorMagenta = lipgloss.Color("#FF79C6")
	colorCyan    = lipgloss.Color("#8BE9FD")
	colorGreen   = lipgloss.Color("#50FA7B")
	colorRed     = lipgloss.Color("#FF5555")
	colorDim     = lipgloss.Color("#6272A4")
	colorFg      = lipgloss.Color("#F8F8F2")
	colorBgHL    = lipgloss.Color("#44475A")
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(colorMagenta).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(colorMagenta).
			Bold(true)

	activeItemStyle = lipgloss.NewStyle().
			Foreground(colorFg).
			Background(colorBgHL).
			Bold(true)

	currentStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	normalStyle = lipgloss.NewStyle().
			Foreground(colorFg)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	accentStyle = lipgloss.NewStyle().
			Foreground(colorCyan)

	keyBadgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#282A36")).
			Background(colorDim).
			Bold(true).
			Padding(0, 1)

	keyDescStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorDim)
)

type footerKey struct {
	key  string
	desc string
}

// hexToRGB parses "#RRGGBB" into float64 components.
func hexToRGB(hex string) (float64, float64, float64) {
	hex = strings.TrimPrefix(hex, "#")
	r, _ := strconv.ParseInt(hex[0:2], 16, 64)
	g, _ := strconv.ParseInt(hex[2:4], 16, 64)
	b, _ := strconv.ParseInt(hex[4:6], 16, 64)
	return float64(r), float64(g), float64(b)
}

func lerpF(a, b, t float64) float64 { return a + (b-a)*t }

// gradientColor returns an interpolated hex color at position t (0..1)
// across the gradient stops: magenta → purple → cyan.
func gradientColor(t float64) string {
	type stop struct{ r, g, b float64 }
	stops := []stop{
		{255, 121, 198}, // #FF79C6 magenta
		{189, 147, 249}, // #BD93F9 purple
		{139, 233, 253}, // #8BE9FD cyan
	}

	if t <= 0 {
		s := stops[0]
		return fmt.Sprintf("#%02x%02x%02x", int(s.r), int(s.g), int(s.b))
	}
	if t >= 1 {
		s := stops[len(stops)-1]
		return fmt.Sprintf("#%02x%02x%02x", int(s.r), int(s.g), int(s.b))
	}

	// Map t to segment between stops
	segments := float64(len(stops) - 1)
	scaled := t * segments
	idx := int(scaled)
	if idx >= len(stops)-1 {
		idx = len(stops) - 2
	}
	frac := scaled - float64(idx)

	a, b := stops[idx], stops[idx+1]
	return fmt.Sprintf("#%02x%02x%02x",
		int(lerpF(a.r, b.r, frac)),
		int(lerpF(a.g, b.g, frac)),
		int(lerpF(a.b, b.b, frac)),
	)
}

func renderLogo() string {
	lines := []string{
		"▀█▀ █▀█ █▀▀ █▀▀ █▀▀ █   █▀█ █ █ █",
		" █  █▀▄ ██▀ ██▀ █▀  █   █ █ █▄█▄█",
		" ▀  ▀ ▀ ▀▀▀ ▀▀▀ ▀   ▀▀▀ ▀▀▀  ▀ ▀ ",
	}

	totalRows := len(lines)
	maxCols := 0
	for _, l := range lines {
		if len([]rune(l)) > maxCols {
			maxCols = len([]rune(l))
		}
	}

	var rendered []string
	for row, line := range lines {
		runes := []rune(line)
		var b strings.Builder
		b.WriteString("  ") // left padding
		for col, r := range runes {
			if r == ' ' {
				b.WriteRune(' ')
				continue
			}
			// Horizontal gradient with subtle vertical shift
			colT := float64(col) / float64(maxCols-1)
			rowT := float64(row) / float64(totalRows-1)
			t := colT*0.85 + rowT*0.15
			if t > 1 {
				t = 1
			}

			hex := gradientColor(t)
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(hex)).
				Bold(true).
				Render(string(r)))
		}
		rendered = append(rendered, b.String())
	}
	return strings.Join(rendered, "\n")
}

func renderPanel(title, content string, width int) string {
	if width < 20 {
		width = 80
	}

	bdr := lipgloss.NewStyle().Foreground(colorPurple)
	ttl := lipgloss.NewStyle().Foreground(colorMagenta).Bold(true)

	innerWidth := width - 6 // "│" + "  " + content + "  " + "│"

	// Top border with optional title
	var topBorder string
	if title != "" {
		titleStr := ttl.Render(title)
		titleVisWidth := lipgloss.Width(titleStr)
		dashes := width - 5 - titleVisWidth
		if dashes < 1 {
			dashes = 1
		}
		topBorder = bdr.Render("╭─ ") + titleStr + bdr.Render(" " + strings.Repeat("─", dashes) + "╮")
	} else {
		topBorder = bdr.Render("╭" + strings.Repeat("─", width-2) + "╮")
	}

	bottomBorder := bdr.Render("╰" + strings.Repeat("─", width-2) + "╯")

	content = strings.TrimRight(content, "\n")
	contentLines := strings.Split(content, "\n")
	var body strings.Builder

	emptyLine := bdr.Render("│") + strings.Repeat(" ", width-2) + bdr.Render("│")
	body.WriteString(emptyLine + "\n")

	for _, line := range contentLines {
		visWidth := lipgloss.Width(line)
		rightPad := innerWidth - visWidth
		if rightPad < 0 {
			rightPad = 0
		}
		body.WriteString(bdr.Render("│") + "  " + line + strings.Repeat(" ", rightPad) + "  " + bdr.Render("│") + "\n")
	}

	body.WriteString(emptyLine + "\n")

	return topBorder + "\n" + body.String() + bottomBorder
}

func renderFooter(keys []footerKey) string {
	if len(keys) == 0 {
		return ""
	}
	var parts []string
	for _, k := range keys {
		badge := keyBadgeStyle.Render(k.key)
		desc := keyDescStyle.Render(k.desc)
		parts = append(parts, badge+" "+desc)
	}
	return " " + strings.Join(parts, "  ")
}

func truncatePath(path string, maxWidth int) string {
	if len(path) <= maxWidth {
		return path
	}
	ellipsis := "…/"
	// Walk forward through path segments until we find a suffix that fits
	for i := 1; i < len(path); i++ {
		if path[i] == '/' {
			candidate := ellipsis + path[i+1:]
			if len(candidate) <= maxWidth {
				return candidate
			}
		}
	}
	// Last resort: hard truncate
	if maxWidth > len(ellipsis) {
		return ellipsis + path[len(path)-(maxWidth-len(ellipsis)):]
	}
	return path[:maxWidth]
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
