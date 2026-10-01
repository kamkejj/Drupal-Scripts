# Runtime installation configuration

- `cms.json` has `composer_packages` for packages to require after the CMS project is created and before the browser installer launches, and `enabled_modules` for modules enabled after the browser site installation completes. The package list starts empty; add the packages your project needs (for example, `"drupal/token"`). Keep modules already provided by Drupal core out of `composer_packages`.
- `commerce.json` lists Commerce Composer packages and modules.
- `drupal.json` lists Composer packages and enabled modules for Drupal 8–11 and 12, plus the static site-install profile and site name.

Dropkit reads these files from disk when the relevant command runs; edit them without rebuilding. Set `DROPKIT_MODULE_CONFIG_DIR` to the directory containing all three files when running from elsewhere (for example, after installing the binary in `/usr/local/bin`). By default, Dropkit looks beside the executable, at the repository root for binaries built at `binary/macos/dropkit`, or up from the working directory for a repository `go run .` invocation. If the files are missing or invalid, the command fails instead of using stale defaults. Admin credentials, version constraints, and command execution remain in Go.
