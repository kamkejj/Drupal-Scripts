package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestEmbeddedConfigurations(t *testing.T) {
	cms, err := LoadModules("cms")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cms.EnabledModules, []string{"config", "inline_form_errors", "settings_tray", "toolbar", "syslog", "workspaces", "workspaces_ui"}) {
		t.Fatalf("CMS modules = %v", cms.EnabledModules)
	}

	commerce, err := LoadModules("commerce")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(commerce.ComposerPackages, []string{"drupal/commerce:^3.3"}) || len(commerce.EnabledModules) != 9 {
		t.Fatalf("Commerce configuration = %+v", commerce)
	}

	drupal, err := LoadDrupal()
	if err != nil {
		t.Fatal(err)
	}
	if len(drupal.Versions["8-11"].ComposerPackages) != 13 || len(drupal.Versions["8-11"].EnabledModules) != 16 || len(drupal.Versions["12"].ComposerPackages) != 4 || len(drupal.Versions["12"].EnabledModules) != 6 {
		t.Fatalf("Drupal version configuration = %+v", drupal.Versions)
	}
	if drupal.SiteInstall.Profile != "standard" || drupal.SiteInstall.SiteName != "Super Awesome Site" {
		t.Fatalf("site install = %+v", drupal.SiteInstall)
	}
}

func TestReadsConfigurationAgainAtRuntime(t *testing.T) {
	path := t.TempDir()
	t.Setenv("DROPKIT_MODULE_CONFIG_DIR", path)
	file := filepath.Join(path, "cms.json")
	if err := os.WriteFile(file, []byte(`{"enabled_modules":["first"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	first, err := LoadModules("cms")
	if err != nil || !slices.Equal(first.EnabledModules, []string{"first"}) {
		t.Fatalf("first load = %+v, %v", first, err)
	}
	if err := os.WriteFile(file, []byte(`{"enabled_modules":["second"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	second, err := LoadModules("cms")
	if err != nil || !slices.Equal(second.EnabledModules, []string{"second"}) {
		t.Fatalf("second load = %+v, %v", second, err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadModules("cms"); err == nil || !strings.Contains(err.Error(), "read cms configuration") {
		t.Fatalf("missing file error = %v", err)
	}
}

func TestUnknownModuleConfiguration(t *testing.T) {
	if _, err := LoadModules("unknown"); err == nil {
		t.Fatal("expected unknown configuration error")
	}
}
