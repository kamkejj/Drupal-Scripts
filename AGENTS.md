# Agent Guidelines for Drupal-Scripts

## Build Commands
- Build binary: `go build -o dropkit` (creates binary in current directory)
- Build for macOS: `go build -o binary/macos/dropkit`
- Run: `./dropkit` or `go run .`
- Test: `go test ./...`
- Vet: `go vet ./...`

## CLI Design for Humans and AI Agents

- Keep one installation module and one behavior contract for every caller. Human prompts and machine-readable commands are adapters over the same `Plan`, `Apply`, and `Verify` workflow.
- Preserve `dropkit install` as the interactive human wizard when stdin is a terminal.
- Never prompt when stdin is not a terminal. Return a stable failure that tells automated callers which explicit command or argument is required.
- Keep planning and verification read-only. `plan` may inspect files and process state but must not install packages, start runtimes, create directories, or modify files.
- Require explicit authorization before `apply` performs network, host-level, privileged, or destructive effects. Validate every required authorization before the first mutation.
- Treat saved plans as declarative data. Plans may contain stable semantic step IDs and redacted previews, but never arbitrary executable commands, shell fragments, passwords, tokens, or other secrets.
- Schema-version and digest saved plans. Reject unsupported, altered, stale, ambiguous, or semantically invalid plans before mutation.
- In JSON mode, write exactly one final JSON document to stdout, including on failure. Write progress and subprocess output to stderr; support JSONL events for machine-readable streaming.
- Never write prompts, banners, ANSI color codes, progress text, or subprocess output to JSON stdout.
- Return stable failure codes, meaningful process exit codes, the failed semantic step, external exit status when available, retryability, and concrete recovery guidance. Do not require callers to parse prose to determine the failure.
- Make retry behavior explicit for every mutating step: safe, reconcile first, or manual recovery. Reinspect relevant state before applying a saved plan and fail closed when material state has drifted.
- Keep CLI help complete and discoverable at every command level. Document required inputs, defaults, side effects, approvals, output formats, exit behavior, and examples that can be copied by humans or agents.
- Prefer explicit flags and absolute normalized paths for automation. Defaults must be safe, deterministic, and identical between documented behavior and implementation.
- Preserve readable human output and a guided confirmation flow without weakening the machine contract or duplicating installation logic.
- Add contract tests whenever the CLI interface changes. Cover non-interactive behavior, clean JSON output, help discovery, approval preflight, plan validation, drift detection, failure results, and read-only verification.

## Code Style

### Language & Module
- Go 1.25.1
- Module: `dropkit`
- No external dependencies (stdlib only)

### Imports
- Standard library only
- Group imports: stdlib only, no third-party
- Use blank identifier for embed: `_ "embed"`

### Naming & Structure
- Use camelCase for private functions and PascalCase for exported domain types
- Name installation steps with stable semantic IDs such as `runtime.start` and `drupal.dependencies`
- Keep exact external commands private to the installation module

### Error Handling
- Return errors for critical failures, propagate up to `main()`
- Map typed failures to stable process exit codes at the CLI seam
- Preserve partial installation results when a step fails
- Keep machine-readable failure output separate from human diagnostics
- Continue with warnings only for explicitly non-critical failures

### Configuration
- Embed config files using `//go:embed` directive
- Store YAML configs in `config/` directory
- No comments in code

## Commit Messages
- Use Conventional Commits: `<type>[optional scope]: <description>`
- Common types: `feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `build:`
- Examples: `feat: add colima support`, `fix: correct docker check logic`, `docs: update installation steps`
- See https://www.conventionalcommits.org/en/v1.0.0/ for full specification
