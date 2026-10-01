# Installation architecture review and plan

## Current structure

`main.go` dispatches three product commands. `internal/command/{drupal,commerce,cms}` select product policy and adapt arguments to `internal/installer`. `internal/installer/cli.go` and `tui.go` adapt machine and human interaction to the same `InstallationModule` interface: `Plan`, `Apply`, and `Verify`. `internal/installer/module.go` owns host inspection, declarative steps, approvals, execution, and verification. `config` loads editable product configuration from `module_config/`.

The installation module is appropriately deep: callers need not know external commands or the differences between CMS browser setup and Drupal site installation. The runner and filesystem interfaces are useful internal seams, with real and test adapters. Splitting by product or adding pass-through interfaces would scatter the installation contract.

## Risk and implementation order

1. **Bind configuration to saved plans (implemented).** Previously product packages and Drupal commands could change between plan creation and application without being represented in the saved plan. Schema 8 includes a deterministic hash of effective product policy and Drupal configuration. `Apply` checks it after approval preflight and before mutation; `Verify` checks it read-only. Drupal execution uses the validated configuration snapshot rather than reloading policy midway through application. Old plans must be recreated.
2. **Consolidate policy loading (future).** The command adapters currently load CMS/Commerce modules while the installer loads Drupal configuration. A single read-only policy loader can produce an immutable product-specific snapshot for all three commands. Preserve `InstallationModule` as the external seam; avoid exposing configuration-loading internals to CLI and TUI callers. Validate product inputs and return structured errors, including JSON-mode failures, at the CLI seam.
3. **Localize workflows (future).** After policy loading is unified, separate step construction and execution by workflow *inside* the installer, keeping plan validation, approval preflight, drift checks, and result handling shared. Do not introduce a public workflow interface until there are real alternate adapters. Preserve stable semantic step IDs and schema/digest checks.
4. **Test through the shared interface (ongoing).** Cover all three product plans and drift, read-only planning/verification, approval preflight, command effects, retries, and clean JSON output. Replace redundant tests of implementation details as interface-level coverage grows.

Do not combine the future phases with this schema change: each changes a different seam and requires its own contract review.
