package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Worktree struct {
	Path      string
	Branch    string
	IsMain    bool
	IsCurrent bool
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), string(exitErr.Stderr))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func RepoRoot(dir string) (string, error) {
	return runGit(dir, "rev-parse", "--show-toplevel")
}

func RepoName(dir string) (string, error) {
	root, err := RepoRoot(dir)
	if err != nil {
		return "", err
	}
	return filepath.Base(root), nil
}

func ListWorktrees(dir string) ([]Worktree, error) {
	out, err := runGit(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	var worktrees []Worktree
	var current Worktree
	isFirst := true

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "worktree "):
			if !isFirst {
				worktrees = append(worktrees, current)
			}
			current = Worktree{Path: strings.TrimPrefix(line, "worktree ")}
			isFirst = false
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "":
			// end of entry
		}
	}
	if !isFirst {
		worktrees = append(worktrees, current)
	}

	if len(worktrees) > 0 {
		worktrees[0].IsMain = true
	}

	return worktrees, nil
}

func MarkCurrent(worktrees []Worktree, dir string) {
	resolved, _ := filepath.EvalSymlinks(dir)
	for i := range worktrees {
		wtResolved, _ := filepath.EvalSymlinks(worktrees[i].Path)
		if wtResolved == resolved {
			worktrees[i].IsCurrent = true
		}
	}
}
