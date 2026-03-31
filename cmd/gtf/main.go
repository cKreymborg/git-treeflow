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
