package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed config/environment_indicator.indicator.yml
var configIndicatorYML string

//go:embed config/environment_indicator.settings.yml
var configSettingsYML string

const (
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
	colorBlue   = "\033[0;34m"
	colorReset  = "\033[0m"
)

func printStatus(msg string) {
	fmt.Printf("%s[INFO]%s %s\n", colorBlue, colorReset, msg)
}

func printSuccess(msg string) {
	fmt.Printf("%s[SUCCESS]%s %s\n", colorGreen, colorReset, msg)
}

func printWarning(msg string) {
	fmt.Printf("%s[WARNING]%s %s\n", colorYellow, colorReset, msg)
}

func printError(msg string) {
	fmt.Printf("%s[ERROR]%s %s\n", colorRed, colorReset, msg)
}

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCommandOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func brewPackageInstalled(pkg string) bool {
	cmd := exec.Command("brew", "list", pkg)
	err := cmd.Run()
	return err == nil
}

func checkHomebrew() error {
	if !commandExists("brew") {
		printError("Homebrew is not installed. Please install Homebrew first:")
		fmt.Println("See https://docs.brew.sh/Installation for instructions.")
		return fmt.Errorf("homebrew not installed")
	}
	printSuccess("Homebrew is installed")
	return nil
}

func installDocker() bool {
	printStatus("Checking Docker Desktop installation...")
	if brewPackageInstalled("docker") {
		printSuccess("Docker Desktop is already installed")
		return true
	}
	printStatus("Docker Desktop not found. Installing via Homebrew...")
	if err := runCommand("brew", "install", "docker"); err != nil {
		printError("Failed to install Docker Desktop")
		return false
	}
	printSuccess("Docker Desktop installed. Please start Docker Desktop from Applications.")
	printWarning("You may need to restart your terminal after starting Docker Desktop.")
	return false
}

func checkDockerRunning() bool {
	cmd := exec.Command("docker", "info")
	err := cmd.Run()
	if err == nil {
		printSuccess("Docker is running")
		return true
	}
	printWarning("Docker is not running. Please start Docker Desktop.")
	return false
}

func installColima() bool {
	printStatus("Checking Colima installation...")
	if commandExists("colima") {
		printSuccess("Colima is already installed")
		return true
	}
	printStatus("Colima not found. Installing via Homebrew...")
	if err := runCommand("brew", "install", "colima"); err != nil {
		printError("Failed to install Colima")
		return false
	}
	printSuccess("Colima installed")
	return false
}

func checkColimaRunning() bool {
	cmd := exec.Command("colima", "status")
	err := cmd.Run()
	if err == nil {
		printSuccess("Colima is running")
		return true
	}
	printWarning("Colima is not running.")
	return false
}

func startColima() {
	printStatus("Starting Colima...")
	cmd := exec.Command("colima", "status")
	if cmd.Run() == nil {
		printSuccess("Colima is already running")
		return
	}
	if err := runCommand("colima", "start"); err != nil {
		printError("Failed to start Colima")
		return
	}
	printSuccess("Colima started")
}

func installDDEV() bool {
	printStatus("Checking DDEV installation...")
	if commandExists("ddev") {
		printSuccess("DDEV is already installed")
		return true
	}
	printStatus("DDEV not found. Installing via Homebrew...")
	if err := runCommand("brew", "install", "ddev/ddev/ddev"); err != nil {
		printError("Failed to install DDEV")
		return false
	}
	printSuccess("DDEV installed")
	return false
}

func checkDDEVVersion() {
	if commandExists("ddev") {
		output, _ := runCommandOutput("ddev", "version")
		printSuccess(fmt.Sprintf("DDEV version: %s", strings.TrimSpace(output)))
	}
}

func checkPrerequisites(dockerProvider string) {
	fmt.Println("==========================================")
	printStatus("Checking Prerequisites")
	fmt.Println("==========================================")

	if commandExists("brew") {
		printSuccess("✓ Homebrew is installed")
	} else {
		printError("✗ Homebrew is not installed")
	}

	if dockerProvider == "docker" {
		if brewPackageInstalled("docker") {
			printSuccess("✓ Docker Desktop is installed")
		} else {
			printWarning("✗ Docker Desktop is not installed")
		}
	} else {
		if commandExists("colima") {
			printSuccess("✓ Colima is installed")
		} else {
			printWarning("✗ Colima is not installed")
		}
	}

	if commandExists("ddev") {
		printSuccess("✓ DDEV is installed")
	} else {
		printWarning("✗ DDEV is not installed")
	}

	fmt.Println()
}

func initDrupalProject() (string, error) {
	printStatus("Initializing Drupal project...")

	reader := bufio.NewReader(os.Stdin)
	fmt.Println()
	fmt.Print("Enter your Drupal project name (e.g., 'my-drupal-site'): ")
	projectName, _ := reader.ReadString('\n')
	projectName = strings.TrimSpace(projectName)

	if projectName == "" {
		printError("Project name cannot be empty")
		return "", fmt.Errorf("empty project name")
	}

	projectName = strings.ToLower(projectName)
	projectName = strings.ReplaceAll(projectName, " ", "-")

	cwd, err := os.Getwd()
	if err != nil {
		printError("Failed to get current directory")
		return "", err
	}

	projectPath := filepath.Join(cwd, projectName)

	printStatus(fmt.Sprintf("Creating Drupal project: %s", projectName))
	if err := runCommand("composer", "create-project", "drupal/recommended-project:^11", projectPath); err != nil {
		printError("Failed to create Drupal project")
		return "", err
	}
	printSuccess(fmt.Sprintf("✓ Drupal project '%s' initialized", projectName))

	return projectPath, nil
}

func setupDrupalSettings(projectPath string) error {
	printStatus("Setting up Drupal settings...")

	configSyncPath := filepath.Join(projectPath, "config", "sync")
	if err := os.MkdirAll(configSyncPath, 0755); err != nil {
		printError("Failed to create config directory")
		return err
	}

	settingsPath := filepath.Join(projectPath, "web", "sites", "default", "settings.ddev.php")
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		printError("Failed to read settings.ddev.php")
		return err
	}

	newContent := strings.ReplaceAll(string(content), "sites/default/files/sync", "../config/sync")
	if err := os.WriteFile(settingsPath, []byte(newContent), 0644); err != nil {
		printError("Failed to write settings.ddev.php")
		return err
	}

	indicatorPath := filepath.Join(configSyncPath, "environment_indicator.indicator.yml")
	if err := os.WriteFile(indicatorPath, []byte(configIndicatorYML), 0644); err != nil {
		printError("Failed to write config files")
		return err
	}

	settingsConfigPath := filepath.Join(configSyncPath, "environment_indicator.settings.yml")
	if err := os.WriteFile(settingsConfigPath, []byte(configSettingsYML), 0644); err != nil {
		printError("Failed to write config files")
		return err
	}

	if _, err := os.Stat(configSyncPath); os.IsNotExist(err) {
		printError("Config directory not found at config/sync. Skipping config import.")
		return err
	}

	printSuccess("✓ Drupal settings setup completed")
	return nil
}

func initDDEVProject(projectPath string) error {
	printStatus("Initializing DDEV project...")

	ddevPath := filepath.Join(projectPath, ".ddev")
	if _, err := os.Stat(ddevPath); err == nil {
		printWarning(".ddev directory already exists. Skipping DDEV init.")
		return nil
	}

	cmd := exec.Command("ddev", "config", "--project-type=drupal11", "--docroot=web", "--create-docroot")
	cmd.Dir = projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		printError("Failed to initialize DDEV project")
		return err
	}
	printSuccess("DDEV project initialized")
	return nil
}

func startDDEV(projectPath string) error {
	printStatus("Starting DDEV...")
	cmd := exec.Command("ddev", "start")
	cmd.Dir = projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		printError("Failed to start DDEV")
		return err
	}
	printSuccess("DDEV started")
	return nil
}

func installDrupalDependencies(projectPath string) error {
	printStatus("Installing Drupal dependencies with Composer...")

	packages := [][]string{
		{"composer", "install"},
		{"composer", "require", "drupal/core-dev", "--dev", "-W"},
		{"composer", "require", "drush/drush"},
		{"composer", "require", "drupal/admin_toolbar"},
		{"composer", "require", "drupal/token"},
		{"composer", "require", "drupal/pathauto"},
		{"composer", "require", "drupal/config_ignore"},
		{"composer", "require", "drupal/config_split"},
		{"composer", "require", "drupal/devel"},
		{"composer", "require", "drupal/environment_indicator"},
		{"composer", "require", "drupal/better_exposed_filters"},
		{"composer", "require", "drupal/key"},
		{"composer", "require", "drupal/webprofiler"},
		{"composer", "require", "drupal/diff:^2.0@beta"},
		{"composer", "require", "drupal/ultimate_cron:^2.0@beta"},
	}

	for _, pkg := range packages {
		cmd := exec.Command("ddev", pkg...)
		cmd.Dir = projectPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			printError(fmt.Sprintf("Failed to install %v", pkg))
			return err
		}
	}

	printSuccess("✓ Drupal dependencies installed")
	return nil
}

func installDrupalSite(projectPath string) error {
	printStatus("Installing Drupal site...")

	cmd := exec.Command("ddev", "drush", "site:install", "standard", "--yes",
		"--account-name=admin", "--account-pass=admin", "--site-name=Super Awesome Site")
	cmd.Dir = projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		printError("Failed to install Drupal site")
		return err
	}

	printSuccess("Drupal site installed")
	printStatus("Admin credentials: username=admin, password=admin")
	return nil
}

func enableDrupalModules(projectPath string) error {
	printStatus("Enabling Drupal modules...")

	modules := []string{
		"admin_toolbar", "config_split", "devel", "environment_indicator",
		"environment_indicator_ui", "environment_indicator_toolbar",
		"token", "pathauto", "config_ignore", "better_exposed_filters",
		"key", "webprofiler", "diff", "ultimate_cron", "devel_generate",
	}

	args := append([]string{"drush", "en", "-y"}, modules...)
	cmd := exec.Command("ddev", args...)
	cmd.Dir = projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		printError("Failed to enable modules")
		return err
	}

	printSuccess("✓ Drupal modules enabled")
	return nil
}

func importDrupalConfig(projectPath string) error {
	printStatus("Importing Drupal config...")

	importPath := filepath.Join(projectPath, "config", "sync")
	if _, err := os.Stat(importPath); os.IsNotExist(err) {
		printWarning("Config directory not found at config/sync. Skipping config import.")
		return nil
	}

	cmd := exec.Command("ddev", "drush", "config:import", "--partial", "--yes")
	cmd.Dir = projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		printError("Failed to import config")
		return err
	}

	printSuccess("✓ Drupal config imported")
	return nil
}

func generateDrupalContent(projectPath string) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Do you want to generate content? (y/N): ")
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "y" {
		printSuccess("✓ Drupal content generation skipped")
		return nil
	}

	printStatus("Generating Drupal content...")

	cmd := exec.Command("ddev", "drush", "genu", "10", "--kill", "--roles=content_editor")
	cmd.Dir = projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		printError("Failed to generate users")
		return err
	}

	cmd = exec.Command("ddev", "drush", "genc", "25", "-y", "--kill", "--roles=content_editor", "--skip-fields=field_tags")
	cmd.Dir = projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		printError("Failed to generate content")
		return err
	}

	printSuccess("✓ Drupal content generated")
	return nil
}

func getSiteURL(projectPath string) string {
	printStatus("Getting site URL...")

	cmd := exec.Command("ddev", "describe", "--json-output")
	cmd.Dir = projectPath
	output, err := cmd.Output()
	if err != nil {
		printWarning("Could not determine site URL. Try running 'ddev describe'")
		return ""
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		printWarning("Could not parse site URL")
		return ""
	}

	if raw, ok := result["raw"].([]interface{}); ok && len(raw) > 0 {
		if project, ok := raw[0].(map[string]interface{}); ok {
			if httpsURL, ok := project["https_url"].(string); ok {
				printSuccess(fmt.Sprintf("Site URL: %s", httpsURL))
				return httpsURL
			}
		}
	}

	printWarning("Could not determine site URL. Try running 'ddev describe'")
	return ""
}

func displayFinalInstructions(siteURL string) {
	fmt.Println()
	fmt.Println("==========================================")
	printSuccess("Drupal 11 installation completed!")
	fmt.Println("==========================================")
	fmt.Println()
	fmt.Println("Next steps:")
	if siteURL != "" {
		fmt.Printf("1. Visit your site: %s\n", siteURL)
	} else {
		fmt.Println("1. Run 'ddev describe' to get your site URL")
	}
	fmt.Println("2. Login with: username=admin, password=admin")
	fmt.Println("3. Useful DDEV commands:")
	fmt.Println("   - ddev describe    # Show project info")
	fmt.Println("   - ddev drush       # Run Drush commands")
	fmt.Println("   - ddev ssh         # SSH into container")
	fmt.Println("   - ddev stop        # Stop the project")
	fmt.Println("   - ddev start       # Start the project")
	fmt.Println()
	fmt.Println("For more information, visit: https://ddev.readthedocs.io/")
}

func selectDockerProvider() string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Which Docker provider would you like to use?")
	fmt.Println("1. Docker Desktop")
	fmt.Println("2. Colima")
	fmt.Print("Enter your choice (1 or 2): ")

	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)

	if response == "2" {
		return "colima"
	}
	return "docker"
}

func main() {
	fmt.Println("==========================================")
	fmt.Println("Drupal 11 Installation Script")
	fmt.Println("==========================================")
	fmt.Println()

	dockerProvider := selectDockerProvider()
	fmt.Println()

	checkPrerequisites(dockerProvider)

	if err := checkHomebrew(); err != nil {
		os.Exit(1)
	}

	fmt.Println()

	if dockerProvider == "docker" {
		installDocker()
		if !checkDockerRunning() {
			printError("Please start Docker Desktop and run this script again.")
			os.Exit(1)
		}
	} else {
		installColima()
		if !checkColimaRunning() {
			startColima()
			if !checkColimaRunning() {
				printError("Failed to start Colima. Please start it manually and run this script again.")
				os.Exit(1)
			}
		}
	}

	if !installDDEV() {
		os.Exit(1)
	}

	checkDDEVVersion()

	projectPath, err := initDrupalProject()
	if err != nil {
		os.Exit(1)
	}

	if err := initDDEVProject(projectPath); err != nil {
		os.Exit(1)
	}

	if err := startDDEV(projectPath); err != nil {
		os.Exit(1)
	}

	if err := installDrupalDependencies(projectPath); err != nil {
		os.Exit(1)
	}

	if err := setupDrupalSettings(projectPath); err != nil {
		os.Exit(1)
	}

	if err := installDrupalSite(projectPath); err != nil {
		os.Exit(1)
	}

	if err := enableDrupalModules(projectPath); err != nil {
		os.Exit(1)
	}

	if err := importDrupalConfig(projectPath); err != nil {
		os.Exit(1)
	}

	if err := generateDrupalContent(projectPath); err != nil {
		printWarning("Content generation failed, but continuing...")
	}

	siteURL := getSiteURL(projectPath)
	displayFinalInstructions(siteURL)
}
