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
	if cfg.WorktreePath != "" {
		t.Error("expected empty config for missing file")
	}
}

func TestMerge(t *testing.T) {
	global := Config{
		WorktreePath:    "../{repoName}.worktree/{worktreeName}",
		CopyFiles:       []string{".env"},
		PostCreateHooks: []string{"make setup"},
	}
	repo := Config{
		CopyFiles: []string{".env", ".env.local"},
	}
	merged := Merge(global, repo)
	if len(merged.CopyFiles) != 2 {
		t.Errorf("expected repo copy_files to win, got %v", merged.CopyFiles)
	}
	if merged.WorktreePath != "../{repoName}.worktree/{worktreeName}" {
		t.Errorf("expected global worktree_path, got %q", merged.WorktreePath)
	}
	if len(merged.PostCreateHooks) != 1 {
		t.Errorf("expected global hooks, got %v", merged.PostCreateHooks)
	}
}

func TestExpandPath(t *testing.T) {
	tmpl := "../{repoName}.worktree/{worktreeName}"
	vars := map[string]string{
		"repoName":     "my-app",
		"worktreeName": "feature-x",
		"branchName":   "feat/x",
		"date":         "2026-03-31",
	}
	result := ExpandPath(tmpl, vars)
	expected := "../my-app.worktree/feature-x"
	if result != expected {
		t.Errorf("ExpandPath = %q, want %q", result, expected)
	}
}

func TestDefaultConfig_AutoFetchTrue(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.AutoFetchRemoteBranches {
		t.Error("expected AutoFetchRemoteBranches default true")
	}
}

func TestLoad_AutoFetchOverrideFalse(t *testing.T) {
	repoRoot := t.TempDir()
	repoCfg := `auto_fetch_remote_branches = false` + "\n"
	if err := os.WriteFile(filepath.Join(repoRoot, ".git-treeflow.toml"), []byte(repoCfg), 0644); err != nil {
		t.Fatalf("write repo config: %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load(repoRoot)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AutoFetchRemoteBranches {
		t.Error("expected AutoFetchRemoteBranches=false from repo config to override default true")
	}
}

func TestLoad_AutoFetchUnsetKeepsDefault(t *testing.T) {
	repoRoot := t.TempDir()
	repoCfg := `worktree_path = "../{repoName}.wt/{worktreeName}"` + "\n"
	if err := os.WriteFile(filepath.Join(repoRoot, ".git-treeflow.toml"), []byte(repoCfg), 0644); err != nil {
		t.Fatalf("write repo config: %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load(repoRoot)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.AutoFetchRemoteBranches {
		t.Error("expected AutoFetchRemoteBranches to remain true when repo config does not set it")
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
