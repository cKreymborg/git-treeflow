package git

import (
	"fmt"
	"os"
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

// DefaultBranch returns the repository's integration branch, preferring
// origin/HEAD (if it resolves to a branch that exists locally), then local
// "main", then local "master". Returns an error if none are found.
func DefaultBranch(dir string) (string, error) {
	if out, err := runGit(dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if candidate, ok := strings.CutPrefix(out, "origin/"); ok && candidate != "" {
			if _, err := runGit(dir, "show-ref", "--verify", "--quiet", "refs/heads/"+candidate); err == nil {
				return candidate, nil
			}
		}
	}
	if _, err := runGit(dir, "show-ref", "--verify", "--quiet", "refs/heads/main"); err == nil {
		return "main", nil
	}
	if _, err := runGit(dir, "show-ref", "--verify", "--quiet", "refs/heads/master"); err == nil {
		return "master", nil
	}
	return "", fmt.Errorf("could not detect default branch: no origin/HEAD, and neither 'main' nor 'master' exists locally")
}

func MainWorktreeRoot(dir string) (string, error) {
	gitCommonDir, err := runGit(dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(gitCommonDir) {
		gitCommonDir = filepath.Join(dir, gitCommonDir)
	}
	return filepath.Clean(filepath.Dir(gitCommonDir)), nil
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

func ListLocalBranches(dir string) ([]string, error) {
	out, err := runGit(dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func ListRemoteBranches(dir string) ([]string, error) {
	out, err := runGit(dir, "for-each-ref", "--format=%(refname:short)", "refs/remotes/")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var branches []string
	for _, b := range strings.Split(out, "\n") {
		if strings.HasSuffix(b, "/HEAD") {
			continue
		}
		branches = append(branches, b)
	}
	return branches, nil
}

func DeleteBranch(dir, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := runGit(dir, "branch", flag, branch)
	return err
}

func ValidateBranchName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	if strings.HasPrefix(name, "-") || name == "@" {
		return fmt.Errorf("invalid branch name %q", name)
	}
	cmd := exec.Command("git", "check-ref-format", "refs/heads/"+name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("invalid branch name %q", name)
	}
	return nil
}

// CreateWorktree creates a worktree at path.
//
//   - If newBranch is true and base != "":
//     git worktree add -b <branch> <path> <base>
//   - If newBranch is true and base == "":
//     git worktree add -b <branch> <path>       (fallback to HEAD of dir)
//   - If newBranch is false:
//     git worktree add <path> <branch>          (base is ignored)
func CreateWorktree(dir, path, branch, base string, newBranch bool) error {
	if newBranch {
		if base != "" {
			_, err := runGit(dir, "worktree", "add", "-b", branch, path, base)
			return err
		}
		_, err := runGit(dir, "worktree", "add", "-b", branch, path)
		return err
	}
	_, err := runGit(dir, "worktree", "add", path, branch)
	return err
}

func RemoveWorktree(dir, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	_, err := runGit(dir, args...)
	return err
}

func HasUncommittedChanges(dir string) (bool, error) {
	out, err := runGit(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func Stash(dir, message string) error {
	args := []string{"stash", "push"}
	if message != "" {
		args = append(args, "-m", message)
	}
	_, err := runGit(dir, args...)
	return err
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

func PruneWorktrees(dir string) error {
	_, err := runGit(dir, "worktree", "prune")
	return err
}

func StaleWorktrees(dir string) ([]Worktree, error) {
	trees, err := ListWorktrees(dir)
	if err != nil {
		return nil, err
	}
	var stale []Worktree
	for _, wt := range trees {
		if wt.IsMain {
			continue
		}
		if _, err := os.Stat(wt.Path); os.IsNotExist(err) {
			stale = append(stale, wt)
		}
	}
	return stale, nil
}

func StaleBranches(dir string) ([]string, error) {
	out, err := runGit(dir, "branch", "-vv")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var stale []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, ": gone]") {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "* ")
			parts := strings.Fields(line)
			if len(parts) > 0 {
				stale = append(stale, parts[0])
			}
		}
	}
	return stale, nil
}
