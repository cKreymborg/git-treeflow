# Future Todos

## Togglable List Detail Levels

Allow users to cycle between detail levels in the worktree list via a keybinding:

- **Minimal** — worktree name and branch name (v1 default)
- **Medium** — name, branch, clean/dirty status indicator
- **Rich** — name, branch, clean/dirty status, last commit message, ahead/behind remote

## Global Worktree Navigator

When `gtf` is run outside a git repo context, scan for repos with worktrees in common parent directories and let the user jump to any of them. Essentially a cross-repo worktree navigator.
