package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const KeyAutoFetchRemoteBranches = "auto_fetch_remote_branches"

type Config struct {
	WorktreePath            string   `toml:"worktree_path"`
	CopyFiles               []string `toml:"copy_files"`
	PostCreateHooks         []string `toml:"post_create_hooks"`
	AutoFetchRemoteBranches bool     `toml:"auto_fetch_remote_branches"`
}

func DefaultConfig() Config {
	return Config{
		WorktreePath:            "../{repoName}.worktree/{worktreeName}",
		CopyFiles:               []string{".env*"},
		AutoFetchRemoteBranches: true,
	}
}

func LoadFromFile(path string) (Config, error) {
	cfg, _, err := loadFromFileWithMeta(path)
	return cfg, err
}

func loadFromFileWithMeta(path string) (Config, toml.MetaData, error) {
	var cfg Config
	var meta toml.MetaData
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, meta, nil
	}
	meta, err := toml.DecodeFile(path, &cfg)
	return cfg, meta, err
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
	global, gMeta, err := loadFromFileWithMeta(GlobalConfigPath())
	if err != nil {
		return cfg, err
	}
	cfg = mergeWithMeta(cfg, global, gMeta)
	repo, rMeta, err := loadFromFileWithMeta(RepoConfigPath(repoRoot))
	if err != nil {
		return cfg, err
	}
	cfg = mergeWithMeta(cfg, repo, rMeta)
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

// mergeWithMeta layers override onto base, using meta to detect explicitly-set
// bool fields whose Go zero value can't be distinguished from "unset".
func mergeWithMeta(base, override Config, meta toml.MetaData) Config {
	result := Merge(base, override)
	if meta.IsDefined(KeyAutoFetchRemoteBranches) {
		result.AutoFetchRemoteBranches = override.AutoFetchRemoteBranches
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
