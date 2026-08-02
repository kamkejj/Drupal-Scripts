package cms

import (
	"bytes"
	"strings"
	"testing"
)

func TestNonInteractiveCommandNeverPrompts(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(nil, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "interactive cms requires a terminal") {
		t.Fatalf("Run() exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
	}
}

func TestSubcommandHelpIsDiscoverable(t *testing.T) {
	for _, test := range []struct {
		command string
		text    string
	}{
		{command: "plan", text: "dropkit cms plan --name NAME"},
		{command: "apply", text: "dropkit cms apply"},
		{command: "verify", text: "Verify is read-only"},
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

func TestUsageDescribesCMSWorkflow(t *testing.T) {
	var output bytes.Buffer
	PrintUsage(&output)
	usage := output.String()
	if !strings.Contains(usage, "drupal/cms") || !strings.Contains(usage, "browser setup assistant") {
		t.Fatalf("usage = %q", usage)
	}
}

func TestInstallationConfigOwnsCMSPolicy(t *testing.T) {
	if config.CommandName != "cms" || config.Type != "cms" || config.ProductName != "Drupal CMS" {
		t.Fatalf("identity config = %#v", config)
	}
	if config.MinimumDrupalVersion != 11 || config.MaximumDrupalVersion != 11 || config.FixedDrupalVersion != 11 {
		t.Fatalf("version config = %#v", config)
	}
	if config.ProjectTemplate != "drupal/cms" || !config.BrowserInstaller {
		t.Fatalf("workflow config = %#v", config)
	}
}

func TestPlanRejectsCoreOnlyOptions(t *testing.T) {
	for _, option := range []string{"--drupal-version", "--generate-content", "--admin-user"} {
		t.Run(option, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := Run([]string{"plan", "--output", "json", option, "11"}, strings.NewReader(""), &stdout, &stderr)
			if exitCode != 2 || stderr.Len() != 0 || !strings.Contains(stdout.String(), `"code": "invalid_request"`) {
				t.Fatalf("Run() exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
			}
		})
	}
}
