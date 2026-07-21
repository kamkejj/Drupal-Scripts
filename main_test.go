package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunCLI(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		exitCode   int
		stdoutText string
		stderrText string
	}{
		{
			name:       "missing command",
			exitCode:   1,
			stderrText: "Usage: dropkit <command>",
		},
		{
			name:       "help",
			args:       []string{"help"},
			stdoutText: "install    Install a Drupal 11 development environment",
		},
		{
			name:       "short help",
			args:       []string{"-h"},
			stdoutText: "Usage: dropkit <command>",
		},
		{
			name:       "long help",
			args:       []string{"--help"},
			stdoutText: "Usage: dropkit <command>",
		},
		{
			name:       "install help",
			args:       []string{"install", "--help"},
			stdoutText: "dropkit install plan",
		},
		{
			name:       "help install",
			args:       []string{"help", "install"},
			stdoutText: "dropkit install apply",
		},
		{
			name:       "unknown command",
			args:       []string{"update"},
			exitCode:   1,
			stderrText: `unknown command "update"`,
		},
		{
			name:       "unknown help topic",
			args:       []string{"help", "update"},
			exitCode:   1,
			stderrText: `unknown help topic "update"`,
		},
		{
			name:       "unknown install command",
			args:       []string{"install", "project-name"},
			exitCode:   2,
			stderrText: `unknown install command "project-name"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			exitCode := runCLI(test.args, &stdout, &stderr)

			if exitCode != test.exitCode {
				t.Fatalf("runCLI() exit code = %d, want %d", exitCode, test.exitCode)
			}
			if !strings.Contains(stdout.String(), test.stdoutText) {
				t.Errorf("runCLI() stdout = %q, want text %q", stdout.String(), test.stdoutText)
			}
			if !strings.Contains(stderr.String(), test.stderrText) {
				t.Errorf("runCLI() stderr = %q, want text %q", stderr.String(), test.stderrText)
			}
		})
	}
}
