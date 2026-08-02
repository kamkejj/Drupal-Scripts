package commerce

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestNonInteractiveCommandNeverPrompts(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(nil, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "interactive commerce requires a terminal") {
		t.Fatalf("Run() exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
	}
}

func TestSubcommandHelpIsDiscoverable(t *testing.T) {
	for _, test := range []struct {
		command string
		text    string
	}{
		{command: "plan", text: "--drupal-version 10|11"},
		{command: "apply", text: "dropkit commerce apply"},
		{command: "verify", text: "dropkit commerce verify"},
	} {
		t.Run(test.command, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := Run([]string{test.command, "--help"}, strings.NewReader(""), &stdout, &stderr)
			if exitCode != 0 || !strings.Contains(stdout.String(), test.text) || stderr.Len() != 0 {
				t.Fatalf("help exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
			}
		})
	}
}

func TestUsageDescribesCommerceCommand(t *testing.T) {
	var output bytes.Buffer
	PrintUsage(&output)
	if !strings.Contains(output.String(), "dropkit commerce plan") || !strings.Contains(output.String(), "drupal/commerce:^3.3") {
		t.Fatalf("usage = %q", output.String())
	}
}

func TestInstallationConfigOwnsCommercePolicy(t *testing.T) {
	wantPackages := []string{"drupal/commerce:^3.3"}
	wantModules := []string{
		"commerce",
		"commerce_cart",
		"commerce_checkout",
		"commerce_order",
		"commerce_store",
		"commerce_price",
		"commerce_tax",
		"commerce_product",
		"commerce_payment",
	}

	if config.CommandName != "commerce" || config.Type != "commerce" || config.ProductName != "Drupal Commerce" {
		t.Fatalf("identity config = %#v", config)
	}
	if config.MinimumDrupalVersion != 10 || config.MaximumDrupalVersion != 11 {
		t.Fatalf("version config = %d through %d", config.MinimumDrupalVersion, config.MaximumDrupalVersion)
	}
	if !slices.Equal(config.ComposerPackages, wantPackages) {
		t.Fatalf("Composer packages = %#v, want %#v", config.ComposerPackages, wantPackages)
	}
	if !slices.Equal(config.EnabledModules, wantModules) {
		t.Fatalf("enabled modules = %#v, want %#v", config.EnabledModules, wantModules)
	}
}
