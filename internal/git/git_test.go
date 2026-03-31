package git

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup cmd %v failed: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestRepoRoot(t *testing.T) {
	dir := setupTestRepo(t)
	root, err := RepoRoot(dir)
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	expected, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(root)
	if got != expected {
		t.Errorf("RepoRoot = %q, want %q", got, expected)
	}
}

func TestRepoName(t *testing.T) {
	dir := setupTestRepo(t)
	name, err := RepoName(dir)
	if err != nil {
		t.Fatalf("RepoName: %v", err)
	}
	expected := filepath.Base(dir)
	if name != expected {
		t.Errorf("RepoName = %q, want %q", name, expected)
	}
}

func TestListWorktrees(t *testing.T) {
	dir := setupTestRepo(t)
	trees, err := ListWorktrees(dir)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if len(trees) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(trees))
	}
	if trees[0].Branch == "" {
		t.Error("expected branch to be set")
	}
}

func TestListWorktrees_Multiple(t *testing.T) {
	dir := setupTestRepo(t)

	wtPath := filepath.Join(t.TempDir(), "feature-wt")
	cmd := exec.Command("git", "worktree", "add", "-b", "feature", wtPath)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add failed: %v\n%s", err, out)
	}

	trees, err := ListWorktrees(dir)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if len(trees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(trees))
	}

	var foundMain, foundFeature bool
	for _, wt := range trees {
		if wt.IsMain {
			foundMain = true
		}
		if wt.Branch == "feature" {
			foundFeature = true
		}
	}
	if !foundMain {
		t.Error("expected to find main worktree")
	}
	if !foundFeature {
		t.Error("expected to find feature worktree")
	}
}

func TestListLocalBranches(t *testing.T) {
	dir := setupTestRepo(t)
	for _, b := range []string{"feature-a", "feature-b"} {
		cmd := exec.Command("git", "branch", b)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("branch create failed: %v\n%s", err, out)
		}
	}
	branches, err := ListLocalBranches(dir)
	if err != nil {
		t.Fatalf("ListLocalBranches: %v", err)
	}
	if len(branches) < 3 {
		t.Errorf("expected at least 3 branches, got %d: %v", len(branches), branches)
	}
}

func TestListRemoteBranches(t *testing.T) {
	dir := setupTestRepo(t)
	branches, err := ListRemoteBranches(dir)
	if err != nil {
		t.Fatalf("ListRemoteBranches: %v", err)
	}
	if len(branches) != 0 {
		t.Errorf("expected 0 remote branches, got %d", len(branches))
	}
}
