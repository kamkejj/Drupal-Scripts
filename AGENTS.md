# Agent Guidelines for Drupal-Scripts

## Build Commands
- Build binary: `go build -o install-drupal` (creates binary in current directory)
- Build for macOS: `go build -o binary/macos/install-drupal`
- Run: `./install-drupal` or `go run main.go`
- No test suite exists currently

## Code Style

### Language & Module
- Go 1.25.1
- Module: `drupal-installer`
- No external dependencies (stdlib only)

### Imports
- Standard library only (embed, bufio, encoding/json, fmt, os, os/exec, path/filepath, strings)
- Group imports: stdlib only, no third-party
- Use blank identifier for embed: `_ "embed"`

### Naming & Structure
- Use camelCase for functions: `checkHomebrew()`, `installDocker()`
- Use constants for color codes with clear names: `colorRed`, `colorGreen`
- Prefix print functions by type: `printStatus()`, `printSuccess()`, `printError()`, `printWarning()`

### Error Handling
- Return errors for critical failures, propagate up to `main()`
- Use `os.Exit(1)` in `main()` for unrecoverable errors
- Print descriptive error messages with `printError()` before returning
- Continue with warnings for non-critical issues using `printWarning()`

### Configuration
- Embed config files using `//go:embed` directive
- Store YAML configs in `config/` directory
- No comments in code

## Commit Messages
- Use Conventional Commits: `<type>[optional scope]: <description>`
- Common types: `feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `build:`
- Examples: `feat: add colima support`, `fix: correct docker check logic`, `docs: update installation steps`
- See https://www.conventionalcommits.org/en/v1.0.0/ for full specification
