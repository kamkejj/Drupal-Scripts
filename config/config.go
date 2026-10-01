package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Modules struct {
	ComposerPackages []string `json:"composer_packages"`
	EnabledModules   []string `json:"enabled_modules"`
}

type SiteInstall struct {
	Profile  string `json:"profile"`
	SiteName string `json:"site_name"`
}

type Drupal struct {
	Versions    map[string]Modules `json:"versions"`
	SiteInstall SiteInstall        `json:"site_install"`
}

func directory() string {
	if path := os.Getenv("DROPKIT_MODULE_CONFIG_DIR"); path != "" {
		return path
	}
	if executable, err := os.Executable(); err == nil {
		base := filepath.Dir(executable)
		if base == "" {
			return "module_config"
		}
		if filepath.Base(base) == "macos" && filepath.Base(filepath.Dir(base)) == "binary" {
			return filepath.Join(base, "..", "..", "module_config")
		}
		if _, err := os.Stat(filepath.Join(base, "module_config")); err == nil {
			return filepath.Join(base, "module_config")
		}
	}
	path, err := os.Getwd()
	if err != nil {
		return "module_config"
	}
	for {
		if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
			return filepath.Join(path, "module_config")
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	return "module_config"
}

func load(name string, destination any) error {
	data, err := os.ReadFile(filepath.Join(directory(), name+".json"))
	if err != nil {
		return fmt.Errorf("read %s configuration: %w", name, err)
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("decode %s configuration: %w", name, err)
	}
	return nil
}

func LoadModules(name string) (Modules, error) {
	if name != "cms" && name != "commerce" {
		return Modules{}, fmt.Errorf("unknown module configuration %q", name)
	}
	var modules Modules
	if err := load(name, &modules); err != nil {
		return Modules{}, err
	}
	if len(modules.EnabledModules) == 0 || (name == "commerce" && len(modules.ComposerPackages) == 0) {
		return Modules{}, fmt.Errorf("incomplete %s module configuration", name)
	}
	return modules, nil
}

func LoadDrupal() (Drupal, error) {
	var drupal Drupal
	if err := load("drupal", &drupal); err != nil {
		return Drupal{}, err
	}
	for _, version := range []string{"8-11", "12"} {
		modules := drupal.Versions[version]
		if len(modules.ComposerPackages) == 0 || len(modules.EnabledModules) == 0 {
			return Drupal{}, fmt.Errorf("incomplete Drupal %s module configuration", version)
		}
	}
	if drupal.SiteInstall.Profile == "" || drupal.SiteInstall.SiteName == "" {
		return Drupal{}, fmt.Errorf("incomplete Drupal site install configuration")
	}
	return drupal, nil
}
