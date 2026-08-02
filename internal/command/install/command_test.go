package install

import (
	"bytes"
	"strings"
	"testing"
)

func TestNonInteractiveCommandNeverPrompts(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(nil, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "interactive install requires a terminal") {
		t.Fatalf("Run() exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
	}
}

func TestSubcommandHelpIsDiscoverable(t *testing.T) {
	for _, test := range []struct {
		command string
		text    string
	}{
		{command: "plan", text: "--drupal-version 8|9|10|11|12"},
		{command: "apply", text: "--allow-host-changes"},
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

func TestUsageDescribesInstallCommand(t *testing.T) {
	var output bytes.Buffer
	PrintUsage(&output)
	if !strings.Contains(output.String(), "dropkit install plan") || strings.Contains(output.String(), "drupal/commerce") {
		t.Fatalf("usage = %q", output.String())
	}
}
