# Dropkit

Dropkit automates the installation of Drupal 8 through 12 with all necessary prerequisites on macOS, including essential development modules.

## Installation

### Option 1: Automated Installer (Recommended)

The project now includes an automated installer script that handles the installation process:

```bash
# Run the installer script
./install.sh

# Or with custom options
./install.sh --path ~/bin/dropkit --add-to-path
```

Run `./install.sh --help` for all available options.

### Option 2: Manual Installation

1. **Copy the binary to a location in your PATH:**
   ```bash
   sudo install -m 755 binary/macos/dropkit /usr/local/bin/dropkit
   ```

2. **Verify installation:**
   ```bash
   which dropkit
   ```

Now you can run `dropkit` from any directory.

### Option 3: Run Directly

Navigate to the parent directory where you want your Drupal project created and run:

```bash
 /path/to/Drupal-Scripts/binary/macos/dropkit install
```

## Usage

1. **Navigate to the parent directory where your Drupal project will be created:**
   ```bash
   cd ~/Projects
   ```

2. **Run the installer:**
   ```bash
   dropkit install
   ```
   
   Or if not in PATH:
   ```bash
   /path/to/Drupal-Scripts/binary/macos/dropkit install
   ```

3. **Complete the terminal wizard** - Choose a container runtime and Drupal version, name the project, and decide whether to generate sample content

4. **Review and authorize the plan** - The full-screen TUI shows every planned step and the network, host, or destructive effects requiring approval before anything is changed

To create a Drupal Commerce project, use the Commerce installer and choose Drupal 10 or 11:

```bash
dropkit commerce
```

It runs the same Drupal installation and configuration workflow, installs `drupal/commerce:^3.3`, and enables the Commerce modules needed for stores, products, orders, carts, checkout, pricing, and tax.

The interactive installer is built with [Bubble Tea](https://github.com/charmbracelet/bubbletea). Use the arrow keys or `Tab` to move, `Enter` to continue, and `Esc` to cancel. During installation, the TUI displays live semantic step events from the same installation module used by automated callers.

**Note:** The tool will create the new Drupal project as a subdirectory of your current working directory.

## Agent and automation usage

Dropkit provides a non-interactive plan, apply, and verify workflow. Planning inspects the host but does not install packages, start runtimes, create directories, or modify files.

Create and review a machine-readable plan:

```bash
dropkit install plan \
  --name my-drupal-site \
  --parent "$PWD" \
  --provider colima \
  --drupal-version 12 \
  --admin-password-env DRUPAL_ADMIN_PASSWORD \
  --output json > dropkit-plan.json
```

Apply the exact saved plan with explicit authorization for its effects:

```bash
export DRUPAL_ADMIN_PASSWORD='use-a-secret-value'

dropkit install apply \
  --plan dropkit-plan.json \
  --allow-network \
  --allow-host-changes \
  --output json \
  --events jsonl
```

Verify the resulting project without modifying it:

```bash
dropkit install verify \
  --plan dropkit-plan.json \
  --output json
```

The Commerce command provides the same automation contract. Replace `install` with `commerce` in the plan, apply, and verify commands, and use `--drupal-version 10` or `--drupal-version 11`. Commerce verification checks both the package and enabled modules:

```bash
dropkit commerce plan \
  --name my-store \
  --parent "$PWD" \
  --provider colima \
  --drupal-version 11 \
  --output json > dropkit-commerce-plan.json
```

In JSON mode, stdout contains one final JSON document. Progress and subprocess output are written to stderr; `--events jsonl` makes every stderr event machine-readable. A non-terminal `dropkit install` invocation never prompts and returns an error directing the caller to create a plan.

`--drupal-version` is required for automated plans and accepts a major version from 8 through 12. The selected version is stored in the plan and controls both the Composer project constraint and DDEV project type. Until Drupal 12 has a stable release, selecting version 12 installs its development branch.

Plans contain stable semantic step IDs, effect classifications, retry guidance, and a digest. They do not contain executable shell commands or password values. Applying a plan fails before mutation when required authorization is missing, the plan has been altered, or inspected host state has changed.

Effect authorization flags are:

- `--allow-network` for downloads
- `--allow-host-changes` for host package installation and runtime changes
- `--allow-destructive` for operations such as replacing generated sample content

If a plan reports a blocker, resolve it and create a new plan. Docker Desktop must already be running; Colima can be started by an explicitly authorized plan.

## Prerequisites

- macOS (tested on macOS 10.15+)
- Internet connection
- Admin/sudo privileges (for Homebrew installations)

## What the installer does

1. **Prompts for Docker provider** - Choose between Docker Desktop or Colima
2. **Prompts for Drupal version** - Choose a major version from Drupal 8 through 12
3. **Checks prerequisites** - Displays status of required tools based on your Docker provider choice
4. **Checks for Homebrew** - Ensures Homebrew is installed (required for other installations)
5. **Installs and starts Docker provider** - Installs and ensures your chosen provider (Docker Desktop or Colima) is running
6. **Installs DDEV** - Drupal development environment
7. **Creates Drupal project** - Prompts for project name and creates the selected Drupal version in the current directory
8. **Initializes DDEV project** - Sets the DDEV project type to the selected Drupal version
9. **Starts DDEV** - Launches the development environment
10. **Installs Drupal dependencies** - Runs `composer install` and installs essential modules via DDEV
11. **Configures Drupal settings** - Sets up config sync directory and environment indicator configs
12. **Installs Drupal site** - Creates a fresh Drupal installation with admin credentials
13. **Enables development modules** - Automatically enables admin_toolbar, config_split, devel, and more
14. **Imports configuration** - Imports environment indicator and other configs
15. **Generates content (optional)** - Optionally generates sample users and content for testing

The `commerce` command performs all of these steps, runs `ddev composer require 'drupal/commerce:^3.3'`, and enables `commerce`, `commerce_cart`, `commerce_checkout`, `commerce_order`, `commerce_store`, `commerce_price`, and `commerce_tax`.

## What gets installed

### Required Tools (via Homebrew)
- **Docker Desktop** - Container runtime
- **Colima** - Lightweight Docker alternative for macOS
- **DDEV** - Drupal development environment

### Drupal Setup
- The selected Drupal core version and dependencies
- Standard Drupal installation
- Admin account: `admin` / `admin`
- **Development modules automatically installed and enabled:**
  - **Admin Toolbar** - Enhanced admin interface
  - **Config Split** - Configuration management for different environments
  - **Config Ignore** - Ignore specific configuration during imports
  - **Devel** - Development and debugging tools
  - **Devel Generate** - Generate test content
  - **Webprofiler** - Performance profiling
  - **Environment Indicator** - Visual environment indicators
  - **Token** - Token replacement system
  - **Pathauto** - Automatic URL alias generation
  - **Better Exposed Filters** - Enhanced Views filters
  - **Key** - Key management system
  - **Diff** - Configuration comparison tools
  - **Ultimate Cron** - Advanced cron management

Drupal 12 currently installs the Drupal 12-compatible subset of this bundle: Token, Devel, Devel Generate, and Environment Indicator. The remaining contributed modules are omitted until their published Composer metadata supports Drupal 12.

## After Installation

Once the script completes, you'll have:

- A fully functional site using the selected Drupal version in a project directory
- DDEV development environment running
- Access to your site via the provided URL
- Admin access with username: `admin`, password: `admin`
- Essential development modules ready to use

## Useful DDEV Commands

```bash
# Show project information
ddev describe

# Access the site
ddev launch

# Run Drush commands
ddev drush status
ddev drush cr  # Clear cache

# SSH into the container
ddev ssh

# Stop the project
ddev stop

# Start the project
ddev start

# View logs
ddev logs
```

## Troubleshooting

### Docker provider selection
The script will prompt you to choose between Docker Desktop and Colima at startup. Choose the provider you prefer or that better suits your system resources.

### Docker not running
If your chosen Docker provider isn't running:
- **Docker Desktop**: You'll be prompted to start it manually
- **Colima**: The script will attempt to start it automatically

### Permission issues
Make sure you have admin privileges for Homebrew installations.

### Port conflicts
If you encounter port conflicts, DDEV will automatically find available ports.

### Project directory structure
The installer creates a new directory with your project name as a child of your current working directory. Make sure you're in the parent directory where you want the project folder to be created.

### Binary location
The Go binary embeds all configuration files, so you can place it anywhere on your system. For convenience, add it to your PATH (see Installation section above).

## Building from Source

If you want to build the binary yourself:

```bash
cd /path/to/Drupal-Scripts
go build -o dropkit
```

This will create the `dropkit` binary in the current directory.

### Using Makefile

For easier building and installation, you can use the provided Makefile:

```bash
# Build the binary
make build

# Install the binary to /usr/local/bin
make install

# Show all available commands
make help
```

## Manual Drupal Installation Steps

If you prefer to install Drupal manually without using this tool:

1. **Install Homebrew** (if not already installed), see https://docs.brew.sh/Installation
   ```bash
   /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
   ```

2. **Install Docker Desktop**:
   ```bash
   brew install docker
   ```

3. **Install Colima**:
   ```bash
   brew install colima
   colima start
   ```

4. **Install DDEV**:
   ```bash
   brew install ddev/ddev/ddev
   ```

5. **Create Drupal project**:
   ```bash
   composer create-project drupal/recommended-project:^11 my-drupal-site
   cd my-drupal-site
   ```

   Replace `11` with the Drupal major version you want to install.

6. **Initialize DDEV project**:
   ```bash
   ddev config --project-type=drupal11 --docroot=web --create-docroot
   ddev start
   ```

   The DDEV project type must use the same Drupal major version selected for Composer.

7. **Install Drupal dependencies and modules**:
   ```bash
   ddev composer install
   ddev composer require drush/drush
   ddev composer require drupal/admin_toolbar
   ddev composer require drupal/config_split
   ddev composer require drupal/devel
   ```

8. **Install Drupal site**:
   ```bash
   ddev drush site:install standard --yes --account-name=admin --account-pass=admin --site-name="Drupal 11 Site"
   ```

9. **Enable development modules**:
   ```bash
   ddev drush en admin_toolbar config_split devel
   ```

## Support

For issues with:
- **DDEV**: https://ddev.readthedocs.io/
- **Docker**: https://docs.docker.com/
- **Drupal**: https://www.drupal.org/support
