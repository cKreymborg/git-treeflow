# git-treeflow project notes

## UI conventions

### Spinners

Always construct TUI spinners the same way, so async states look consistent across the app:

```go
spinner.New(
    spinner.WithSpinner(spinner.Dot),
    spinner.WithStyle(selectedStyle),
)
```

Use this for every spinner in `internal/tui/`. See `internal/tui/delete.go` for the canonical example.
