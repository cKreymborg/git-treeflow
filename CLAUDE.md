# git-treeflow project notes

## UI conventions

### Spinners

All TUI spinners share one style so async states look consistent across the app.
Construct them via the `newDefaultSpinner()` helper in `internal/tui/helpers.go`:

```go
m.spinner = newDefaultSpinner()
```

Do not call `spinner.New(...)` directly in new TUI code — extend the helper instead
if a new style is ever needed.
