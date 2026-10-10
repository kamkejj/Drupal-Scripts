package delete

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".ddev"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".ddev", "config.yaml"), []byte("name: test\n"), 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		args       []string
		input      string
		terminal   bool
		cwd        string
		commandErr error
		wantCode   int
		wantCalls  int
		wantOutput string
	}{
		{name: "explicit approval", args: []string{"--yes"}, cwd: dir, wantCalls: 1},
		{name: "interactive approval", input: "delete\n", terminal: true, cwd: dir, wantCalls: 1},
		{name: "interactive refusal", input: "no\n", terminal: true, cwd: dir, wantCode: 2, wantOutput: "deletion cancelled"},
		{name: "noninteractive refusal", cwd: dir, wantCode: 2, wantOutput: "--yes"},
		{name: "missing config", args: []string{"--yes"}, cwd: t.TempDir(), wantCode: 2, wantOutput: "no DDEV project"},
		{name: "invalid argument", args: []string{"--all"}, cwd: dir, wantCode: 2, wantOutput: "unknown delete argument"},
		{name: "duplicate approval", args: []string{"--yes", "--yes"}, cwd: dir, wantCode: 2, wantOutput: "duplicate --yes"},
		{name: "missing ddev", args: []string{"--yes"}, cwd: dir, commandErr: exec.ErrNotFound, wantCode: 1, wantCalls: 1, wantOutput: "cannot run ddev delete"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			calls := 0
			code := run(test.args, strings.NewReader(test.input), &stdout, &stderr,
				func() (string, error) { return test.cwd, nil },
				func(io.Reader) bool { return test.terminal },
				func(actualDir string, out, errOut io.Writer) error {
					calls++
					if actualDir != dir {
						t.Errorf("command directory = %q, want %q", actualDir, dir)
					}
					return test.commandErr
				})
			if code != test.wantCode || calls != test.wantCalls {
				t.Errorf("exit code = %d, calls = %d; want %d, %d", code, calls, test.wantCode, test.wantCalls)
			}
			if !strings.Contains(stderr.String(), test.wantOutput) {
				t.Errorf("stderr = %q, want %q", stderr.String(), test.wantOutput)
			}
		})
	}
}

func TestRunDirectoryFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--yes"}, strings.NewReader(""), &stdout, &stderr,
		func() (string, error) { return "", errors.New("unavailable") },
		func(io.Reader) bool { return false },
		func(string, io.Writer, io.Writer) error { t.Fatal("unexpected execution"); return nil })
	if code != 1 || !strings.Contains(stderr.String(), "unavailable") {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
}

func TestExecuteUsesCurrentProjectAndSkipsSnapshot(t *testing.T) {
	dir := t.TempDir()
	bin := t.TempDir()
	output := filepath.Join(t.TempDir(), "invocation")
	script := "#!/bin/sh\nprintf '%s\\n' \"$PWD\" \"$@\" > \"$DROP_KIT_TEST_OUTPUT\"\n"
	if err := os.WriteFile(filepath.Join(bin, "ddev"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DROP_KIT_TEST_OUTPUT", output)
	if err := execute(dir, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(contents), dir+"\ndelete\n--omit-snapshot\n--yes\n"; got != want {
		t.Errorf("invocation = %q, want %q", got, want)
	}
}
