# git-treeflow Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go TUI for managing git worktrees — list, switch (cd), create, delete, prune — with vim keybindings and shell integration.

**Architecture:** Single Go binary using Bubbletea for the TUI. Root App model routes between sub-views (list, create, delete, settings, prune, help). All TUI rendering goes to stderr; stdout is reserved for the worktree path on switch. Git operations are in a dedicated package that shells out to `git`. Config is two-layer TOML (global + per-repo) with merge logic.

**Tech Stack:** Go, Bubbletea/Bubbles/Lipgloss (TUI), BurntSushi/toml (config), sahilm/fuzzy (matching)

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/gtf/main.go`
- Create: `internal/git/git.go`
- Create: `internal/config/config.go`
- Create: `internal/shell/shell.go`
- Create: `internal/tui/app.go`

**Step 1: Initialize Go module and install dependencies**

```bash
cd /Users/christopherkreymborg/Developer/Code/git-treeflow
go mod init git-treeflow
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/BurntSushi/toml@latest
go get github.com/sahilm/fuzzy@latest
```

**Step 2: Create minimal main.go that compiles**

```go
// cmd/gtf/main.go
package main

import "fmt"

func main() {
	fmt.Println("git-treeflow")
}
```

**Step 3: Create empty package files with package declarations**

`internal/git/git.go`:
```go
package git
```

`internal/config/config.go`:
```go
package config
```

`internal/shell/shell.go`:
```go
package shell
```

`internal/tui/app.go`:
```go
package tui
```

**Step 4: Verify it builds**

```bash
go build ./cmd/gtf
```

Expected: builds with no errors, produces `gtf` binary.

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: scaffold project with Go module and package structure"
```

---

### Task 2: Git Package — Repo Info & Worktree Listing

**Files:**
- Create: `internal/git/git.go`
- Create: `internal/git/git_test.go`

**Step 1: Write the failing tests**

```go
// internal/git/git_test.go
package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temp git repo and returns its path + cleanup func
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
	// Resolve symlinks for macOS /private/tmp
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

	// Create a worktree
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

	// Check that one is main and one is the worktree
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
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/git/ -v
```

Expected: FAIL — functions not defined.

**Step 3: Implement git package core**

```go
// internal/git/git.go
package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree represents a git worktree.
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
			// refs/heads/main -> main
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "":
			// end of entry, will be added on next "worktree" line or at end
		}
	}
	if !isFirst {
		worktrees = append(worktrees, current)
	}

	// Mark the first worktree as main
	if len(worktrees) > 0 {
		worktrees[0].IsMain = true
	}

	return worktrees, nil
}

// MarkCurrent sets IsCurrent on the worktree matching the given directory.
func MarkCurrent(worktrees []Worktree, dir string) {
	resolved, _ := filepath.EvalSymlinks(dir)
	for i := range worktrees {
		wtResolved, _ := filepath.EvalSymlinks(worktrees[i].Path)
		if wtResolved == resolved {
			worktrees[i].IsCurrent = true
		}
	}
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/git/ -v
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add internal/git/
git commit -m "feat: add git package with repo info and worktree listing"
```

---

### Task 3: Git Package — Branch Operations

**Files:**
- Modify: `internal/git/git.go`
- Modify: `internal/git/git_test.go`

**Step 1: Write the failing tests**

Add to `internal/git/git_test.go`:

```go
func TestListLocalBranches(t *testing.T) {
	dir := setupTestRepo(t)

	// Create extra branches
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
	// No remote set up, should return empty without error
	branches, err := ListRemoteBranches(dir)
	if err != nil {
		t.Fatalf("ListRemoteBranches: %v", err)
	}
	if len(branches) != 0 {
		t.Errorf("expected 0 remote branches, got %d", len(branches))
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/git/ -v -run "Branch"
```

Expected: FAIL — functions not defined.

**Step 3: Implement branch operations**

Add to `internal/git/git.go`:

```go
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
		// Skip HEAD pointer
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
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/git/ -v
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add internal/git/
git commit -m "feat: add branch listing and deletion to git package"
```

---

### Task 4: Git Package — Worktree Create/Remove, Stash, Uncommitted Changes

**Files:**
- Modify: `internal/git/git.go`
- Modify: `internal/git/git_test.go`

**Step 1: Write the failing tests**

Add to `internal/git/git_test.go`:

```go
func TestCreateAndRemoveWorktree(t *testing.T) {
	dir := setupTestRepo(t)
	wtPath := filepath.Join(t.TempDir(), "new-wt")

	err := CreateWorktree(dir, wtPath, "new-branch", true)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	// Verify it exists
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

	// Remove it
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

	// Create a branch first
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

	// Make it dirty
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

	// Make dirty
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
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/git/ -v -run "Create|Remove|Uncommitted|Stash"
```

Expected: FAIL — functions not defined.

**Step 3: Implement the functions**

Add to `internal/git/git.go`:

```go
func CreateWorktree(dir, path, branch string, newBranch bool) error {
	if newBranch {
		_, err := runGit(dir, "worktree", "add", "-b", branch, path)
		return err
	}
	_, err := runGit(dir, "worktree", "add", path, branch)
	return err
}

func RemoveWorktree(dir, path string, force bool) error {
	args := []string{"worktree", "remove", path}
	if force {
		args = append(args, "--force")
	}
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
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/git/ -v
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add internal/git/
git commit -m "feat: add worktree create/remove, stash, and dirty detection"
```

---

### Task 5: Git Package — Prune & Stale Branch Detection

**Files:**
- Modify: `internal/git/git.go`
- Modify: `internal/git/git_test.go`

**Step 1: Write the failing tests**

Add to `internal/git/git_test.go`:

```go
func TestPruneWorktrees(t *testing.T) {
	dir := setupTestRepo(t)

	// Create a worktree then manually delete its directory
	wtPath := filepath.Join(t.TempDir(), "prunable-wt")
	exec.Command("git", "-C", dir, "worktree", "add", "-b", "prunable", wtPath).Run()
	os.RemoveAll(wtPath)

	err := PruneWorktrees(dir)
	if err != nil {
		t.Fatalf("PruneWorktrees: %v", err)
	}

	// After prune, the worktree should be gone from the list
	trees, _ := ListWorktrees(dir)
	for _, wt := range trees {
		if wt.Branch == "prunable" {
			t.Error("prunable worktree still in list after prune")
		}
	}
}

func TestStaleBranches(t *testing.T) {
	dir := setupTestRepo(t)
	// No remote configured, should return empty without error
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

	// Create a worktree then delete directory
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
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/git/ -v -run "Prune|Stale"
```

Expected: FAIL — functions not defined.

**Step 3: Implement prune and stale detection**

Add to `internal/git/git.go`:

```go
func PruneWorktrees(dir string) error {
	_, err := runGit(dir, "worktree", "prune")
	return err
}

// StaleWorktrees returns worktrees whose directories no longer exist on disk.
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

// StaleBranches returns local branches whose remote tracking branch is gone.
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
			// Extract branch name: "  branch-name abc1234 [origin/branch: gone] msg"
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
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/git/ -v
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add internal/git/
git commit -m "feat: add worktree pruning and stale branch detection"
```

---

### Task 6: Config Package

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write the failing tests**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.WorktreePath == "" {
		t.Error("expected default worktree_path")
	}
	if len(cfg.CopyFiles) == 0 {
		t.Error("expected default copy_files")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
worktree_path = "../{repoName}.wt/{worktreeName}"
copy_files = [".env"]
post_create_hooks = ["npm install"]
`
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if cfg.WorktreePath != "../{repoName}.wt/{worktreeName}" {
		t.Errorf("WorktreePath = %q", cfg.WorktreePath)
	}
	if len(cfg.CopyFiles) != 1 || cfg.CopyFiles[0] != ".env" {
		t.Errorf("CopyFiles = %v", cfg.CopyFiles)
	}
	if len(cfg.PostCreateHooks) != 1 || cfg.PostCreateHooks[0] != "npm install" {
		t.Errorf("PostCreateHooks = %v", cfg.PostCreateHooks)
	}
}

func TestLoadFromFile_NotFound(t *testing.T) {
	cfg, err := LoadFromFile("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("should not error on missing file: %v", err)
	}
	// Should return zero config
	if cfg.WorktreePath != "" {
		t.Error("expected empty config for missing file")
	}
}

func TestMerge(t *testing.T) {
	global := Config{
		WorktreePath:    "../{repoName}.worktrees/{worktreeName}",
		CopyFiles:       []string{".env"},
		PostCreateHooks: []string{"make setup"},
	}
	repo := Config{
		CopyFiles: []string{".env", ".env.local"},
	}

	merged := Merge(global, repo)

	// repo overrides copy_files
	if len(merged.CopyFiles) != 2 {
		t.Errorf("expected repo copy_files to win, got %v", merged.CopyFiles)
	}
	// global worktree_path is kept since repo didn't set it
	if merged.WorktreePath != "../{repoName}.worktrees/{worktreeName}" {
		t.Errorf("expected global worktree_path, got %q", merged.WorktreePath)
	}
	// global hooks kept since repo didn't set them
	if len(merged.PostCreateHooks) != 1 {
		t.Errorf("expected global hooks, got %v", merged.PostCreateHooks)
	}
}

func TestExpandPath(t *testing.T) {
	tmpl := "../{repoName}.worktrees/{worktreeName}"
	vars := map[string]string{
		"repoName":     "my-app",
		"worktreeName": "feature-x",
		"branchName":   "feat/x",
		"date":         "2026-03-31",
	}
	result := ExpandPath(tmpl, vars)
	expected := "../my-app.worktrees/feature-x"
	if result != expected {
		t.Errorf("ExpandPath = %q, want %q", result, expected)
	}
}

func TestSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := Config{
		WorktreePath:    "../{repoName}.wt/{worktreeName}",
		CopyFiles:       []string{".env"},
		PostCreateHooks: []string{"npm install"},
	}

	err := Save(cfg, path)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile after save: %v", err)
	}
	if loaded.WorktreePath != cfg.WorktreePath {
		t.Errorf("round-trip WorktreePath = %q", loaded.WorktreePath)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/ -v
```

Expected: FAIL — functions/types not defined.

**Step 3: Implement config package**

```go
// internal/config/config.go
package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	WorktreePath    string   `toml:"worktree_path"`
	CopyFiles       []string `toml:"copy_files"`
	PostCreateHooks []string `toml:"post_create_hooks"`
}

func DefaultConfig() Config {
	return Config{
		WorktreePath: "../{repoName}.worktrees/{worktreeName}",
		CopyFiles:    []string{".env*"},
	}
}

func LoadFromFile(path string) (Config, error) {
	var cfg Config
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}
	_, err := toml.DecodeFile(path, &cfg)
	return cfg, err
}

func GlobalConfigPath() string {
	cfgDir := os.Getenv("XDG_CONFIG_HOME")
	if cfgDir == "" {
		home, _ := os.UserHomeDir()
		cfgDir = filepath.Join(home, ".config")
	}
	return filepath.Join(cfgDir, "git-treeflow", "config.toml")
}

func RepoConfigPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".git-treeflow.toml")
}

// Load loads the merged config (global + per-repo, with defaults).
func Load(repoRoot string) (Config, error) {
	cfg := DefaultConfig()

	global, err := LoadFromFile(GlobalConfigPath())
	if err != nil {
		return cfg, err
	}
	cfg = Merge(cfg, global)

	repo, err := LoadFromFile(RepoConfigPath(repoRoot))
	if err != nil {
		return cfg, err
	}
	cfg = Merge(cfg, repo)

	return cfg, nil
}

// Merge merges override into base. Non-zero override fields win.
func Merge(base, override Config) Config {
	result := base
	if override.WorktreePath != "" {
		result.WorktreePath = override.WorktreePath
	}
	if override.CopyFiles != nil {
		result.CopyFiles = override.CopyFiles
	}
	if override.PostCreateHooks != nil {
		result.PostCreateHooks = override.PostCreateHooks
	}
	return result
}

func ExpandPath(tmpl string, vars map[string]string) string {
	result := tmpl
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{"+k+"}", v)
	}
	return result
}

func Save(cfg Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/ -v
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config package with TOML loading, merging, and path expansion"
```

---

### Task 7: Shell Init Scripts

**Files:**
- Create: `internal/shell/shell.go`
- Create: `internal/shell/shell_test.go`

**Step 1: Write the failing tests**

```go
// internal/shell/shell_test.go
package shell

import (
	"strings"
	"testing"
)

func TestInitScript_Zsh(t *testing.T) {
	script, err := InitScript("zsh")
	if err != nil {
		t.Fatalf("InitScript zsh: %v", err)
	}
	if !strings.Contains(script, "gtf()") {
		t.Error("expected shell function definition")
	}
	if !strings.Contains(script, "cd") {
		t.Error("expected cd command")
	}
}

func TestInitScript_Bash(t *testing.T) {
	script, err := InitScript("bash")
	if err != nil {
		t.Fatalf("InitScript bash: %v", err)
	}
	if !strings.Contains(script, "gtf()") {
		t.Error("expected shell function definition")
	}
}

func TestInitScript_Fish(t *testing.T) {
	script, err := InitScript("fish")
	if err != nil {
		t.Fatalf("InitScript fish: %v", err)
	}
	if !strings.Contains(script, "function gtf") {
		t.Error("expected fish function definition")
	}
}

func TestInitScript_Unknown(t *testing.T) {
	_, err := InitScript("powershell")
	if err == nil {
		t.Error("expected error for unknown shell")
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/shell/ -v
```

Expected: FAIL — function not defined.

**Step 3: Implement shell init scripts**

```go
// internal/shell/shell.go
package shell

import "fmt"

const bashZshInit = `gtf() {
    local result
    result=$(command gtf "$@")
    local exit_code=$?
    if [ $exit_code -eq 0 ] && [ -n "$result" ] && [ -d "$result" ]; then
        cd "$result" || return
    elif [ -n "$result" ]; then
        echo "$result"
    fi
    return $exit_code
}`

const fishInit = `function gtf
    set -l result (command gtf $argv)
    set -l exit_code $status
    if test $exit_code -eq 0; and test -n "$result"; and test -d "$result"
        cd $result
    else if test -n "$result"
        echo $result
    end
    return $exit_code
end`

func InitScript(shell string) (string, error) {
	switch shell {
	case "zsh", "bash":
		return bashZshInit, nil
	case "fish":
		return fishInit, nil
	default:
		return "", fmt.Errorf("unsupported shell: %s (supported: zsh, bash, fish)", shell)
	}
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/shell/ -v
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add internal/shell/
git commit -m "feat: add shell init script generation for zsh, bash, and fish"
```

---

### Task 8: TUI — App Skeleton & Worktree List View

**Files:**
- Create: `internal/tui/app.go`
- Create: `internal/tui/list.go`
- Create: `internal/tui/styles.go`
- Create: `internal/tui/keys.go`

**Step 1: Create shared styles**

```go
// internal/tui/styles.go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	currentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("114")).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("114")).
			Bold(true)
)
```

**Step 2: Create key definitions**

```go
// internal/tui/keys.go
package tui

import "github.com/charmbracelet/bubbles/key"

type listKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Top    key.Binding
	Bottom key.Binding
	Select key.Binding
	Create key.Binding
	Delete key.Binding
	Prune  key.Binding
	Settings key.Binding
	Help   key.Binding
	Quit   key.Binding
}

var listKeys = listKeyMap{
	Up:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
	Down:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
	Top:    key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
	Bottom: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
	Select: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "switch to worktree")),
	Create: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "create worktree")),
	Delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete worktree")),
	Prune:  key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prune stale")),
	Settings: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "settings")),
	Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:   key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "quit")),
}
```

**Step 3: Create the list view model**

```go
// internal/tui/list.go
package tui

import (
	"fmt"
	"strings"

	"git-treeflow/internal/git"

	tea "github.com/charmbracelet/bubbletea"
)

type listModel struct {
	worktrees []git.Worktree
	cursor    int
	repoRoot  string
	err       error
}

func newListModel(repoRoot string) listModel {
	return listModel{repoRoot: repoRoot}
}

type worktreesLoadedMsg struct {
	worktrees []git.Worktree
	err       error
}

func loadWorktrees(repoRoot, cwd string) tea.Cmd {
	return func() tea.Msg {
		trees, err := git.ListWorktrees(repoRoot)
		if err != nil {
			return worktreesLoadedMsg{err: err}
		}
		git.MarkCurrent(trees, cwd)
		return worktreesLoadedMsg{worktrees: trees}
	}
}

func (m listModel) Update(msg tea.Msg) (listModel, tea.Cmd) {
	switch msg := msg.(type) {
	case worktreesLoadedMsg:
		m.worktrees = msg.worktrees
		m.err = msg.err
	}
	return m, nil
}

func (m listModel) moveUp() listModel {
	if m.cursor > 0 {
		m.cursor--
	}
	return m
}

func (m listModel) moveDown() listModel {
	if m.cursor < len(m.worktrees)-1 {
		m.cursor++
	}
	return m
}

func (m listModel) moveToTop() listModel {
	m.cursor = 0
	return m
}

func (m listModel) moveToBottom() listModel {
	if len(m.worktrees) > 0 {
		m.cursor = len(m.worktrees) - 1
	}
	return m
}

func (m listModel) selectedWorktree() *git.Worktree {
	if len(m.worktrees) == 0 {
		return nil
	}
	return &m.worktrees[m.cursor]
}

func (m listModel) View(width int) string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}
	if len(m.worktrees) == 0 {
		return dimStyle.Render("No worktrees found.")
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Worktrees"))
	b.WriteString("\n")

	for i, wt := range m.worktrees {
		cursor := "  "
		if i == m.cursor {
			cursor = "▸ "
		}

		name := wt.Branch
		if name == "" {
			name = "(detached)"
		}

		label := fmt.Sprintf("%s%s", cursor, name)

		switch {
		case wt.IsCurrent && i == m.cursor:
			b.WriteString(currentStyle.Render(label + " ●"))
		case wt.IsCurrent:
			b.WriteString(currentStyle.Render(label + " ●"))
		case i == m.cursor:
			b.WriteString(selectedStyle.Render(label))
		default:
			b.WriteString(dimStyle.Render(label))
		}
		b.WriteString("\n")
	}

	return b.String()
}
```

**Step 4: Create the app (root) model**

```go
// internal/tui/app.go
package tui

import (
	"fmt"
	"os"

	"git-treeflow/internal/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewState int

const (
	viewList viewState = iota
	viewCreate
	viewDelete
	viewSettings
	viewPrune
	viewHelp
)

type AppModel struct {
	view       viewState
	list       listModel
	create     createModel
	del        deleteModel
	settings   settingsModel
	prune      pruneModel
	showHelp   bool
	width      int
	height     int
	repoRoot   string
	cwd        string
	cfg        config.Config
	SwitchPath string // exported: read by main after quit
	err        error
}

func NewApp(repoRoot, cwd string, cfg config.Config) AppModel {
	return AppModel{
		view:     viewList,
		list:     newListModel(repoRoot),
		repoRoot: repoRoot,
		cwd:      cwd,
		cfg:      cfg,
	}
}

func (m AppModel) Init() tea.Cmd {
	return loadWorktrees(m.repoRoot, m.cwd)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case worktreesLoadedMsg:
		m.list, _ = m.list.Update(msg)
		return m, nil

	case createDoneMsg:
		m.view = viewList
		if msg.err != nil {
			m.err = msg.err
		}
		return m, loadWorktrees(m.repoRoot, m.cwd)

	case deleteDoneMsg:
		m.view = viewList
		if msg.err != nil {
			m.err = msg.err
		}
		return m, loadWorktrees(m.repoRoot, m.cwd)

	case pruneDoneMsg:
		m.view = viewList
		return m, loadWorktrees(m.repoRoot, m.cwd)

	case stashDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Stash succeeded, now switch
		wt := m.list.selectedWorktree()
		if wt != nil {
			m.SwitchPath = wt.Path
		}
		return m, tea.Quit
	}

	// Delegate to active view
	var cmd tea.Cmd
	switch m.view {
	case viewCreate:
		m.create, cmd = m.create.Update(msg)
	case viewDelete:
		m.del, cmd = m.del.Update(msg)
	case viewSettings:
		m.settings, cmd = m.settings.Update(msg)
	case viewPrune:
		m.prune, cmd = m.prune.Update(msg)
	}
	return m, cmd
}

func (m AppModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Help toggle works everywhere
	if msg.String() == "?" && m.view == viewList {
		m.showHelp = !m.showHelp
		return m, nil
	}

	if m.showHelp {
		m.showHelp = false
		return m, nil
	}

	// Delegate keys to sub-views when not on list
	if m.view != viewList {
		var cmd tea.Cmd
		switch m.view {
		case viewCreate:
			m.create, cmd = m.create.Update(msg)
		case viewDelete:
			m.del, cmd = m.del.Update(msg)
		case viewSettings:
			m.settings, cmd = m.settings.Update(msg)
		case viewPrune:
			m.prune, cmd = m.prune.Update(msg)
		}
		// Check for back/quit in sub-views
		if msg.String() == "esc" && m.view != viewCreate {
			m.view = viewList
			return m, loadWorktrees(m.repoRoot, m.cwd)
		}
		return m, cmd
	}

	// List view keys
	switch {
	case matches(msg, listKeys.Quit):
		return m, tea.Quit
	case matches(msg, listKeys.Up):
		m.list = m.list.moveUp()
	case matches(msg, listKeys.Down):
		m.list = m.list.moveDown()
	case matches(msg, listKeys.Top):
		m.list = m.list.moveToTop()
	case matches(msg, listKeys.Bottom):
		m.list = m.list.moveToBottom()
	case matches(msg, listKeys.Select):
		return m.handleSwitch()
	case matches(msg, listKeys.Create):
		m.view = viewCreate
		m.create = newCreateModel(m.repoRoot, m.cfg)
		return m, m.create.Init()
	case matches(msg, listKeys.Delete):
		wt := m.list.selectedWorktree()
		if wt != nil && !wt.IsMain {
			m.view = viewDelete
			m.del = newDeleteModel(*wt, m.repoRoot)
		}
		return m, nil
	case matches(msg, listKeys.Prune):
		m.view = viewPrune
		m.prune = newPruneModel(m.repoRoot)
		return m, m.prune.Init()
	case matches(msg, listKeys.Settings):
		m.view = viewSettings
		m.settings = newSettingsModel(m.cfg, m.repoRoot)
		return m, nil
	}

	return m, nil
}

func (m AppModel) handleSwitch() (tea.Model, tea.Cmd) {
	wt := m.list.selectedWorktree()
	if wt == nil {
		return m, nil
	}
	if wt.IsCurrent {
		return m, nil
	}

	// Check for uncommitted changes
	dirty, err := hasUncommittedChanges(m.cwd)
	if err != nil {
		m.err = err
		return m, nil
	}
	if dirty {
		// Show stash prompt inline — for now, auto-stash
		return m, stashAndSwitch(m.cwd, wt.Path)
	}

	m.SwitchPath = wt.Path
	return m, tea.Quit
}

func hasUncommittedChanges(dir string) (bool, error) {
	return gitPkg.HasUncommittedChanges(dir)
}

type stashDoneMsg struct{ err error }

func stashAndSwitch(cwd, targetPath string) tea.Cmd {
	return func() tea.Msg {
		err := gitPkg.Stash(cwd, "git-treeflow: auto-stash before switch")
		return stashDoneMsg{err: err}
	}
}

func matches(msg tea.KeyMsg, binding key.Binding) bool {
	return key.Matches(msg, binding)
}

func (m AppModel) View() string {
	if m.showHelp {
		return m.helpView()
	}

	var content string
	switch m.view {
	case viewList:
		content = m.list.View(m.width)
	case viewCreate:
		content = m.create.View()
	case viewDelete:
		content = m.del.View()
	case viewSettings:
		content = m.settings.View()
	case viewPrune:
		content = m.prune.View()
	}

	if m.err != nil {
		content += "\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	statusBar := statusBarStyle.Render("? help • c create • d delete • p prune • s settings • q quit")

	return lipgloss.JoinVertical(lipgloss.Left, content, statusBar)
}

func (m AppModel) helpView() string {
	help := titleStyle.Render("Keybindings") + "\n\n"
	help += "  j/↓     move down\n"
	help += "  k/↑     move up\n"
	help += "  g       jump to top\n"
	help += "  G       jump to bottom\n"
	help += "  enter   switch to worktree\n"
	help += "  c       create worktree\n"
	help += "  d       delete worktree\n"
	help += "  p       prune stale worktrees\n"
	help += "  s       settings\n"
	help += "  ?       toggle help\n"
	help += "  q/esc   quit / back\n"
	help += "\n" + dimStyle.Render("Press any key to close")
	return help
}

// RunApp starts the TUI and returns the path to switch to (empty if none).
func RunApp(repoRoot, cwd string, cfg config.Config) (string, error) {
	app := NewApp(repoRoot, cwd, cfg)
	p := tea.NewProgram(app, tea.WithOutput(os.Stderr), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}
	return finalModel.(AppModel).SwitchPath, nil
}
```

Note: This file references `gitPkg`, `createModel`, `deleteModel`, `settingsModel`, `pruneModel` — these are placeholders. The sub-view models will be added in subsequent tasks, and `gitPkg` will be replaced with direct function calls. The important thing is to get the routing architecture right.

**Step 5: Verify it compiles (will need stub types first — see next tasks)**

This task's code won't compile on its own since it references types from tasks 9-13. Move on to the next tasks to fill in the sub-models, then verify compilation.

**Step 6: Commit**

```bash
git add internal/tui/
git commit -m "feat: add TUI app skeleton with list view, styles, and vim keybindings"
```

---

### Task 9: TUI — Create Flow

**Files:**
- Create: `internal/tui/create.go`

**Step 1: Implement the create flow model**

```go
// internal/tui/create.go
package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"git-treeflow/internal/config"
	gitpkg "git-treeflow/internal/git"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
)

type createStep int

const (
	stepName createStep = iota
	stepBranchMode
	stepBranchName
	stepBranchSelect
	stepConfirm
	stepCreating
)

type branchMode int

const (
	branchNew branchMode = iota
	branchLocal
	branchRemote
)

type createModel struct {
	step         createStep
	nameInput    textinput.Model
	branchInput  textinput.Model
	searchInput  textinput.Model
	branchMode   branchMode
	modeCursor   int
	branches     []string
	filtered     []string
	branchCursor int
	repoRoot     string
	cfg          config.Config
	err          error
	worktreeName string
	branchName   string
}

type createDoneMsg struct {
	err      error
	wtPath   string
}

type branchesLoadedMsg struct {
	branches []string
	err      error
}

func newCreateModel(repoRoot string, cfg config.Config) createModel {
	ni := textinput.New()
	ni.Placeholder = "worktree-name"
	ni.Focus()

	bi := textinput.New()
	bi.Placeholder = "branch-name"

	si := textinput.New()
	si.Placeholder = "filter branches..."

	return createModel{
		step:        stepName,
		nameInput:   ni,
		branchInput: bi,
		searchInput: si,
		repoRoot:    repoRoot,
		cfg:         cfg,
	}
}

func (m createModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m createModel) Update(msg tea.Msg) (createModel, tea.Cmd) {
	switch msg := msg.(type) {
	case branchesLoadedMsg:
		m.branches = msg.branches
		m.filtered = msg.branches
		m.err = msg.err
		return m, nil

	case createDoneMsg:
		// Bubble up to app
		return m, func() tea.Msg { return msg }

	case tea.KeyMsg:
		if msg.String() == "esc" {
			if m.step == stepName {
				return m, func() tea.Msg { return createDoneMsg{} }
			}
			m.step--
			if m.step == stepBranchMode {
				m.branchInput.Blur()
				m.searchInput.Blur()
			}
			if m.step == stepName {
				m.nameInput.Focus()
			}
			return m, nil
		}
		return m.handleKey(msg)
	}

	// Update active text input
	var cmd tea.Cmd
	switch m.step {
	case stepName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case stepBranchName:
		m.branchInput, cmd = m.branchInput.Update(msg)
	case stepBranchSelect:
		m.searchInput, cmd = m.searchInput.Update(msg)
	}
	return m, cmd
}

func (m createModel) handleKey(msg tea.KeyMsg) (createModel, tea.Cmd) {
	switch m.step {
	case stepName:
		switch msg.String() {
		case "enter":
			val := strings.TrimSpace(m.nameInput.Value())
			if val == "" {
				return m, nil
			}
			m.worktreeName = val
			m.step = stepBranchMode
			m.nameInput.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			return m, cmd
		}

	case stepBranchMode:
		switch msg.String() {
		case "j", "down":
			if m.modeCursor < 2 {
				m.modeCursor++
			}
		case "k", "up":
			if m.modeCursor > 0 {
				m.modeCursor--
			}
		case "enter":
			m.branchMode = branchMode(m.modeCursor)
			switch m.branchMode {
			case branchNew:
				m.step = stepBranchName
				m.branchInput.Focus()
				return m, textinput.Blink
			case branchLocal:
				m.step = stepBranchSelect
				m.searchInput.Focus()
				return m, tea.Batch(textinput.Blink, m.loadBranches(false))
			case branchRemote:
				m.step = stepBranchSelect
				m.searchInput.Focus()
				return m, tea.Batch(textinput.Blink, m.loadBranches(true))
			}
		}
		return m, nil

	case stepBranchName:
		switch msg.String() {
		case "enter":
			val := strings.TrimSpace(m.branchInput.Value())
			if val == "" {
				return m, nil
			}
			m.branchName = val
			m.step = stepConfirm
			m.branchInput.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.branchInput, cmd = m.branchInput.Update(msg)
			return m, cmd
		}

	case stepBranchSelect:
		switch msg.String() {
		case "enter":
			if len(m.filtered) > 0 {
				m.branchName = m.filtered[m.branchCursor]
				m.step = stepConfirm
				m.searchInput.Blur()
			}
			return m, nil
		case "ctrl+j", "ctrl+n":
			if m.branchCursor < len(m.filtered)-1 {
				m.branchCursor++
			}
			return m, nil
		case "ctrl+k", "ctrl+p":
			if m.branchCursor > 0 {
				m.branchCursor--
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			// Filter branches
			query := m.searchInput.Value()
			if query == "" {
				m.filtered = m.branches
			} else {
				matches := fuzzy.Find(query, m.branches)
				m.filtered = make([]string, len(matches))
				for i, match := range matches {
					m.filtered[i] = match.Str
				}
			}
			m.branchCursor = 0
			return m, cmd
		}

	case stepConfirm:
		switch msg.String() {
		case "enter", "y":
			m.step = stepCreating
			return m, m.doCreate()
		}
		return m, nil
	}

	return m, nil
}

func (m createModel) loadBranches(remote bool) tea.Cmd {
	return func() tea.Msg {
		var branches []string
		var err error
		if remote {
			branches, err = gitpkg.ListRemoteBranches(m.repoRoot)
		} else {
			branches, err = gitpkg.ListLocalBranches(m.repoRoot)
		}
		return branchesLoadedMsg{branches: branches, err: err}
	}
}

func (m createModel) doCreate() tea.Cmd {
	return func() tea.Msg {
		repoName, err := gitpkg.RepoName(m.repoRoot)
		if err != nil {
			return createDoneMsg{err: err}
		}

		vars := map[string]string{
			"repoName":     repoName,
			"worktreeName": m.worktreeName,
			"branchName":   m.branchName,
			"date":         time.Now().Format("2006-01-02"),
		}

		relPath := config.ExpandPath(m.cfg.WorktreePath, vars)
		wtPath := filepath.Join(m.repoRoot, relPath)
		wtPath, _ = filepath.Abs(wtPath)

		isNew := m.branchMode == branchNew
		branch := m.branchName
		// For remote branches, strip the remote prefix for the local branch
		if m.branchMode == branchRemote {
			parts := strings.SplitN(branch, "/", 2)
			if len(parts) == 2 {
				branch = parts[1]
			}
		}

		err = gitpkg.CreateWorktree(m.repoRoot, wtPath, branch, isNew)
		if err != nil {
			return createDoneMsg{err: err}
		}

		// Copy files
		copyFiles(m.repoRoot, wtPath, m.cfg.CopyFiles)

		// Run post-create hooks
		runHooks(wtPath, m.cfg.PostCreateHooks)

		return createDoneMsg{wtPath: wtPath}
	}
}

func (m createModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Create Worktree"))
	b.WriteString("\n\n")

	switch m.step {
	case stepName:
		b.WriteString("Worktree name:\n")
		b.WriteString(m.nameInput.View())
		b.WriteString("\n\n" + dimStyle.Render("enter to continue • esc to cancel"))

	case stepBranchMode:
		b.WriteString(fmt.Sprintf("Worktree: %s\n\n", selectedStyle.Render(m.worktreeName)))
		b.WriteString("Branch mode:\n\n")
		modes := []string{"Create new branch", "Checkout local branch", "Checkout remote branch"}
		for i, mode := range modes {
			cursor := "  "
			if i == m.modeCursor {
				cursor = "▸ "
			}
			if i == m.modeCursor {
				b.WriteString(selectedStyle.Render(cursor + mode))
			} else {
				b.WriteString(dimStyle.Render(cursor + mode))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n" + dimStyle.Render("j/k navigate • enter select • esc back"))

	case stepBranchName:
		b.WriteString(fmt.Sprintf("Worktree: %s\n", selectedStyle.Render(m.worktreeName)))
		b.WriteString("New branch name:\n")
		b.WriteString(m.branchInput.View())
		b.WriteString("\n\n" + dimStyle.Render("enter to continue • esc back"))

	case stepBranchSelect:
		b.WriteString(fmt.Sprintf("Worktree: %s\n", selectedStyle.Render(m.worktreeName)))
		b.WriteString("Select branch:\n")
		b.WriteString(m.searchInput.View())
		b.WriteString("\n\n")
		if len(m.filtered) == 0 {
			b.WriteString(dimStyle.Render("No branches found"))
		} else {
			limit := 10
			if len(m.filtered) < limit {
				limit = len(m.filtered)
			}
			for i := 0; i < limit; i++ {
				cursor := "  "
				if i == m.branchCursor {
					cursor = "▸ "
				}
				if i == m.branchCursor {
					b.WriteString(selectedStyle.Render(cursor + m.filtered[i]))
				} else {
					b.WriteString(dimStyle.Render(cursor + m.filtered[i]))
				}
				b.WriteString("\n")
			}
			if len(m.filtered) > limit {
				b.WriteString(dimStyle.Render(fmt.Sprintf("  ... and %d more", len(m.filtered)-limit)))
			}
		}
		b.WriteString("\n" + dimStyle.Render("ctrl+j/k navigate • enter select • esc back"))

	case stepConfirm:
		b.WriteString("Confirm creation:\n\n")
		b.WriteString(fmt.Sprintf("  Worktree:  %s\n", selectedStyle.Render(m.worktreeName)))
		b.WriteString(fmt.Sprintf("  Branch:    %s\n", selectedStyle.Render(m.branchName)))
		modeStr := "new"
		if m.branchMode == branchLocal {
			modeStr = "local"
		} else if m.branchMode == branchRemote {
			modeStr = "remote"
		}
		b.WriteString(fmt.Sprintf("  Mode:      %s\n", dimStyle.Render(modeStr)))
		b.WriteString("\n" + dimStyle.Render("enter/y confirm • esc back"))

	case stepCreating:
		b.WriteString("Creating worktree...")
	}

	if m.err != nil {
		b.WriteString("\n\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	return b.String()
}
```

**Step 2: Verify it compiles (after subsequent tasks provide remaining stubs)**

**Step 3: Commit**

```bash
git add internal/tui/create.go
git commit -m "feat: add create worktree TUI flow with multi-step wizard"
```

---

### Task 10: TUI — Delete Confirmation

**Files:**
- Create: `internal/tui/delete.go`

**Step 1: Implement delete confirmation model**

```go
// internal/tui/delete.go
package tui

import (
	"fmt"
	"strings"

	gitpkg "git-treeflow/internal/git"

	tea "github.com/charmbracelet/bubbletea"
)

type deleteModel struct {
	worktree     gitpkg.Worktree
	deleteBranch bool
	repoRoot     string
	confirmed    bool
	err          error
}

type deleteDoneMsg struct {
	err error
}

func newDeleteModel(wt gitpkg.Worktree, repoRoot string) deleteModel {
	return deleteModel{
		worktree: wt,
		repoRoot: repoRoot,
	}
}

func (m deleteModel) Update(msg tea.Msg) (deleteModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y":
			m.confirmed = true
			return m, m.doDelete()
		case "tab", " ":
			m.deleteBranch = !m.deleteBranch
			return m, nil
		case "esc", "n", "q":
			return m, func() tea.Msg { return deleteDoneMsg{} }
		}
	case deleteDoneMsg:
		return m, func() tea.Msg { return msg }
	}
	return m, nil
}

func (m deleteModel) doDelete() tea.Cmd {
	return func() tea.Msg {
		err := gitpkg.RemoveWorktree(m.repoRoot, m.worktree.Path, true)
		if err != nil {
			return deleteDoneMsg{err: err}
		}
		if m.deleteBranch && m.worktree.Branch != "" {
			err = gitpkg.DeleteBranch(m.repoRoot, m.worktree.Branch, true)
		}
		return deleteDoneMsg{err: err}
	}
}

func (m deleteModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Delete Worktree"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Worktree: %s\n", errorStyle.Render(m.worktree.Branch)))
	b.WriteString(fmt.Sprintf("  Path:     %s\n", dimStyle.Render(m.worktree.Path)))
	b.WriteString("\n")

	toggle := "[ ]"
	if m.deleteBranch {
		toggle = "[x]"
	}
	b.WriteString(fmt.Sprintf("  %s Also delete branch\n", toggle))

	b.WriteString("\n" + dimStyle.Render("y confirm • tab toggle branch deletion • esc cancel"))

	if m.err != nil {
		b.WriteString("\n\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	return b.String()
}
```

**Step 2: Commit**

```bash
git add internal/tui/delete.go
git commit -m "feat: add delete worktree confirmation view"
```

---

### Task 11: TUI — Prune View

**Files:**
- Create: `internal/tui/prune.go`

**Step 1: Implement prune model**

```go
// internal/tui/prune.go
package tui

import (
	"fmt"
	"strings"

	gitpkg "git-treeflow/internal/git"

	tea "github.com/charmbracelet/bubbletea"
)

type pruneModel struct {
	staleWorktrees []gitpkg.Worktree
	staleBranches  []string
	repoRoot       string
	loading        bool
	err            error
}

type pruneDoneMsg struct{}

type pruneDataLoadedMsg struct {
	staleWorktrees []gitpkg.Worktree
	staleBranches  []string
	err            error
}

func newPruneModel(repoRoot string) pruneModel {
	return pruneModel{
		repoRoot: repoRoot,
		loading:  true,
	}
}

func (m pruneModel) Init() tea.Cmd {
	return func() tea.Msg {
		staleWt, err := gitpkg.StaleWorktrees(m.repoRoot)
		if err != nil {
			return pruneDataLoadedMsg{err: err}
		}
		staleBr, err := gitpkg.StaleBranches(m.repoRoot)
		if err != nil {
			return pruneDataLoadedMsg{err: err}
		}
		return pruneDataLoadedMsg{
			staleWorktrees: staleWt,
			staleBranches:  staleBr,
		}
	}
}

func (m pruneModel) Update(msg tea.Msg) (pruneModel, tea.Cmd) {
	switch msg := msg.(type) {
	case pruneDataLoadedMsg:
		m.loading = false
		m.staleWorktrees = msg.staleWorktrees
		m.staleBranches = msg.staleBranches
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "y":
			return m, m.doPrune()
		case "esc", "q", "n":
			return m, func() tea.Msg { return pruneDoneMsg{} }
		}
	}
	return m, nil
}

func (m pruneModel) doPrune() tea.Cmd {
	return func() tea.Msg {
		// Prune stale worktrees
		gitpkg.PruneWorktrees(m.repoRoot)

		// Delete stale branches
		for _, branch := range m.staleBranches {
			gitpkg.DeleteBranch(m.repoRoot, branch, true)
		}

		return pruneDoneMsg{}
	}
}

func (m pruneModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Prune"))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString("Scanning for stale worktrees and branches...")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n" + dimStyle.Render("esc to go back"))
		return b.String()
	}

	hasStale := len(m.staleWorktrees) > 0 || len(m.staleBranches) > 0
	if !hasStale {
		b.WriteString(successStyle.Render("Nothing to prune — all clean!"))
		b.WriteString("\n\n" + dimStyle.Render("esc to go back"))
		return b.String()
	}

	if len(m.staleWorktrees) > 0 {
		b.WriteString("Stale worktrees (directory missing):\n")
		for _, wt := range m.staleWorktrees {
			b.WriteString(fmt.Sprintf("  • %s (%s)\n", errorStyle.Render(wt.Branch), dimStyle.Render(wt.Path)))
		}
		b.WriteString("\n")
	}

	if len(m.staleBranches) > 0 {
		b.WriteString("Branches with deleted remote:\n")
		for _, branch := range m.staleBranches {
			b.WriteString(fmt.Sprintf("  • %s\n", errorStyle.Render(branch)))
		}
		b.WriteString("\n")
	}

	b.WriteString(dimStyle.Render("y prune all • esc cancel"))
	return b.String()
}
```

**Step 2: Commit**

```bash
git add internal/tui/prune.go
git commit -m "feat: add prune view for stale worktrees and deleted-remote branches"
```

---

### Task 12: TUI — Settings View

**Files:**
- Create: `internal/tui/settings.go`

**Step 1: Implement settings model**

```go
// internal/tui/settings.go
package tui

import (
	"fmt"
	"strings"

	"git-treeflow/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type settingsModel struct {
	cfg      config.Config
	repoRoot string
	cursor   int
	editing  bool
	input    textinput.Model
	fields   []settingsField
	saveMode int // 0 = not saving, 1 = choosing where to save
	err      error
	saved    bool
}

type settingsField struct {
	label string
	key   string
	value string
}

func newSettingsModel(cfg config.Config, repoRoot string) settingsModel {
	ti := textinput.New()

	fields := []settingsField{
		{label: "Worktree Path", key: "worktree_path", value: cfg.WorktreePath},
		{label: "Copy Files", key: "copy_files", value: strings.Join(cfg.CopyFiles, ", ")},
		{label: "Post-Create Hooks", key: "post_create_hooks", value: strings.Join(cfg.PostCreateHooks, ", ")},
	}

	return settingsModel{
		cfg:      cfg,
		repoRoot: repoRoot,
		input:    ti,
		fields:   fields,
	}
}

func (m settingsModel) Update(msg tea.Msg) (settingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.saveMode == 1 {
			return m.handleSaveChoice(msg)
		}
		if m.editing {
			return m.handleEditing(msg)
		}
		return m.handleNavigation(msg)
	}

	if m.editing {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m settingsModel) handleNavigation(msg tea.KeyMsg) (settingsModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.fields)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		m.editing = true
		m.input.SetValue(m.fields[m.cursor].value)
		m.input.Focus()
		return m, textinput.Blink
	case "w":
		m.saveMode = 1
	}
	return m, nil
}

func (m settingsModel) handleEditing(msg tea.KeyMsg) (settingsModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.fields[m.cursor].value = m.input.Value()
		m.editing = false
		m.input.Blur()
		m.applyFields()
		return m, nil
	case "esc":
		m.editing = false
		m.input.Blur()
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m settingsModel) handleSaveChoice(msg tea.KeyMsg) (settingsModel, tea.Cmd) {
	switch msg.String() {
	case "g":
		err := config.Save(m.cfg, config.GlobalConfigPath())
		m.err = err
		m.saveMode = 0
		if err == nil {
			m.saved = true
		}
	case "r":
		err := config.Save(m.cfg, config.RepoConfigPath(m.repoRoot))
		m.err = err
		m.saveMode = 0
		if err == nil {
			m.saved = true
		}
	case "esc":
		m.saveMode = 0
	}
	return m, nil
}

func (m *settingsModel) applyFields() {
	for _, f := range m.fields {
		switch f.key {
		case "worktree_path":
			m.cfg.WorktreePath = f.value
		case "copy_files":
			m.cfg.CopyFiles = splitAndTrim(f.value)
		case "post_create_hooks":
			m.cfg.PostCreateHooks = splitAndTrim(f.value)
		}
	}
}

func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func (m settingsModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Settings"))
	b.WriteString("\n\n")

	for i, f := range m.fields {
		cursor := "  "
		if i == m.cursor {
			cursor = "▸ "
		}

		if m.editing && i == m.cursor {
			b.WriteString(fmt.Sprintf("%s%s:\n    %s\n", cursor, f.label, m.input.View()))
		} else {
			label := fmt.Sprintf("%s%s: %s", cursor, f.label, f.value)
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(label))
			} else {
				b.WriteString(dimStyle.Render(label))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	if m.saveMode == 1 {
		b.WriteString("Save to: " + selectedStyle.Render("g") + " global • " + selectedStyle.Render("r") + " repo • esc cancel")
	} else if m.saved {
		b.WriteString(successStyle.Render("Saved!") + "\n")
		b.WriteString(dimStyle.Render("enter edit • w save • esc back"))
	} else {
		b.WriteString(dimStyle.Render("enter edit • w save • esc back"))
	}

	if m.err != nil {
		b.WriteString("\n\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	return b.String()
}
```

**Step 2: Commit**

```bash
git add internal/tui/settings.go
git commit -m "feat: add settings view with inline editing and save to global/repo"
```

---

### Task 13: TUI — Helpers (File Copy, Hooks) & Wire Up App

**Files:**
- Create: `internal/tui/helpers.go`
- Modify: `internal/tui/app.go` — remove placeholder references, wire everything

**Step 1: Create helpers for file copying and hook execution**

```go
// internal/tui/helpers.go
package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gitpkg "git-treeflow/internal/git"
)

// Re-export git functions used by app.go to avoid circular reference confusion
var gitHasUncommittedChanges = gitpkg.HasUncommittedChanges
var gitStash = gitpkg.Stash

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
		cmd.Stdout = os.Stderr // TUI renders to stderr
		cmd.Stderr = os.Stderr
		cmd.Run() // Best effort — don't fail worktree creation on hook failure
	}
}
```

**Step 2: Update app.go to use real git functions instead of placeholders**

Replace the `hasUncommittedChanges` and `stashAndSwitch` functions in `app.go`:

Replace:
```go
func hasUncommittedChanges(dir string) (bool, error) {
	return gitPkg.HasUncommittedChanges(dir)
}
```
With:
```go
func hasUncommittedChanges(dir string) (bool, error) {
	return gitHasUncommittedChanges(dir)
}
```

Replace:
```go
func stashAndSwitch(cwd, targetPath string) tea.Cmd {
	return func() tea.Msg {
		err := gitPkg.Stash(cwd, "git-treeflow: auto-stash before switch")
		return stashDoneMsg{err: err}
	}
}
```
With:
```go
func stashAndSwitch(cwd, targetPath string) tea.Cmd {
	return func() tea.Msg {
		err := gitStash(cwd, "git-treeflow: auto-stash before switch")
		return stashDoneMsg{err: err}
	}
}
```

And add the proper import for `key`:
```go
import (
	"fmt"
	"os"

	"git-treeflow/internal/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)
```

**Step 3: Verify the TUI package compiles**

```bash
go build ./internal/tui/
```

Expected: no errors.

**Step 4: Commit**

```bash
git add internal/tui/
git commit -m "feat: add file copy helpers, hook execution, and wire up TUI app"
```

---

### Task 14: CLI Entry Point — main.go

**Files:**
- Modify: `cmd/gtf/main.go`

**Step 1: Implement the CLI entry point with flags**

```go
// cmd/gtf/main.go
package main

import (
	"fmt"
	"os"
	"strings"

	"git-treeflow/internal/config"
	gitpkg "git-treeflow/internal/git"
	"git-treeflow/internal/shell"
	"git-treeflow/internal/tui"

	"github.com/sahilm/fuzzy"
)

func main() {
	args := os.Args[1:]

	// Handle --init flag
	if len(args) >= 2 && args[0] == "--init" {
		script, err := shell.InitScript(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(script)
		return
	}

	// Handle --help
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		printUsage()
		return
	}

	// Handle --version
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-v") {
		fmt.Println("git-treeflow v0.1.0")
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	repoRoot, err := gitpkg.RepoRoot(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: not a git repository\n")
		os.Exit(1)
	}

	cfg, err := config.Load(repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	// Direct jump: gtf <name>
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		handleDirectJump(repoRoot, args[0])
		return
	}

	// Launch TUI
	switchPath, err := tui.RunApp(repoRoot, cwd, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if switchPath != "" {
		fmt.Println(switchPath)
	}
}

func handleDirectJump(repoRoot, name string) {
	trees, err := gitpkg.ListWorktrees(repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Exact match first
	for _, wt := range trees {
		if wt.Branch == name {
			fmt.Println(wt.Path)
			return
		}
	}

	// Fuzzy match
	branches := make([]string, len(trees))
	for i, wt := range trees {
		branches[i] = wt.Branch
	}
	matches := fuzzy.Find(name, branches)
	if len(matches) == 1 {
		fmt.Println(trees[matches[0].Index].Path)
		return
	}
	if len(matches) > 1 {
		fmt.Fprintf(os.Stderr, "Multiple matches for %q:\n", name)
		for _, m := range matches {
			fmt.Fprintf(os.Stderr, "  %s\n", trees[m.Index].Branch)
		}
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "No worktree matching %q\n", name)
	os.Exit(1)
}

func printUsage() {
	fmt.Fprint(os.Stderr, `git-treeflow — manage git worktrees with a TUI

Usage:
  gtf                 Launch the TUI
  gtf <name>          Jump directly to a worktree by name (fuzzy match)
  gtf --init <shell>  Print shell init script (zsh, bash, fish)
  gtf --help          Show this help
  gtf --version       Show version

Setup:
  Add to your shell config:
    eval "$(gtf --init zsh)"    # for zsh
    eval "$(gtf --init bash)"   # for bash
    gtf --init fish | source    # for fish
`)
}
```

**Step 2: Build and verify**

```bash
go build -o gtf ./cmd/gtf
./gtf --help
./gtf --version
./gtf --init zsh
```

Expected: help text prints, version prints, shell function prints.

**Step 3: Commit**

```bash
git add cmd/gtf/main.go
git commit -m "feat: add CLI entry point with --init, --help, direct jump, and TUI launch"
```

---

### Task 15: Example Config & .gitignore

**Files:**
- Create: `config.toml.example`
- Create: `.gitignore`

**Step 1: Create example config**

```toml
# git-treeflow configuration
# Place at ~/.config/git-treeflow/config.toml (global)
# or .git-treeflow.toml in your repo root (per-repo override)

# Where to create worktrees relative to the repo root
# Available variables: {repoName}, {worktreeName}, {branchName}, {date}
worktree_path = "../{repoName}.worktrees/{worktreeName}"

# Files/globs to copy from the main repo into new worktrees
copy_files = [".env*"]

# Commands to run after creating a worktree (executed in order, from the new worktree dir)
# post_create_hooks = [
#     "npm install",
#     "cp .env.example .env.local",
# ]
```

**Step 2: Create .gitignore**

```
gtf
*.exe
dist/
```

**Step 3: Commit**

```bash
git add config.toml.example .gitignore
git commit -m "feat: add example config and .gitignore"
```

---

### Task 16: Integration — Build, Manual Test, Fix Compilation Issues

**Files:**
- Potentially modify any file from prior tasks

**Step 1: Build the full binary**

```bash
go build -o gtf ./cmd/gtf
```

Fix any compilation errors that arise from cross-task integration (missing imports, type mismatches, etc.).

**Step 2: Run all tests**

```bash
go test ./... -v
```

Fix any test failures.

**Step 3: Test shell integration manually**

```bash
./gtf --init zsh
./gtf --version
./gtf --help
```

**Step 4: Test TUI in a real repo**

```bash
./gtf
```

Verify: list view shows, vim keys work, help overlay works.

**Step 5: Test create flow**

Press `c`, enter a name, select branch mode, confirm. Verify worktree is created.

**Step 6: Test delete flow**

Press `d` on a non-main worktree, confirm. Verify worktree is removed.

**Step 7: Commit any fixes**

```bash
git add -A
git commit -m "fix: resolve compilation and integration issues"
```

---

### Task 17: Final — Run `go vet`, `go fmt`, clean up

**Step 1: Format and vet**

```bash
gofmt -w .
go vet ./...
```

**Step 2: Run tests one final time**

```bash
go test ./... -v
```

**Step 3: Final commit**

```bash
git add -A
git commit -m "chore: format code and fix vet warnings"
```
