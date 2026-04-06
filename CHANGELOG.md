# Changelog

## v0.2.1

- Fix cursor disappearing after deleting the last worktree in the list (#1)

## v0.2.0

- Purple neon UI with gradient logo, bordered panels, and display toggle
- Shell wrapper detection with setup tip for new users
- Version display on start screen
- Responsive footer that wraps on narrow terminals
- GoReleaser + GitHub Actions release pipeline
- Homebrew tap distribution

## v0.1.0

- Initial release
- TUI with worktree list, create, delete, prune, and settings views
- Vim keybindings (j/k/g/G)
- Direct jump via `gtf <name>` with fuzzy matching
- Shell integration (zsh, bash, fish) for cd-on-exit
- Two-layer TOML config (global + per-repo)
- Configurable worktree path templates
- File copying (.env*) and post-create hooks
- Stash-aware worktree switching
- Stale worktree and branch pruning
