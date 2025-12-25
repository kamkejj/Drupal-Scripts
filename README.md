# Drupal 11 Installation Script (MacOS)

This tool automates the installation of Drupal 11 with all necessary prerequisites on macOS, including essential development modules.

## Installation

### Option 1: Automated Installer (Recommended)

The project now includes an automated installer script that handles the installation process:

```bash
# Run the installer script
./install.sh

# Or with custom options
./install.sh --path ~/bin/install-drupal --add-to-path
```

Run `./install.sh --help` for all available options.

### Option 2: Manual Installation

1. **Copy the binary to a location in your PATH:**
   ```bash
   sudo cp binary/macos/install-drupal /usr/local/bin/
   sudo chmod +x /usr/local/bin/install-drupal
   ```

2. **Or create a symlink:**
   ```bash
   sudo ln -s /path/to/Drupal-Scripts/binary/macos/install-drupal /usr/local/bin/install-drupal
   ```

3. **Verify installation:**
   ```bash
   which install-drupal
   ```

Now you can run `install-drupal` from any directory.

### Option 3: Run Directly

Navigate to the parent directory where you want your Drupal project created and run:

```bash
/path/to/Drupal-Scripts/binary/macos/install-drupal
```

## Usage

1. **Navigate to the parent directory where your Drupal project will be created:**
   ```bash
   cd ~/Projects
   ```

2. **Run the installer:**
   ```bash
   install-drupal
   ```
   
   Or if not in PATH:
   ```bash
   /path/to/Drupal-Scripts/install-drupal
   ```

3. **Follow the prompts** - The script will guide you through the installation process

**Note:** The tool will create the new Drupal project as a subdirectory of your current working directory.

## Prerequisites

- macOS (tested on macOS 10.15+)
- Internet connection
- Admin/sudo privileges (for Homebrew installations)

## What the installer does

1. **Prompts for Docker provider** - Choose between Docker Desktop or Colima
2. **Checks prerequisites** - Displays status of required tools based on your Docker provider choice
3. **Checks for Homebrew** - Ensures Homebrew is installed (required for other installations)
4. **Installs and starts Docker provider** - Installs and ensures your chosen provider (Docker Desktop or Colima) is running
5. **Installs DDEV** - Drupal development environment
6. **Creates Drupal project** - Prompts for project name and creates Drupal 11 project in current directory
7. **Initializes DDEV project** - Sets up DDEV configuration for Drupal 11
8. **Starts DDEV** - Launches the development environment
9. **Installs Drupal dependencies** - Runs `composer install` and installs essential modules via DDEV
10. **Configures Drupal settings** - Sets up config sync directory and environment indicator configs
11. **Installs Drupal site** - Creates a fresh Drupal 11 installation with admin credentials
12. **Enables development modules** - Automatically enables admin_toolbar, config_split, devel, and more
13. **Imports configuration** - Imports environment indicator and other configs
14. **Generates content (optional)** - Optionally generates sample users and content for testing

## What gets installed

### Required Tools (via Homebrew)
- **Docker Desktop** - Container runtime
- **Colima** - Lightweight Docker alternative for macOS
- **DDEV** - Drupal development environment

### Drupal Setup
- Drupal 11 core and dependencies
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

## After Installation

Once the script completes, you'll have:

- A fully functional Drupal 11 site in a project directory
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
go build -o install-drupal
```

This will create the `install-drupal` binary in the current directory.

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

6. **Initialize DDEV project**:
   ```bash
   ddev config --project-type=drupal11 --docroot=web --create-docroot
   ddev start
   ```

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
