# git-treeflow

A terminal UI for managing git worktrees, launched via `gtf`.

## Installation

### From source (requires Go 1.21+)

```bash
go install github.com/cKreymborg/git-treeflow/cmd/gtf@latest
```

Or clone and build:

```bash
git clone https://github.com/cKreymborg/git-treeflow.git
cd git-treeflow
go install ./cmd/gtf
```

### Shell setup (required)

The `gtf` command needs a shell wrapper to `cd` into worktrees. Add one of these to your shell config:

**zsh** (`~/.zshrc`):
```bash
eval "$(gtf --init zsh)"
```

**bash** (`~/.bashrc`):
```bash
eval "$(gtf --init bash)"
```

**fish** (`~/.config/fish/config.fish`):
```fish
gtf --init fish | source
```

Restart your shell or `source` the config file.

## Usage

```bash
gtf              # Launch the TUI
gtf <name>       # Jump directly to a worktree (fuzzy match)
gtf --init zsh   # Print shell init script
gtf --help       # Show help
gtf --version    # Show version
```

### TUI Keybindings

| Key | Action |
|-----|--------|
| `j` / `k` | Move down / up |
| `g` / `G` | Jump to top / bottom |
| `Enter` | Switch to selected worktree |
| `c` | Create new worktree |
| `d` | Delete worktree |
| `p` | Prune stale worktrees |
| `s` | Settings |
| `?` | Help |
| `q` / `Esc` | Quit / back |

### Creating a worktree

Press `c` to start the create flow:

1. Enter a worktree name
2. Choose branch mode: new branch, local branch, or remote branch
3. Select or enter the branch name (with fuzzy search for existing branches)
4. Confirm

New worktrees are created at `../{repoName}.worktree/{worktreeName}` by default. Files matching `.env*` are automatically copied from the main repo.

## Configuration

Config uses TOML with two layers:

- **Global:** `~/.config/git-treeflow/config.toml`
- **Per-repo:** `.git-treeflow.toml` in the repo root

Per-repo settings override global settings. Edit from the TUI via `s`, or directly:

```toml
# Where to create worktrees (relative to repo root)
# Variables: {repoName}, {worktreeName}, {branchName}, {date}
worktree_path = "../{repoName}.worktree/{worktreeName}"

# Files/globs to copy into new worktrees
copy_files = [".env*"]

# Commands to run after creating a worktree
post_create_hooks = [
    "npm install",
]
```

## License

MIT
