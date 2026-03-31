package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func copyFiles(srcDir, dstDir string, patterns []string) {
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(srcDir, pattern))
		if err != nil {
			continue
		}
		for _, src := range matches {
			data, err := os.ReadFile(src)
			if err != nil {
				continue
			}
			dst := filepath.Join(dstDir, filepath.Base(src))
			os.WriteFile(dst, data, 0644)
		}
	}
}

func runHooks(dir string, hooks []string) {
	for _, hook := range hooks {
		parts := strings.Fields(hook)
		if len(parts) == 0 {
			continue
		}
		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.Dir = dir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		cmd.Run()
	}
}
