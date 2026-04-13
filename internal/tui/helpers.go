package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
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
