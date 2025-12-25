# Makefile for Drupal 11 Installation Script

# Variables
BINARY_NAME = install-drupal
BINARY_DIR = binary/macos
SOURCE_FILE = main.go
INSTALL_SCRIPT = install.sh

# Default target
.PHONY: build install help clean

help:
	@echo "Drupal 11 Installation Script Makefile"
	@echo "======================================"
	@echo "Available targets:"
	@echo "  build    - Build the binary for macOS"
	@echo "  install  - Install the binary to /usr/local/bin"
	@echo "  clean    - Remove built binaries"
	@echo "  help     - Show this help message"

build:
	@echo "Building $(BINARY_NAME) for macOS..."
	go build -o $(BINARY_DIR)/$(BINARY_NAME) $(SOURCE_FILE)
	@echo "Build completed successfully!"

install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BINARY_DIR)/$(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "Installation completed! You can now run '$(BINARY_NAME)' from any directory."

clean:
	@echo "Cleaning up built binaries..."
	rm -f $(BINARY_DIR)/$(BINARY_NAME)
	@echo "Clean completed!"

# Convenience target for development
dev: build
	@echo "Running the installer in development mode..."
	go run $(SOURCE_FILE)