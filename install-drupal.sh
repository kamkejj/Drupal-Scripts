#!/bin/bash

# Drupal 11 Installation Script
# This script checks for prerequisites (Docker, Colima, DDEV) and installs them if needed,
# then sets up Drupal 11 with DDEV

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to check if Homebrew is installed
check_homebrew() {
    if ! command_exists brew; then
        print_error "Homebrew is not installed. Please install Homebrew first:"
        echo "See https://docs.brew.sh/Installation for instructions."
        exit 1
    fi
    print_success "Homebrew is installed"
}

# Function to check and install Docker Desktop
install_docker() {
    print_status "Checking Docker Desktop installation..."
    if brew list docker >/dev/null 2>&1; then
        print_success "Docker Desktop is already installed"
        return 0
    else
        print_status "Docker Desktop not found. Installing via Homebrew..."
        brew install docker
        print_success "Docker Desktop installed. Please start Docker Desktop from Applications."
        print_warning "You may need to restart your terminal after starting Docker Desktop."
        return 1
    fi
}

# Function to check if Docker is running
check_docker_running() {
    if docker info >/dev/null 2>&1; then
        print_success "Docker is running"
        return 0
    else
        print_warning "Docker is not running. Please start Docker Desktop."
        return 1
    fi
}

# Function to check and install Colima
install_colima() {
    print_status "Checking Colima installation..."
    if command_exists colima; then
        print_success "Colima is already installed"
        return 0
    else
        print_status "Colima not found. Installing via Homebrew..."
        brew install colima
        print_success "Colima installed"
        return 1
    fi
}

# Function to start Colima
start_colima() {
    print_status "Starting Colima..."
    if colima status >/dev/null 2>&1; then
        print_success "Colima is already running"
    else
        colima start
        print_success "Colima started"
    fi
}

# Function to check and install DDEV
install_ddev() {
    print_status "Checking DDEV installation..."
    if command_exists ddev; then
        print_success "DDEV is already installed"
        return 0
    else
        print_status "DDEV not found. Installing via Homebrew..."
        brew install ddev/ddev/ddev
        print_success "DDEV installed"
        return 1
    fi
}

# Function to check DDEV version
check_ddev_version() {
    if command_exists ddev; then
        DDEV_VERSION=$(ddev version)
        print_success "DDEV version: $DDEV_VERSION"
    fi
}

# Function to check all prerequisites and show status
check_prerequisites() {
    echo "=========================================="
    print_status "Checking Prerequisites"
    echo "=========================================="
    
    # Check Homebrew
    if command_exists brew; then
        print_success "✓ Homebrew is installed"
    else
        print_error "✗ Homebrew is not installed"
        return 1
    fi
    
    # Check Docker Desktop
    if brew list docker >/dev/null 2>&1; then
        print_success "✓ Docker Desktop is installed"
    else
        print_warning "✗ Docker Desktop is not installed"
    fi
    
    # Check Colima
    if command_exists colima; then
        print_success "✓ Colima is installed"
    else
        print_warning "✗ Colima is not installed"
    fi
    
    # Check DDEV
    if command_exists ddev; then
        print_success "✓ DDEV is installed"
    else
        print_warning "✗ DDEV is not installed"
    fi
    
    echo ""
}

# Function to initialize DDEV project
init_ddev_project() {
    print_status "Initializing DDEV project..."
    # Check if .ddev directory exists
    if [ -d ".ddev" ]; then
        print_warning ".ddev directory already exists. Skipping DDEV init."
        return 0
    fi
    
    # Initialize DDEV with Drupal 11 configuration
    ddev config --project-type=drupal11 --docroot=web --create-docroot
    print_success "DDEV project initialized"
}

# Function to start DDEV
start_ddev() {
    print_status "Starting DDEV..."
    ddev start
    print_success "DDEV started"
}

# Function to initialize Drupal project
init_drupal_project() {
    print_status "Initializing Drupal project..."
    
    # Ask user for project name
    echo ""
    read -p "Enter your Drupal project name (e.g., 'my-drupal-site'): " PROJECT_NAME
    
    # Validate project name (basic validation)
    if [ -z "$PROJECT_NAME" ]; then
        print_error "Project name cannot be empty"
        exit 1
    fi
    
    # Remove any spaces and convert to lowercase for consistency
    PROJECT_NAME=$(echo "$PROJECT_NAME" | tr '[:upper:]' '[:lower:]' | tr ' ' '-')
    
    print_status "Creating Drupal project: $PROJECT_NAME"
    composer create-project drupal/recommended-project:^11 "$PROJECT_NAME"
    print_success "✓ Drupal project '$PROJECT_NAME' initialized"
}

setup_drupal_settings() {
    print_status "Setting up Drupal settings..."
    # create config directory
    mkdir -p config/sync
    # change settings.ddev.php to use config/sync instead of 'sites/default/files/sync'
    sed -i '' 's/sites\/default\/files\/sync/..\/config\/sync/' "web/sites/default/settings.ddev.php"
    # copy ../config to web/sites/default/config/sync
    cp -r ../Drupal-Scripts/config/ config/sync
    # check that config/sync exists
    if [ ! -d "config/sync" ]; then
        print_error "Config directory not found at config/sync. Skipping config import."
        exit 1
    fi
    print_success "✓ Drupal settings setup completed"
}

# Function to install Drupal dependencies
install_drupal_dependencies() {
    print_status "Installing Drupal dependencies with Composer..."
    ddev composer install
    ddev composer require drush/drush
    ddev composer require drupal/admin_toolbar
    ddev composer require drupal/token
    ddev composer require drupal/pathauto
    ddev composer require drupal/config_ignore
    ddev composer require drupal/config_split
    ddev composer require drupal/devel
    ddev composer require drupal/environment_indicator
    ddev composer require drupal/better_exposed_filters
    ddev composer require drupal/key
    ddev composer require drupal/webprofiler
    ddev composer require 'drupal/diff:^2.0@beta'
    ddev composer require 'drupal/ultimate_cron:^2.0@beta'
    print_success "✓ Drupal dependencies installed"
}

# Function to install Drupal site
install_drupal_site() {
    print_status "Installing Drupal site..."
    
    # Install Drupal with standard profile
    ddev drush site:install standard --yes --account-name=admin --account-pass=admin --site-name="Drupal 11 Site"

    print_success "Drupal site installed"
    print_status "Admin credentials: username=admin, password=admin"
}

enable_drupal_modules() {
    print_status "Enabling Drupal modules..."
    ddev drush en admin_toolbar config_split devel environment_indicator environment_indicator_ui environment_indicator_toolbar -y
    ddev drush en token pathauto config_ignore better_exposed_filters key webprofiler diff ultimate_cron devel_generate -y
    print_success "✓Drupal modules enabled"
}

import_drupal_config() {
    print_status "Importing Drupal config..."
    import_path="config/sync"

    # Check if config directory exists
    if [ ! -d "$import_path" ]; then
        print_warning "Config directory not found at $import_path. Skipping config import."
        return 0
    fi
    
    ddev drush config:import --partial --yes
    print_success "✓ Drupal config imported"
}

generate_drupal_content() {
    # ask the user if they want to generate content, default to N
    GENERATE_CONTENT="N"
    read -p "Do you want to generate content? (y/N): " GENERATE_CONTENT
    if [ "$GENERATE_CONTENT" != "y" ]; then
        print_success "✓ Drupal content generation skipped"
        return 0
    fi

    print_status "Generating Drupal content..."
    ddev drush genu 10 --kill --roles=content_editor
    ddev drush genc 25 -y --kill --roles=content_editor --skip-fields=field_tags
    print_success "✓ Drupal content generated"
}

# Function to get site URL
get_site_url() {
    print_status "Getting site URL..."
    SITE_URL=$(ddev describe --json-output | grep -o '"https_url":"[^"]*"' | cut -d'"' -f4)
    if [ -n "$SITE_URL" ]; then
        print_success "Site URL: $SITE_URL"
    else
        print_warning "Could not determine site URL. Try running 'ddev describe'"
    fi
}

# Function to display final instructions
display_final_instructions() {
    echo ""
    echo "=========================================="
    print_success "Drupal 11 installation completed!"
    echo "=========================================="
    echo ""
    echo "Next steps:"
    echo "1. Visit your site: $SITE_URL"
    echo "2. Login with: username=admin, password=admin"
    echo "3. Useful DDEV commands:"
    echo "   - ddev describe    # Show project info"
    echo "   - ddev drush       # Run Drush commands"
    echo "   - ddev ssh         # SSH into container"
    echo "   - ddev stop        # Stop the project"
    echo "   - ddev start       # Start the project"
    echo ""
    echo "For more information, visit: https://ddev.readthedocs.io/"
}

# Main installation process
main() {
    echo "=========================================="
    echo "Drupal 11 Installation Script"
    echo "=========================================="
    echo ""
    
    # Check all prerequisites first
    check_prerequisites
    
    # Check Homebrew (required for other installations)
    check_homebrew
    
    echo ""
    
    # Check if Docker is running, if not, try Colima
    if ! check_docker_running; then
        print_status "Docker is not running. Starting Colima as alternative..."
        start_colima
    fi
    
    # Check DDEV version
    check_ddev_version
    
    # Initialize Drupal project
    init_drupal_project

    # Navigate to project directory
    cd "$PROJECT_NAME"

    # Initialize and start DDEV
    init_ddev_project
    start_ddev
    
    # Install Drupal
    install_drupal_dependencies
    setup_drupal_settings
    install_drupal_site
    enable_drupal_modules
    import_drupal_config
    # Generate Drupal content if user wants to
    generate_drupal_content
    
    # Get site URL and display instructions
    get_site_url
    display_final_instructions

    cd ..
}

# Run main function
main "$@"
