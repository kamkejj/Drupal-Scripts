package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type scriptedRunner struct {
	paths map[string]string
	calls []CommandRequest
	onRun func(CommandRequest) CommandResult
}

func (runner *scriptedRunner) LookPath(name string) (string, error) {
	if path, ok := runner.paths[name]; ok {
		return path, nil
	}
	return "", errors.New("not found")
}

func (runner *scriptedRunner) Run(_ context.Context, request CommandRequest) CommandResult {
	runner.calls = append(runner.calls, request)
	if runner.onRun != nil {
		return runner.onRun(request)
	}
	return CommandResult{}
}

func newTestModule(runner CommandRunner) *installationModule {
	return &installationModule{
		runner:       runner,
		files:        osFileSystem{},
		platform:     "darwin",
		architecture: "arm64",
	}
}

func installedTools() map[string]string {
	return map[string]string{
		"brew":     "/opt/homebrew/bin/brew",
		"docker":   "/usr/local/bin/docker",
		"colima":   "/opt/homebrew/bin/colima",
		"ddev":     "/opt/homebrew/bin/ddev",
		"composer": "/opt/homebrew/bin/composer",
	}
}

func testRequest(parent string) InstallationRequest {
	return InstallationRequest{
		ProjectName:     "agent-site",
		ParentDirectory: parent,
		DockerProvider:  colima,
		AdminUsername:   "admin",
	}
}

func approvePlan(plan InstallationPlan) Approval {
	allowed := map[Effect]bool{}
	for _, effect := range plan.RequiredApprovals {
		allowed[effect] = true
	}
	return Approval{PlanDigest: plan.Digest, AllowedEffects: allowed}
}

func TestPlanIsReadOnlyAndMachineDescriptive(t *testing.T) {
	parent := t.TempDir()
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)

	plan, err := module.Plan(context.Background(), testRequest(parent))
	if err != nil {
		t.Fatal(err)
	}

	if plan.Blocked {
		t.Fatalf("Plan() blocked = true, blockers = %#v", plan.Blockers)
	}
	if plan.ProjectPath != filepath.Join(parent, "agent-site") {
		t.Fatalf("Plan() project path = %q", plan.ProjectPath)
	}
	if plan.Digest == "" || plan.PlanID == "" || plan.Inspection.Fingerprint == "" {
		t.Fatal("Plan() omitted stable identifiers")
	}
	if _, err := os.Stat(plan.ProjectPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Plan() mutated target: %v", err)
	}
	if len(runner.calls) != 1 || runner.calls[0].Name != "colima" || strings.Join(runner.calls[0].Args, " ") != "status" {
		t.Fatalf("Plan() commands = %#v, want only colima status", runner.calls)
	}
	if !containsEffect(plan.RequiredApprovals, effectNetwork) {
		t.Fatalf("Plan() approvals = %#v, want network", plan.RequiredApprovals)
	}
}

func TestApplyRejectsMissingApprovalBeforeMutation(t *testing.T) {
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)
	plan, err := module.Plan(context.Background(), testRequest(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	callCount := len(runner.calls)

	result, err := module.Apply(context.Background(), plan, Approval{PlanDigest: plan.Digest, AllowedEffects: map[Effect]bool{}}, nil)

	if err == nil || result.Failure == nil || result.Failure.Code != "approval_required" {
		t.Fatalf("Apply() result = %#v, error = %v", result, err)
	}
	if len(runner.calls) != callCount {
		t.Fatalf("Apply() ran commands before approval: %#v", runner.calls[callCount:])
	}
}

func TestApplyRejectsHostDrift(t *testing.T) {
	parent := t.TempDir()
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)
	plan, err := module.Plan(context.Background(), testRequest(parent))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(plan.ProjectPath, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := module.Apply(context.Background(), plan, approvePlan(plan), nil)

	if err == nil || result.Failure == nil || result.Failure.Code != "plan_stale" {
		t.Fatalf("Apply() result = %#v, error = %v", result, err)
	}
}

func TestApplyRejectsAlteredSemanticPlan(t *testing.T) {
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)
	plan, err := module.Plan(context.Background(), testRequest(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	plan.Steps[0].Summary = "Altered"
	plan.Digest, err = planDigest(plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.PlanID = plan.Digest[:12]

	result, err := module.Apply(context.Background(), plan, approvePlan(plan), nil)

	if err == nil || result.Failure == nil || result.Failure.Code != "invalid_plan" {
		t.Fatalf("Apply() result = %#v, error = %v", result, err)
	}
}

func TestApplyAndVerifyReturnStructuredResult(t *testing.T) {
	parent := t.TempDir()
	runner := &scriptedRunner{paths: installedTools()}
	runner.onRun = func(request CommandRequest) CommandResult {
		if request.Name == "composer" && len(request.Args) > 0 && request.Args[0] == "create-project" {
			projectPath := request.Args[len(request.Args)-1]
			if err := os.MkdirAll(filepath.Join(projectPath, "web", "sites", "default"), 0755); err != nil {
				return CommandResult{Err: err, ExitCode: -1}
			}
			if err := os.MkdirAll(filepath.Join(projectPath, ".ddev"), 0755); err != nil {
				return CommandResult{Err: err, ExitCode: -1}
			}
			if err := os.WriteFile(filepath.Join(projectPath, "composer.json"), []byte("{}"), 0644); err != nil {
				return CommandResult{Err: err, ExitCode: -1}
			}
			settings := filepath.Join(projectPath, "web", "sites", "default", "settings.ddev.php")
			if err := os.WriteFile(settings, []byte("sites/default/files/sync"), 0644); err != nil {
				return CommandResult{Err: err, ExitCode: -1}
			}
		}
		if request.Name == "ddev" && len(request.Args) > 0 && request.Args[0] == "describe" {
			return CommandResult{Output: `{"raw":[{"https_url":"https://agent-site.ddev.site"}]}`}
		}
		return CommandResult{}
	}
	module := newTestModule(runner)
	plan, err := module.Plan(context.Background(), testRequest(parent))
	if err != nil {
		t.Fatal(err)
	}

	result, err := module.Apply(context.Background(), plan, approvePlan(plan), nil)

	if err != nil {
		t.Fatalf("Apply() error = %v, result = %#v", err, result)
	}
	if result.Status != "succeeded" || result.SiteURL != "https://agent-site.ddev.site" {
		t.Fatalf("Apply() result = %#v", result)
	}
	if len(result.Verification) != 4 {
		t.Fatalf("Apply() verification = %#v", result.Verification)
	}
}

func TestPlanCommandWritesOnlyJSONToStdout(t *testing.T) {
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runPlanCommand(context.Background(), module, []string{"--name", "agent-site", "--parent", t.TempDir(), "--provider", "colima", "--output", "json"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("runPlanCommand() exit = %d, stderr = %q", exitCode, stderr.String())
	}
	var plan InstallationPlan
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("stdout is not one JSON document: %q: %v", stdout.String(), err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestNonInteractiveInstallNeverPrompts(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runInstallCommand(nil, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "requires a terminal") {
		t.Fatalf("runInstallCommand() exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
	}
}

func TestInstallSubcommandHelpIsDiscoverable(t *testing.T) {
	for _, test := range []struct {
		command string
		text    string
	}{
		{command: "plan", text: "--admin-password-env"},
		{command: "apply", text: "--allow-host-changes"},
		{command: "verify", text: "Verify is read-only"},
	} {
		t.Run(test.command, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := runInstallCommand([]string{test.command, "--help"}, strings.NewReader(""), &stdout, &stderr)
			if exitCode != 0 || !strings.Contains(stdout.String(), test.text) || stderr.Len() != 0 {
				t.Fatalf("help exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
			}
		})
	}
}

func containsEffect(effects []Effect, target Effect) bool {
	for _, effect := range effects {
		if effect == target {
			return true
		}
	}
	return false
}
