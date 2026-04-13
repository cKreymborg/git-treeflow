package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init", "-b", "master"},
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

func TestMainWorktreeRoot_FromMain(t *testing.T) {
	dir := setupTestRepo(t)
	root, err := MainWorktreeRoot(dir)
	if err != nil {
		t.Fatalf("MainWorktreeRoot: %v", err)
	}
	expected, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(root)
	if got != expected {
		t.Errorf("MainWorktreeRoot from main = %q, want %q", got, expected)
	}
}

func TestMainWorktreeRoot_FromWorktree(t *testing.T) {
	dir := setupTestRepo(t)
	wtPath := filepath.Join(t.TempDir(), "secondary-wt")

	cmd := exec.Command("git", "worktree", "add", "-b", "secondary", wtPath)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add failed: %v\n%s", err, out)
	}

	root, err := MainWorktreeRoot(wtPath)
	if err != nil {
		t.Fatalf("MainWorktreeRoot from worktree: %v", err)
	}
	expected, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(root)
	if got != expected {
		t.Errorf("MainWorktreeRoot from worktree = %q, want %q", got, expected)
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

	err := CreateWorktree(dir, wtPath, "new-branch", "", true)
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
	err := CreateWorktree(dir, wtPath, "existing-branch", "", false)
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

func TestCreateWorktree_WithBase(t *testing.T) {
	dir := setupTestRepo(t)

	masterSha, err := runGit(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse master: %v", err)
	}

	setupCmds := [][]string{
		{"git", "checkout", "-b", "feature"},
	}
	for _, args := range setupCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup cmd %v failed: %v\n%s", args, err, out)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature"), 0644); err != nil {
		t.Fatalf("write feature.txt: %v", err)
	}
	commitCmds := [][]string{
		{"git", "add", "feature.txt"},
		{"git", "commit", "-m", "feature commit"},
	}
	for _, args := range commitCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("commit cmd %v failed: %v\n%s", args, err, out)
		}
	}

	featureSha, err := runGit(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse feature: %v", err)
	}
	if featureSha == masterSha {
		t.Fatalf("expected feature and master shas to differ, both were %q", masterSha)
	}

	wtPath := filepath.Join(t.TempDir(), "new-child-wt")
	if err := CreateWorktree(dir, wtPath, "new-child", "master", true); err != nil {
		t.Fatalf("CreateWorktree with base: %v", err)
	}

	childSha, err := runGit(dir, "rev-parse", "new-child")
	if err != nil {
		t.Fatalf("rev-parse new-child: %v", err)
	}
	if childSha != masterSha {
		t.Errorf("new-child tip = %q, want %q (master), not %q (feature)", childSha, masterSha, featureSha)
	}
}

func TestCreateWorktree_EmptyBaseFallsBackToHead(t *testing.T) {
	dir := setupTestRepo(t)

	setupCmds := [][]string{
		{"git", "checkout", "-b", "feature"},
	}
	for _, args := range setupCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup cmd %v failed: %v\n%s", args, err, out)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature"), 0644); err != nil {
		t.Fatalf("write feature.txt: %v", err)
	}
	commitCmds := [][]string{
		{"git", "add", "feature.txt"},
		{"git", "commit", "-m", "feature commit"},
	}
	for _, args := range commitCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("commit cmd %v failed: %v\n%s", args, err, out)
		}
	}

	featureSha, err := runGit(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse feature: %v", err)
	}

	wtPath := filepath.Join(t.TempDir(), "child-of-head-wt")
	if err := CreateWorktree(dir, wtPath, "child-of-head", "", true); err != nil {
		t.Fatalf("CreateWorktree empty base: %v", err)
	}

	childSha, err := runGit(dir, "rev-parse", "child-of-head")
	if err != nil {
		t.Fatalf("rev-parse child-of-head: %v", err)
	}
	if childSha != featureSha {
		t.Errorf("child-of-head tip = %q, want %q (feature HEAD)", childSha, featureSha)
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

func TestPruneWorktrees(t *testing.T) {
	dir := setupTestRepo(t)

	wtPath := filepath.Join(t.TempDir(), "prunable-wt")
	exec.Command("git", "-C", dir, "worktree", "add", "-b", "prunable", wtPath).Run()
	os.RemoveAll(wtPath)

	err := PruneWorktrees(dir)
	if err != nil {
		t.Fatalf("PruneWorktrees: %v", err)
	}

	trees, _ := ListWorktrees(dir)
	for _, wt := range trees {
		if wt.Branch == "prunable" {
			t.Error("prunable worktree still in list after prune")
		}
	}
}

func TestStaleBranches(t *testing.T) {
	dir := setupTestRepo(t)
	stale, err := StaleBranches(dir)
	if err != nil {
		t.Fatalf("StaleBranches: %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("expected 0 stale branches, got %d", len(stale))
	}
}

func TestStaleWorktrees(t *testing.T) {
	dir := setupTestRepo(t)

	wtPath := filepath.Join(t.TempDir(), "stale-wt")
	exec.Command("git", "-C", dir, "worktree", "add", "-b", "stale-branch", wtPath).Run()
	os.RemoveAll(wtPath)

	stale, err := StaleWorktrees(dir)
	if err != nil {
		t.Fatalf("StaleWorktrees: %v", err)
	}
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale worktree, got %d", len(stale))
	}
	if stale[0].Branch != "stale-branch" {
		t.Errorf("expected stale branch 'stale-branch', got %q", stale[0].Branch)
	}
}

func setupBareRemote(t *testing.T, branch string) string {
	t.Helper()
	remoteDir := t.TempDir()

	seed := t.TempDir()
	seedCmds := [][]string{
		{"git", "init", "-b", branch},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range seedCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = seed
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("seed cmd %v failed: %v\n%s", args, err, out)
		}
	}

	cmd := exec.Command("git", "init", "--bare", remoteDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bare init failed: %v\n%s", err, out)
	}

	pushCmd := exec.Command("git", "push", remoteDir, branch)
	pushCmd.Dir = seed
	if out, err := pushCmd.CombinedOutput(); err != nil {
		t.Fatalf("push to bare failed: %v\n%s", err, out)
	}

	return remoteDir
}

func TestDefaultBranch_OriginHead(t *testing.T) {
	dir := setupTestRepo(t)
	remote := setupBareRemote(t, "main")

	setupCmds := [][]string{
		{"git", "remote", "add", "origin", remote},
		{"git", "fetch", "origin"},
		{"git", "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"},
		{"git", "branch", "main", "origin/main"},
	}
	for _, args := range setupCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup cmd %v failed: %v\n%s", args, err, out)
		}
	}

	got, err := DefaultBranch(dir)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	if got != "main" {
		t.Errorf("DefaultBranch = %q, want %q", got, "main")
	}
}

func TestDefaultBranch_LocalMainFallback(t *testing.T) {
	dir := setupTestRepo(t)

	setupCmds := [][]string{
		{"git", "branch", "main"},
		{"git", "branch", "feature"},
	}
	for _, args := range setupCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup cmd %v failed: %v\n%s", args, err, out)
		}
	}

	got, err := DefaultBranch(dir)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	if got != "main" {
		t.Errorf("DefaultBranch = %q, want %q", got, "main")
	}
}

func TestDefaultBranch_LocalMasterFallback(t *testing.T) {
	dir := setupTestRepo(t)

	got, err := DefaultBranch(dir)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	if got != "master" {
		t.Errorf("DefaultBranch = %q, want %q", got, "master")
	}
}

func TestDefaultBranch_NoMatch(t *testing.T) {
	dir := setupTestRepo(t)

	setupCmds := [][]string{
		{"git", "checkout", "-b", "foo"},
		{"git", "branch", "-D", "master"},
	}
	for _, args := range setupCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup cmd %v failed: %v\n%s", args, err, out)
		}
	}

	got, err := DefaultBranch(dir)
	if err == nil {
		t.Errorf("DefaultBranch = %q, want error", got)
	}
	if !strings.Contains(err.Error(), "could not detect default branch") {
		t.Fatalf("expected error message to mention 'could not detect default branch', got: %v", err)
	}
}

func TestDefaultBranch_OriginHeadWithoutLocal(t *testing.T) {
	dir := setupTestRepo(t)
	remote := setupBareRemote(t, "main")

	setupCmds := [][]string{
		{"git", "remote", "add", "origin", remote},
		{"git", "fetch", "origin"},
		{"git", "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"},
	}
	for _, args := range setupCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup cmd %v failed: %v\n%s", args, err, out)
		}
	}

	got, err := DefaultBranch(dir)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	if got != "master" {
		t.Errorf("DefaultBranch = %q, want %q", got, "master")
	}
}

func TestValidateBranchName(t *testing.T) {
	valid := []string{
		"feature",
		"feature/foo",
		"feature/foo-bar",
		"release-1.0",
		"user/name/topic",
	}
	for _, name := range valid {
		if err := ValidateBranchName(name); err != nil {
			t.Errorf("ValidateBranchName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{
		"",
		" ",
		"has space",
		"has~tilde",
		"has^caret",
		"has:colon",
		"has?question",
		"has*star",
		"has[bracket",
		"has\\backslash",
		"..dots",
		"double..dot",
		"-leadingdash",
		"--doubledash",
		"-",
		".leadingdot",
		"trailing.",
		"trailing.lock",
		"ends/",
		"//double",
		"@",
		"has@{sequence",
	}
	for _, name := range invalid {
		if err := ValidateBranchName(name); err == nil {
			t.Errorf("ValidateBranchName(%q) = nil, want error", name)
		}
	}
}
