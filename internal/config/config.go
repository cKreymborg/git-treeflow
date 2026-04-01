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
		WorktreePath: "../{repoName}.worktree/{worktreeName}",
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
