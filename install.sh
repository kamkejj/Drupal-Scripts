#!/bin/bash

# Drupal 11 Installation Script Installer
# This script installs the Drupal 11 installer binary to the user's system

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print functions
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

# Check if running on macOS
if [[ "$OSTYPE" != "darwin"* ]]; then
    print_error "This installer is designed for macOS only."
    exit 1
fi

# Default installation path
DEFAULT_INSTALL_PATH="/usr/local/bin/install-drupal"
DEFAULT_BINARY_PATH="./binary/macos/install-drupal"

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to check if user has sudo access
check_sudo_access() {
    if sudo -v 2>/dev/null; then
        return 0
    else
        return 1
    fi
}

# Function to install the binary
install_binary() {
    local install_path="$1"
    local binary_path="$2"
    
    # Check if binary exists
    if [ ! -f "$binary_path" ]; then
        print_error "Binary not found at $binary_path. Please build it first with 'go build -o binary/macos/install-drupal'"
        exit 1
    fi
    
    # Check permissions
    if [ ! -x "$binary_path" ]; then
        print_warning "Binary is not executable. Making it executable..."
        chmod +x "$binary_path"
    fi
    
    # Try to copy with sudo if needed
    if [[ $install_path == /usr/local/bin* ]] || [[ $install_path == /usr/bin* ]]; then
        if ! check_sudo_access; then
            print_error "This script requires sudo access to install to $install_path"
            print_warning "Either run with sudo or choose a different installation path in your home directory"
            exit 1
        fi
        
        print_status "Installing to $install_path with sudo..."
        sudo cp "$binary_path" "$install_path"
        sudo chmod +x "$install_path"
    else
        # Installing to user directory
        print_status "Installing to $install_path..."
        cp "$binary_path" "$install_path"
        chmod +x "$install_path"
    fi
    
    if [ $? -eq 0 ]; then
        print_success "Successfully installed Drupal installer to $install_path"
        return 0
    else
        print_error "Failed to install binary to $install_path"
        return 1
    fi
}

# Function to add to PATH if not already there
add_to_path() {
    local install_dir=$(dirname "$1")
    
    # Check if already in PATH
    if [[ ":$PATH:" == *":$install_dir:"* ]]; then
        print_status "Installation directory is already in PATH"
        return 0
    fi
    
    # Detect shell and config file
    local shell_name=$(basename "$SHELL")
    local config_file=""
    
    case "$shell_name" in
        bash)
            config_file="$HOME/.bash_profile"
            ;;
        zsh)
            config_file="$HOME/.zshrc"
            ;;
        *)
            config_file="$HOME/.profile"
            ;;
    esac
    
    # Add to PATH if not already there
    if [ -f "$config_file" ]; then
        if ! grep -q "export PATH=.*$install_dir" "$config_file"; then
            echo "" >> "$config_file"
            echo "# Added by Drupal installer" >> "$config_file"
            echo "export PATH=\"$install_dir:\$PATH\"" >> "$config_file"
            print_success "Added $install_dir to PATH in $config_file"
            print_warning "Please restart your terminal or run 'source $config_file' to apply changes"
        else
            print_status "Installation directory is already in PATH in $config_file"
        fi
    else
        print_warning "Could not find shell configuration file ($config_file). You may need to manually add $install_dir to your PATH."
    fi
}

# Main installation function
main() {
    print_status "Drupal 11 Installation Script Installer"
    echo "=========================================="
    echo
    
    # Parse command line arguments
    local install_path="$DEFAULT_INSTALL_PATH"
    local binary_path="$DEFAULT_BINARY_PATH"
    local add_to_path_flag=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            -p|--path)
                install_path="$2"
                shift 2
                ;;
            -b|--binary)
                binary_path="$2"
                shift 2
                ;;
            --add-to-path)
                add_to_path_flag=true
                shift
                ;;
            -h|--help)
                echo "Usage: $0 [OPTIONS]"
                echo "Options:"
                echo "  -p, --path PATH     Installation path (default: /usr/local/bin/install-drupal)"
                echo "  -b, --binary PATH   Binary path (default: ./binary/macos/install-drupal)"
                echo "  --add-to-path       Add installation directory to PATH if not already there"
                echo "  -h, --help          Show this help message"
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done
    
    print_status "Installing Drupal 11 installer..."
    
    # Create installation directory if it doesn't exist
    local install_dir=$(dirname "$install_path")
    if [ ! -d "$install_dir" ]; then
        if check_sudo_access; then
            print_status "Creating installation directory $install_dir..."
            sudo mkdir -p "$install_dir"
        else
            print_error "Cannot create installation directory $install_dir - no sudo access"
            exit 1
        fi
    fi
    
    # Install the binary
    if install_binary "$install_path" "$binary_path"; then
        print_success "Drupal 11 installer installed successfully!"
        
        # Add to PATH if requested
        if [ "$add_to_path_flag" = true ]; then
            add_to_path "$install_path"
        fi
        
        echo
        print_status "Installation completed successfully!"
        print_success "You can now run 'install-drupal' from any directory."
        echo "Navigate to the directory where you want to create your Drupal project and run:"
        echo "  install-drupal"
    else
        print_error "Installation failed!"
        exit 1
    fi
}

# Run main function with all arguments
main "$@"