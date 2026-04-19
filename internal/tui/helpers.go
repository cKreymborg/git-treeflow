package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cKreymborg/git-treeflow/internal/config"
	gitpkg "github.com/cKreymborg/git-treeflow/internal/git"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

func newDefaultSpinner() spinner.Model {
	return spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(selectedStyle),
	)
}

func copyFiles(srcDir, dstDir string, patterns []string) []string {
	var errs []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(srcDir, pattern))
		if err != nil {
			errs = append(errs, fmt.Sprintf("glob %q: %v", pattern, err))
			continue
		}
		for _, src := range matches {
			data, err := os.ReadFile(src)
			if err != nil {
				errs = append(errs, fmt.Sprintf("read %s: %v", filepath.Base(src), err))
				continue
			}
			dst := filepath.Join(dstDir, filepath.Base(src))
			if err := os.WriteFile(dst, data, 0644); err != nil {
				errs = append(errs, fmt.Sprintf("write %s: %v", filepath.Base(dst), err))
			}
		}
	}
	return errs
}

// truncateTail shortens s to fit within maxWidth visual columns, appending
// a "…" when truncation occurs. Returns s unchanged if it already fits.
func truncateTail(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth == 1 {
		return "…"
	}
	runes := []rune(s)
	for i := len(runes) - 1; i > 0; i-- {
		candidate := string(runes[:i]) + "…"
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
	}
	return "…"
}

// normalizeInputSpaces rewrites any spaces in ti's value to hyphens in place,
// preserving the cursor column. Both the Create and Quick Create flows use
// this to keep worktree/branch names shell-friendly while the user types.
func normalizeInputSpaces(ti *textinput.Model) {
	val := ti.Value()
	if !strings.Contains(val, " ") {
		return
	}
	pos := ti.Position()
	ti.SetValue(strings.ReplaceAll(val, " ", "-"))
	ti.SetCursor(pos)
}

// createAndFinalizeWorktree resolves the target path from cfg.WorktreePath,
// creates the worktree on the given base, and runs the configured copy/hook
// post-steps. branchLabel is exposed to path templates; branchCheckout is the
// ref actually passed to git (they differ for remote refs where the caller
// strips the "origin/" prefix before checkout).
func createAndFinalizeWorktree(repoRoot string, cfg config.Config, worktreeName, branchLabel, branchCheckout, base string, isNew bool) createDoneMsg {
	mainRoot, err := gitpkg.MainWorktreeRoot(repoRoot)
	if err != nil {
		return createDoneMsg{err: err}
	}

	vars := map[string]string{
		"repoName":     filepath.Base(mainRoot),
		"worktreeName": worktreeName,
		"branchName":   branchLabel,
		"date":         time.Now().Format("2006-01-02"),
	}
	relPath := config.ExpandPath(cfg.WorktreePath, vars)
	wtPath := filepath.Join(mainRoot, relPath)
	wtPath, _ = filepath.Abs(wtPath)

	if err := gitpkg.CreateWorktree(repoRoot, wtPath, branchCheckout, base, isNew); err != nil {
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

func runHooks(dir string, hooks []string) []string {
	var errs []string
	for _, hook := range hooks {
		if strings.TrimSpace(hook) == "" {
			continue
		}
		cmd := exec.Command("sh", "-c", hook)
		cmd.Dir = dir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			errs = append(errs, fmt.Sprintf("hook %q: %v", hook, err))
		}
	}
	return errs
}
