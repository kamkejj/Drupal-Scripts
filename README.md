# Drupal 11 Installation Script

This script automates the installation of Drupal 11 with all necessary prerequisites on macOS, including essential development modules.

## Prerequisites

- macOS (tested on macOS 10.15+)
- Internet connection
- Admin/sudo privileges (for Homebrew installations)

## What the script does

1. **Checks for Homebrew** - Ensures Homebrew is installed (required for other installations)
2. **Installs Docker Desktop** - Uses Homebrew to install Docker Desktop
3. **Installs Colima** - Alternative Docker runtime for macOS
4. **Installs DDEV** - Drupal development environment
5. **Creates Drupal project** - Prompts for project name and creates Drupal 11 project
6. **Initializes DDEV project** - Sets up DDEV configuration for Drupal 11
7. **Installs Drupal dependencies** - Runs `composer install` and installs essential modules
8. **Installs Drupal site** - Creates a fresh Drupal 11 installation
9. **Enables development modules** - Automatically enables admin_toolbar, config_split, and devel

## Usage

1. **Navigate to your Drupal project directory:**
   ```bash
   cd /path/to/your/drupal/project
   ```

2. **Run the installation script:**
   ```bash
   ./install-drupal.sh
   ```

3. **Follow the prompts** - The script will guide you through the installation process

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
  - **Devel** - Development and debugging tools

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

### Docker not running
If Docker Desktop isn't running, the script will automatically start Colima as an alternative.

### Permission issues
Make sure you have admin privileges for Homebrew installations.

### Port conflicts
If you encounter port conflicts, DDEV will automatically find available ports.

### Project directory structure
The script creates a new directory with your project name and sets up Drupal inside it. Make sure you're in the parent directory where you want the project folder to be created.

## Manual Installation Steps

If you prefer to install manually:

1. **Install Homebrew** (if not already installed):
   ```bash
   /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
   ```

2. **Install Docker Desktop**:
   ```bash
   brew install --cask docker
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
