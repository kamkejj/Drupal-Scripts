package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadsUserConfiguration(t *testing.T) {
	path := t.TempDir()
	t.Setenv("DROPKIT_MODULE_CONFIG_DIR", path)
	files := map[string]string{
		"cms.json":      `{"composer_packages":["drupal/cms-extra"],"enabled_modules":["toolbar","config"]}`,
		"commerce.json": `{"composer_packages":["drupal/commerce:^3"],"enabled_modules":["commerce"]}`,
		"drupal.json":   `{"versions":{"8-11":{"composer_packages":["drupal/one"],"enabled_modules":["one"]},"12":{"composer_packages":["drupal/two","drupal/three"],"enabled_modules":["two"]}},"site_install":{"profile":"minimal","site_name":"Example"}}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(path, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cms, err := LoadModules("cms")
	if err != nil || !slices.Equal(cms.EnabledModules, []string{"toolbar", "config"}) || !slices.Equal(cms.ComposerPackages, []string{"drupal/cms-extra"}) {
		t.Fatalf("CMS configuration = %+v, %v", cms, err)
	}
	commerce, err := LoadModules("commerce")
	if err != nil || !slices.Equal(commerce.ComposerPackages, []string{"drupal/commerce:^3"}) || !slices.Equal(commerce.EnabledModules, []string{"commerce"}) {
		t.Fatalf("Commerce configuration = %+v, %v", commerce, err)
	}
	drupal, err := LoadDrupal()
	if err != nil || !slices.Equal(drupal.Versions["8-11"].ComposerPackages, []string{"drupal/one"}) || !slices.Equal(drupal.Versions["12"].ComposerPackages, []string{"drupal/two", "drupal/three"}) || !slices.Equal(drupal.Versions["12"].EnabledModules, []string{"two"}) || drupal.SiteInstall.Profile != "minimal" || drupal.SiteInstall.SiteName != "Example" {
		t.Fatalf("Drupal configuration = %+v, %v", drupal, err)
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
