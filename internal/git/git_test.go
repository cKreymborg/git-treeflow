package git

import (
	"os"
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

func TestCreateAndRemoveWorktree(t *testing.T) {
	dir := setupTestRepo(t)
	wtPath := filepath.Join(t.TempDir(), "new-wt")

	err := CreateWorktree(dir, wtPath, "new-branch", true)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	trees, _ := ListWorktrees(dir)
	found := false
	for _, wt := range trees {
		if wt.Branch == "new-branch" {
			found = true
		}
	}
	if !found {
		t.Error("worktree not found after creation")
	}

	err = RemoveWorktree(dir, wtPath, false)
	if err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	trees, _ = ListWorktrees(dir)
	for _, wt := range trees {
		if wt.Branch == "new-branch" {
			t.Error("worktree still exists after removal")
		}
	}
}

func TestCreateWorktreeExistingBranch(t *testing.T) {
	dir := setupTestRepo(t)

	cmd := exec.Command("git", "branch", "existing-branch")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("branch create failed: %v\n%s", err, out)
	}

	wtPath := filepath.Join(t.TempDir(), "existing-wt")
	err := CreateWorktree(dir, wtPath, "existing-branch", false)
	if err != nil {
		t.Fatalf("CreateWorktree existing: %v", err)
	}

	trees, _ := ListWorktrees(dir)
	found := false
	for _, wt := range trees {
		if wt.Branch == "existing-branch" {
			found = true
		}
	}
	if !found {
		t.Error("worktree for existing branch not found")
	}
}

func TestHasUncommittedChanges(t *testing.T) {
	dir := setupTestRepo(t)

	dirty, err := HasUncommittedChanges(dir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if dirty {
		t.Error("expected clean repo")
	}

	os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("dirty"), 0644)

	dirty, err = HasUncommittedChanges(dir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if !dirty {
		t.Error("expected dirty repo")
	}
}

func TestStash(t *testing.T) {
	dir := setupTestRepo(t)

	os.WriteFile(filepath.Join(dir, "stash-me.txt"), []byte("data"), 0644)
	exec.Command("git", "-C", dir, "add", "stash-me.txt").Run()

	err := Stash(dir, "test stash")
	if err != nil {
		t.Fatalf("Stash: %v", err)
	}

	dirty, _ := HasUncommittedChanges(dir)
	if dirty {
		t.Error("expected clean after stash")
	}
}
