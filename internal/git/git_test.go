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

func TestFetchAllPrune_Success(t *testing.T) {
	dir := setupTestRepo(t)
	remote := setupBareRemote(t, "main")

	setupCmds := [][]string{
		{"git", "remote", "add", "origin", remote},
		{"git", "fetch", "origin"},
	}
	for _, args := range setupCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup cmd %v failed: %v\n%s", args, err, out)
		}
	}

	pusher := t.TempDir()
	pushCmds := [][]string{
		{"git", "clone", remote, "."},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "checkout", "-b", "feature-x"},
		{"git", "commit", "--allow-empty", "-m", "feature commit"},
		{"git", "push", "origin", "feature-x"},
	}
	for _, args := range pushCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = pusher
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("pusher cmd %v failed: %v\n%s", args, err, out)
		}
	}

	before, err := ListRemoteBranches(dir)
	if err != nil {
		t.Fatalf("ListRemoteBranches before: %v", err)
	}
	for _, b := range before {
		if b == "origin/feature-x" {
			t.Fatalf("expected origin/feature-x to be absent before fetch, got %v", before)
		}
	}

	if err := FetchAllPrune(dir); err != nil {
		t.Fatalf("FetchAllPrune: %v", err)
	}

	after, err := ListRemoteBranches(dir)
	if err != nil {
		t.Fatalf("ListRemoteBranches after: %v", err)
	}
	found := false
	for _, b := range after {
		if b == "origin/feature-x" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected origin/feature-x in remote branches after fetch, got %v", after)
	}
}

func TestFetchAllPrune_Prunes(t *testing.T) {
	dir := setupTestRepo(t)
	remote := setupBareRemote(t, "main")

	pusher := t.TempDir()
	pushCmds := [][]string{
		{"git", "clone", remote, "."},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "checkout", "-b", "to-be-deleted"},
		{"git", "commit", "--allow-empty", "-m", "doomed"},
		{"git", "push", "origin", "to-be-deleted"},
	}
	for _, args := range pushCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = pusher
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("pusher cmd %v failed: %v\n%s", args, err, out)
		}
	}

	setupCmds := [][]string{
		{"git", "remote", "add", "origin", remote},
		{"git", "fetch", "origin"},
	}
	for _, args := range setupCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup cmd %v failed: %v\n%s", args, err, out)
		}
	}

	before, _ := ListRemoteBranches(dir)
	foundBefore := false
	for _, b := range before {
		if b == "origin/to-be-deleted" {
			foundBefore = true
		}
	}
	if !foundBefore {
		t.Fatalf("expected origin/to-be-deleted before deletion, got %v", before)
	}

	delCmd := exec.Command("git", "push", "origin", "--delete", "to-be-deleted")
	delCmd.Dir = pusher
	if out, err := delCmd.CombinedOutput(); err != nil {
		t.Fatalf("delete branch on remote failed: %v\n%s", err, out)
	}

	if err := FetchAllPrune(dir); err != nil {
		t.Fatalf("FetchAllPrune: %v", err)
	}

	after, _ := ListRemoteBranches(dir)
	for _, b := range after {
		if b == "origin/to-be-deleted" {
			t.Errorf("expected origin/to-be-deleted to be pruned, got %v", after)
		}
	}
}

func TestFetchAllPrune_NoRemote(t *testing.T) {
	dir := setupTestRepo(t)
	if err := FetchAllPrune(dir); err != nil {
		t.Errorf("FetchAllPrune in repo with no remotes: %v", err)
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

type remoteBranch struct {
	name string
	date string // ISO 8601 e.g. "2099-01-01T00:00:00Z"
}

// setupRemoteWithBranches creates a bare remote with default branch "main",
// then pushes each given branch with its committer/author date controlled.
// Returns the bare remote path. The seed "main" branch's commit date is
// approximately "now" (whenever the test runs); use future dates in
// pushes if you want them sorted ahead of main.
func setupRemoteWithBranches(t *testing.T, branches []remoteBranch) string {
	t.Helper()
	remote := setupBareRemote(t, "main")

	pusher := t.TempDir()
	initCmds := [][]string{
		{"git", "clone", remote, "."},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		// Bare remote's HEAD may not resolve to a real branch after our
		// push-only seed, leaving the clone with no local "main". Force-
		// create it from origin/main so subsequent checkouts can branch
		// from it.
		{"git", "checkout", "-B", "main", "origin/main"},
	}
	for _, args := range initCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = pusher
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("pusher init %v failed: %v\n%s", args, err, out)
		}
	}
	for _, b := range branches {
		coCmd := exec.Command("git", "checkout", "-b", b.name, "main")
		coCmd.Dir = pusher
		if out, err := coCmd.CombinedOutput(); err != nil {
			t.Fatalf("pusher checkout %s failed: %v\n%s", b.name, err, out)
		}
		commitCmd := exec.Command("git", "commit", "--allow-empty", "-m", "commit on "+b.name)
		commitCmd.Dir = pusher
		commitCmd.Env = append(os.Environ(),
			"GIT_COMMITTER_DATE="+b.date,
			"GIT_AUTHOR_DATE="+b.date,
		)
		if out, err := commitCmd.CombinedOutput(); err != nil {
			t.Fatalf("pusher commit %s failed: %v\n%s", b.name, err, out)
		}
		pushCmd := exec.Command("git", "push", "origin", b.name)
		pushCmd.Dir = pusher
		if out, err := pushCmd.CombinedOutput(); err != nil {
			t.Fatalf("pusher push %s failed: %v\n%s", b.name, err, out)
		}
	}
	return remote
}

func addOriginAndFetch(t *testing.T, dir, remote string) {
	t.Helper()
	cmds := [][]string{
		{"git", "remote", "add", "origin", remote},
		{"git", "fetch", "origin"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup cmd %v failed: %v\n%s", args, err, out)
		}
	}
}

func TestListRemoteBranches_SortedByCommitterDate(t *testing.T) {
	dir := setupTestRepo(t)
	remote := setupRemoteWithBranches(t, []remoteBranch{
		{name: "branch-old", date: "2099-01-01T00:00:00Z"},
		{name: "branch-mid", date: "2099-02-01T00:00:00Z"},
		{name: "branch-new", date: "2099-04-01T00:00:00Z"},
	})
	addOriginAndFetch(t, dir, remote)

	branches, err := ListRemoteBranches(dir)
	if err != nil {
		t.Fatalf("ListRemoteBranches: %v", err)
	}

	// Local has only master and remote has main, so DefaultBranch returns
	// "master" (local fallback) and the pin step finds no */master ref —
	// the result is pure date-sorted, no pinning interference.
	indexOf := func(name string) int {
		for i, b := range branches {
			if b == name {
				return i
			}
		}
		return -1
	}
	iNew := indexOf("origin/branch-new")
	iMid := indexOf("origin/branch-mid")
	iOld := indexOf("origin/branch-old")
	if iNew < 0 || iMid < 0 || iOld < 0 {
		t.Fatalf("expected all 3 test branches in result, got %v", branches)
	}
	if !(iNew < iMid && iMid < iOld) {
		t.Errorf("expected order branch-new < branch-mid < branch-old by index, got new=%d mid=%d old=%d in %v",
			iNew, iMid, iOld, branches)
	}
}

func TestListRemoteBranches_PinsDefaultBranch(t *testing.T) {
	dir := setupTestRepo(t)
	remote := setupRemoteWithBranches(t, []remoteBranch{
		{name: "feature-recent", date: "2099-04-01T00:00:00Z"},
	})

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

	branches, err := ListRemoteBranches(dir)
	if err != nil {
		t.Fatalf("ListRemoteBranches: %v", err)
	}

	if len(branches) < 2 {
		t.Fatalf("expected at least 2 branches, got %v", branches)
	}
	// Without pinning, origin/feature-recent (2099-04) would be index 0
	// since it is newer than origin/main (committed at "now"). The pin
	// must override the date order.
	if branches[0] != "origin/main" {
		t.Errorf("expected origin/main pinned at index 0, got %q (full list %v)",
			branches[0], branches)
	}
}

func TestListRemoteBranches_FiltersHEAD(t *testing.T) {
	dir := setupTestRepo(t)
	remote := setupRemoteWithBranches(t, []remoteBranch{
		{name: "feature", date: "2099-04-01T00:00:00Z"},
	})

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

	branches, err := ListRemoteBranches(dir)
	if err != nil {
		t.Fatalf("ListRemoteBranches: %v", err)
	}
	for _, b := range branches {
		if b == "origin/HEAD" || b == "origin" {
			t.Errorf("expected origin/HEAD symbolic ref to be filtered out, got %q in %v", b, branches)
		}
	}
}

func TestListRemoteBranches_NoDefaultBranchSkipsPin(t *testing.T) {
	dir := setupTestRepo(t)
	remote := setupRemoteWithBranches(t, []remoteBranch{
		{name: "branch-old", date: "2099-01-01T00:00:00Z"},
		{name: "branch-new", date: "2099-04-01T00:00:00Z"},
	})
	addOriginAndFetch(t, dir, remote)

	// Make DefaultBranch fail: switch to a non-main/master branch and
	// delete master. No origin/HEAD was set above, so DefaultBranch has
	// nothing to fall back on.
	teardownCmds := [][]string{
		{"git", "checkout", "-b", "foo"},
		{"git", "branch", "-D", "master"},
	}
	for _, args := range teardownCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("teardown cmd %v failed: %v\n%s", args, err, out)
		}
	}

	if _, err := DefaultBranch(dir); err == nil {
		t.Fatalf("DefaultBranch unexpectedly succeeded; test setup is wrong")
	}

	branches, err := ListRemoteBranches(dir)
	if err != nil {
		t.Fatalf("ListRemoteBranches: %v", err)
	}
	if len(branches) == 0 {
		t.Fatalf("expected branches, got empty")
	}

	// origin/main was committed at "now" — older than the 2099-dated
	// test branches. Without pinning, origin/branch-new must be first.
	if branches[0] != "origin/branch-new" {
		t.Errorf("expected origin/branch-new at index 0 (most recent, no pin), got %q (full list %v)",
			branches[0], branches)
	}
}
