package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		DrupalVersion:   defaultDrupalVersion,
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

func findStep(t *testing.T, plan InstallationPlan, id string) InstallationStep {
	t.Helper()
	for _, step := range plan.Steps {
		if step.ID == id {
			return step
		}
	}
	t.Fatalf("plan omitted step %q", id)
	return InstallationStep{}
}

type recordingEventSink struct {
	events []Event
}

func (sink *recordingEventSink) Emit(event Event) {
	sink.events = append(sink.events, event)
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

func TestPlanSupportsDrupalVersionsEightThroughTwelve(t *testing.T) {
	for version := minimumDrupalVersion; version <= maximumDrupalVersion; version++ {
		t.Run(fmt.Sprintf("Drupal%d", version), func(t *testing.T) {
			runner := &scriptedRunner{paths: installedTools()}
			module := newTestModule(runner)
			request := testRequest(t.TempDir())
			request.DrupalVersion = version

			plan, err := module.Plan(context.Background(), request)

			if err != nil {
				t.Fatal(err)
			}
			if plan.Request.DrupalVersion != version {
				t.Fatalf("plan Drupal version = %d, want %d", plan.Request.DrupalVersion, version)
			}
			projectStep := findStep(t, plan, "project.create")
			if !strings.Contains(projectStep.Summary, fmt.Sprintf("Drupal %d", version)) {
				t.Fatalf("project step = %#v", projectStep)
			}
		})
	}
}

func TestCommercePlanReusesDrupalInstallAndAddsCommerceSteps(t *testing.T) {
	for _, version := range []int{10, 11} {
		t.Run(fmt.Sprintf("Drupal%d", version), func(t *testing.T) {
			runner := &scriptedRunner{paths: installedTools()}
			module := newTestModule(runner)
			request := testRequest(t.TempDir())
			request.InstallationType = commerceInstallation
			request.DrupalVersion = version

			plan, err := module.Plan(context.Background(), request)

			if err != nil {
				t.Fatal(err)
			}
			if plan.Request.InstallationType != commerceInstallation {
				t.Fatalf("installation type = %q", plan.Request.InstallationType)
			}
			dependencies := findStep(t, plan, "commerce.dependencies")
			if len(dependencies.DependsOn) != 1 || dependencies.DependsOn[0] != "drupal.sample_content" || dependencies.Disposition != dispositionModify || dependencies.Retry != retryReconcile || !containsEffect(dependencies.Effects, effectNetwork) {
				t.Fatalf("commerce dependencies step = %#v", dependencies)
			}
			modules := findStep(t, plan, "commerce.modules")
			if len(modules.DependsOn) != 1 || modules.DependsOn[0] != dependencies.ID || modules.Disposition != dispositionModify || modules.Retry != retrySafe {
				t.Fatalf("commerce modules step = %#v", modules)
			}
			if plan.Steps[len(plan.Steps)-1].ID != modules.ID {
				t.Fatalf("commerce modules step is not final: %#v", plan.Steps)
			}
		})
	}
}

func TestCommercePlanRejectsUnsupportedDrupalVersionsBeforeInspection(t *testing.T) {
	for _, version := range []int{8, 9, 12} {
		t.Run(fmt.Sprintf("Drupal%d", version), func(t *testing.T) {
			runner := &scriptedRunner{paths: installedTools()}
			module := newTestModule(runner)
			request := testRequest(t.TempDir())
			request.InstallationType = commerceInstallation
			request.DrupalVersion = version

			_, err := module.Plan(context.Background(), request)

			failure := failureFromError(err)
			if err == nil || failure.Code != "invalid_request" || !strings.Contains(failure.Message, "Drupal 10 or 11") {
				t.Fatalf("Plan() error = %#v", err)
			}
			if len(runner.calls) != 0 {
				t.Fatalf("Plan() inspected invalid request: %#v", runner.calls)
			}
		})
	}
}

func TestPlanNormalizesRequestAndCollectsDestructiveApproval(t *testing.T) {
	parent := t.TempDir()
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)
	request := testRequest(parent)
	request.ProjectName = "  Agent Site  "
	request.AdminUsername = ""
	request.GenerateContent = true

	plan, err := module.Plan(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}
	if plan.Request.ProjectName != "agent-site" || plan.Request.AdminUsername != "admin" {
		t.Fatalf("normalized request = %#v", plan.Request)
	}
	if !filepath.IsAbs(plan.Request.ParentDirectory) || plan.ProjectPath != filepath.Join(parent, "agent-site") {
		t.Fatalf("paths = parent %q, project %q", plan.Request.ParentDirectory, plan.ProjectPath)
	}
	if !containsEffect(plan.RequiredApprovals, effectDestructive) {
		t.Fatalf("approvals = %#v", plan.RequiredApprovals)
	}
	sampleStep := findStep(t, plan, "drupal.sample_content")
	if sampleStep.Disposition != dispositionModify || sampleStep.Retry != retryManual || !containsEffect(sampleStep.Effects, effectDestructive) {
		t.Fatalf("sample content step = %#v", sampleStep)
	}
}

func TestPlanRejectsInvalidRequestsBeforeInspection(t *testing.T) {
	tests := []struct {
		name   string
		change func(*InstallationRequest)
		text   string
	}{
		{name: "missing name", change: func(request *InstallationRequest) { request.ProjectName = "" }, text: "project name is required"},
		{name: "leading hyphen", change: func(request *InstallationRequest) { request.ProjectName = "-site" }, text: "only lowercase"},
		{name: "trailing hyphen", change: func(request *InstallationRequest) { request.ProjectName = "site-" }, text: "cannot end"},
		{name: "invalid character", change: func(request *InstallationRequest) { request.ProjectName = "site_name" }, text: "only lowercase"},
		{name: "provider", change: func(request *InstallationRequest) { request.DockerProvider = "podman" }, text: "docker or colima"},
		{name: "parent", change: func(request *InstallationRequest) { request.ParentDirectory = " " }, text: "parent directory is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &scriptedRunner{paths: installedTools()}
			module := newTestModule(runner)
			request := testRequest(t.TempDir())
			test.change(&request)

			_, err := module.Plan(context.Background(), request)

			failure := failureFromError(err)
			if err == nil || failure.Code != "invalid_request" || !strings.Contains(failure.Message, test.text) {
				t.Fatalf("Plan() error = %#v, want %q", err, test.text)
			}
			if len(runner.calls) != 0 {
				t.Fatalf("Plan() inspected invalid request: %#v", runner.calls)
			}
		})
	}
}

func TestPlanDescribesHostAndTargetBlockers(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*testing.T, *installationModule, *scriptedRunner, *InstallationRequest)
		stepID  string
	}{
		{
			name: "unsupported platform",
			prepare: func(_ *testing.T, module *installationModule, _ *scriptedRunner, _ *InstallationRequest) {
				module.platform = "linux"
			},
			stepID: "host.platform",
		},
		{
			name: "missing homebrew",
			prepare: func(_ *testing.T, _ *installationModule, runner *scriptedRunner, _ *InstallationRequest) {
				delete(runner.paths, "brew")
			},
			stepID: "host.homebrew",
		},
		{
			name: "stopped Docker Desktop",
			prepare: func(_ *testing.T, _ *installationModule, runner *scriptedRunner, request *InstallationRequest) {
				request.DockerProvider = dockerDesktop
				runner.onRun = func(command CommandRequest) CommandResult {
					if command.Name == "docker" {
						return CommandResult{Err: errors.New("not running"), ExitCode: 1}
					}
					return CommandResult{}
				}
			},
			stepID: "runtime.start",
		},
		{
			name: "existing directory",
			prepare: func(t *testing.T, _ *installationModule, _ *scriptedRunner, request *InstallationRequest) {
				t.Helper()
				if err := os.Mkdir(filepath.Join(request.ParentDirectory, request.ProjectName), 0755); err != nil {
					t.Fatal(err)
				}
			},
			stepID: "project.create",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &scriptedRunner{paths: installedTools()}
			module := newTestModule(runner)
			request := testRequest(t.TempDir())
			test.prepare(t, module, runner, &request)

			plan, err := module.Plan(context.Background(), request)

			if err != nil {
				t.Fatal(err)
			}
			step := findStep(t, plan, test.stepID)
			if !plan.Blocked || step.Disposition != dispositionBlocked {
				t.Fatalf("plan blocked = %t, step = %#v", plan.Blocked, step)
			}
			if len(plan.Blockers) == 0 {
				t.Fatal("blocked plan omitted structured blockers")
			}
		})
	}
}

func TestPlanReconcilesExistingDrupalProject(t *testing.T) {
	parent := t.TempDir()
	projectPath := filepath.Join(parent, "agent-site")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "composer.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)

	plan, err := module.Plan(context.Background(), testRequest(parent))

	if err != nil {
		t.Fatal(err)
	}
	if plan.Blocked || findStep(t, plan, "project.create").Disposition != dispositionNoOp {
		t.Fatalf("existing project plan = %#v", plan)
	}
}

func TestPlanRejectsUnsupportedDrupalVersion(t *testing.T) {
	for _, version := range []int{0, 7, 13} {
		t.Run(fmt.Sprintf("Version%d", version), func(t *testing.T) {
			runner := &scriptedRunner{paths: installedTools()}
			module := newTestModule(runner)
			request := testRequest(t.TempDir())
			request.DrupalVersion = version

			_, err := module.Plan(context.Background(), request)

			failure := failureFromError(err)
			if err == nil || failure.Code != "invalid_request" || !strings.Contains(failure.Recovery, "--drupal-version") {
				t.Fatalf("Plan() error = %#v", err)
			}
			if len(runner.calls) != 0 {
				t.Fatalf("Plan() inspected host for invalid version: %#v", runner.calls)
			}
		})
	}
}

func TestApplyUsesSelectedDrupalVersionForComposerAndDDEV(t *testing.T) {
	for version := minimumDrupalVersion; version <= maximumDrupalVersion; version++ {
		t.Run(fmt.Sprintf("Drupal%d", version), func(t *testing.T) {
			runner := &scriptedRunner{paths: installedTools()}
			module := newTestModule(runner)
			plan := InstallationPlan{
				ProjectPath: "/projects/site",
				Request: InstallationRequest{
					ParentDirectory: "/projects",
					DrupalVersion:   version,
				},
			}
			emit := func(Event) {}

			if _, err := module.applyStep(context.Background(), plan, InstallationStep{ID: "project.create"}, emit); err != nil {
				t.Fatal(err)
			}
			if _, err := module.applyStep(context.Background(), plan, InstallationStep{ID: "ddev.configure"}, emit); err != nil {
				t.Fatal(err)
			}

			if len(runner.calls) != 2 {
				t.Fatalf("calls = %#v", runner.calls)
			}
			composerArgs := strings.Join(runner.calls[0].Args, " ")
			if composerArgs != fmt.Sprintf("create-project drupal/recommended-project:%s /projects/site", drupalProjectConstraint(version)) {
				t.Fatalf("Composer args = %q", composerArgs)
			}
			ddevArgs := strings.Join(runner.calls[1].Args, " ")
			if ddevArgs != fmt.Sprintf("config --project-type=drupal%d --docroot=web --create-docroot", version) {
				t.Fatalf("DDEV args = %q", ddevArgs)
			}
		})
	}
}

func TestDrupalDependenciesMatchSelectedCoreVersion(t *testing.T) {
	for _, version := range []int{8, 11, 12} {
		t.Run(fmt.Sprintf("Drupal%d", version), func(t *testing.T) {
			runner := &scriptedRunner{paths: installedTools()}
			module := newTestModule(runner)
			plan := InstallationPlan{ProjectPath: "/projects/site", Request: InstallationRequest{DrupalVersion: version}}

			_, err := module.applyStep(context.Background(), plan, InstallationStep{ID: "drupal.dependencies"}, func(Event) {})

			if err != nil {
				t.Fatal(err)
			}
			if len(runner.calls) < 2 {
				t.Fatalf("calls = %#v", runner.calls)
			}
			coreDevArgs := strings.Join(runner.calls[1].Args, " ")
			want := "composer require drupal/core-dev:" + drupalProjectConstraint(version) + " --dev -W"
			if coreDevArgs != want {
				t.Fatalf("core-dev args = %q, want %q", coreDevArgs, want)
			}
		})
	}
}

func TestCommerceDependenciesUseRequestedComposerConstraint(t *testing.T) {
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)
	plan := InstallationPlan{ProjectPath: "/projects/store", Request: InstallationRequest{InstallationType: commerceInstallation, DrupalVersion: 11}}

	_, err := module.applyStep(context.Background(), plan, InstallationStep{ID: "commerce.dependencies"}, func(Event) {})

	if err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	call := runner.calls[0]
	if call.Name != "ddev" || call.Dir != plan.ProjectPath || strings.Join(call.Args, " ") != "composer require drupal/commerce:^3.3" {
		t.Fatalf("commerce command = %#v", call)
	}
}

func TestCommerceModulesAreEnabled(t *testing.T) {
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)
	plan := InstallationPlan{ProjectPath: "/projects/store", Request: InstallationRequest{InstallationType: commerceInstallation, DrupalVersion: 11}}

	_, err := module.applyStep(context.Background(), plan, InstallationStep{ID: "commerce.modules"}, func(Event) {})

	if err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	call := runner.calls[0]
	want := "drush en -y " + strings.Join(commerceModules(), " ")
	if call.Name != "ddev" || call.Dir != plan.ProjectPath || strings.Join(call.Args, " ") != want {
		t.Fatalf("commerce module command = %#v, want %q", call, want)
	}
}

func TestDrupalTwelveUsesOnlyCompatibleContribModules(t *testing.T) {
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)
	plan := InstallationPlan{ProjectPath: "/projects/site", Request: InstallationRequest{DrupalVersion: 12}}

	if _, err := module.applyStep(context.Background(), plan, InstallationStep{ID: "drupal.dependencies"}, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	if _, err := module.applyStep(context.Background(), plan, InstallationStep{ID: "drupal.modules"}, func(Event) {}); err != nil {
		t.Fatal(err)
	}

	dependencyArgs := strings.Join(runner.calls[2].Args, " ")
	for _, incompatible := range []string{"admin_toolbar", "pathauto", "config_ignore", "config_split", "better_exposed_filters", "key", "webprofiler", "diff", "ultimate_cron"} {
		if strings.Contains(dependencyArgs, incompatible) {
			t.Errorf("Drupal 12 dependency command contains incompatible module %q: %s", incompatible, dependencyArgs)
		}
	}
	for _, compatible := range []string{"drush/drush", "drupal/token", "drupal/devel", "drupal/environment_indicator"} {
		if !strings.Contains(dependencyArgs, compatible) {
			t.Errorf("Drupal 12 dependency command omitted compatible package %q: %s", compatible, dependencyArgs)
		}
	}
	enableArgs := strings.Join(runner.calls[3].Args, " ")
	if enableArgs != "drush en -y devel devel_generate environment_indicator environment_indicator_ui environment_indicator_toolbar token" {
		t.Fatalf("Drupal 12 enable args = %q", enableArgs)
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

func TestApplyRejectsUnsupportedDrupalVersionBeforeInspection(t *testing.T) {
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)
	plan, err := module.Plan(context.Background(), testRequest(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	callCount := len(runner.calls)
	plan.Request.DrupalVersion = 13
	plan.Digest, err = planDigest(plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.PlanID = plan.Digest[:12]

	result, err := module.Apply(context.Background(), plan, approvePlan(plan), nil)

	if err == nil || result.Failure == nil || result.Failure.Code != "invalid_plan" {
		t.Fatalf("Apply() result = %#v, error = %v", result, err)
	}
	if len(runner.calls) != callCount {
		t.Fatalf("Apply() inspected host for invalid plan: %#v", runner.calls[callCount:])
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

func TestApplyStopsAtFailedStepAndEmitsRedactedEvents(t *testing.T) {
	parent := t.TempDir()
	runner := &scriptedRunner{paths: installedTools()}
	runner.onRun = func(request CommandRequest) CommandResult {
		if request.Name == "composer" {
			return CommandResult{Output: "admin could not create project", Err: errors.New("composer failed"), ExitCode: 42}
		}
		return CommandResult{}
	}
	module := newTestModule(runner)
	plan, err := module.Plan(context.Background(), testRequest(parent))
	if err != nil {
		t.Fatal(err)
	}
	sink := &recordingEventSink{}

	result, err := module.Apply(context.Background(), plan, approvePlan(plan), sink)

	if err == nil || result.Status != "partial" || result.Failure == nil {
		t.Fatalf("Apply() result = %#v, error = %v", result, err)
	}
	if result.Failure.Code != "step_failed" || result.Failure.StepID != "project.create" || result.Failure.ExitCode == nil || *result.Failure.ExitCode != 42 {
		t.Fatalf("failure = %#v", result.Failure)
	}
	if len(result.Steps) == 0 || result.Steps[len(result.Steps)-1].Status != "failed" {
		t.Fatalf("steps = %#v", result.Steps)
	}
	if len(sink.events) != 3 {
		t.Fatalf("events = %#v", sink.events)
	}
	for index, event := range sink.events {
		if event.Sequence != index+1 || event.Time == "" {
			t.Errorf("event %d = %#v", index, event)
		}
	}
	if sink.events[1].Type != "command_output" || strings.Contains(sink.events[1].Message, "admin") || !strings.Contains(sink.events[1].Message, "[redacted]") {
		t.Fatalf("command event = %#v", sink.events[1])
	}
}

func TestApplyRejectsBlockedPlanAndMismatchedDigestBeforeInspection(t *testing.T) {
	for _, test := range []struct {
		name     string
		change   func(*InstallationPlan, *Approval)
		wantCode string
	}{
		{
			name: "blocked plan",
			change: func(plan *InstallationPlan, _ *Approval) {
				plan.Blocked = true
				plan.Digest, _ = planDigest(*plan)
				plan.PlanID = plan.Digest[:12]
			},
			wantCode: "plan_blocked",
		},
		{
			name: "mismatched approval digest",
			change: func(_ *InstallationPlan, approval *Approval) {
				approval.PlanDigest = "another-plan"
			},
			wantCode: "approval_required",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &scriptedRunner{paths: installedTools()}
			module := newTestModule(runner)
			plan, err := module.Plan(context.Background(), testRequest(t.TempDir()))
			if err != nil {
				t.Fatal(err)
			}
			approval := approvePlan(plan)
			test.change(&plan, &approval)
			callCount := len(runner.calls)

			result, err := module.Apply(context.Background(), plan, approval, nil)

			if err == nil || result.Failure == nil || result.Failure.Code != test.wantCode {
				t.Fatalf("Apply() result = %#v, error = %v", result, err)
			}
			if len(runner.calls) != callCount {
				t.Fatalf("Apply() inspected before rejecting: %#v", runner.calls[callCount:])
			}
		})
	}
}

func TestValidatePlanRejectsMalformedSemanticData(t *testing.T) {
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)
	valid, err := module.Plan(context.Background(), testRequest(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		change func(*InstallationPlan)
		text   string
	}{
		{name: "schema", change: func(plan *InstallationPlan) { plan.SchemaVersion = "1" }, text: "schema"},
		{name: "request schema", change: func(plan *InstallationPlan) { plan.Request.SchemaVersion = "1" }, text: "unsupported Drupal version"},
		{name: "request version", change: func(plan *InstallationPlan) { plan.Request.DrupalVersion = 13 }, text: "unsupported Drupal version"},
		{name: "digest", change: func(plan *InstallationPlan) { plan.Digest = strings.Repeat("0", 64) }, text: "digest"},
		{name: "plan ID", change: func(plan *InstallationPlan) { plan.PlanID = "invalid-plan" }, text: "digest"},
		{
			name: "unknown step",
			change: func(plan *InstallationPlan) {
				plan.Steps[0].ID = "shell.execute"
				plan.Digest, _ = planDigest(*plan)
				plan.PlanID = plan.Digest[:12]
			},
			text: "unknown step",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := valid
			plan.Steps = append([]InstallationStep(nil), valid.Steps...)
			test.change(&plan)

			failure := validatePlan(plan)

			if failure == nil || failure.Code != "invalid_plan" || !strings.Contains(failure.Message, test.text) {
				t.Fatalf("validatePlan() = %#v, want %q", failure, test.text)
			}
		})
	}
}

func TestApplyStepUsesPasswordEnvironmentAndRedactsSecret(t *testing.T) {
	const environmentName = "DROPKIT_TEST_ADMIN_PASSWORD"
	t.Setenv(environmentName, "highly-secret")
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)
	plan := InstallationPlan{
		ProjectPath: "/projects/site",
		Request: InstallationRequest{
			AdminUsername:    "site-owner",
			AdminPasswordEnv: environmentName,
		},
	}
	var events []Event

	_, err := module.applyStep(context.Background(), plan, InstallationStep{ID: "drupal.site"}, func(event Event) {
		events = append(events, event)
	})

	if err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	args := strings.Join(runner.calls[0].Args, " ")
	if !strings.Contains(args, "--account-name=site-owner") || !strings.Contains(args, "--account-pass=highly-secret") {
		t.Fatalf("site install args = %q", args)
	}

	runner.onRun = func(CommandRequest) CommandResult {
		return CommandResult{Output: "password=highly-secret"}
	}
	_, err = module.applyStep(context.Background(), plan, InstallationStep{ID: "ddev.start"}, func(event Event) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Message != "password=[redacted]" {
		t.Fatalf("events = %#v", events)
	}
}

func TestApplyStepRequiresNonEmptyPasswordEnvironment(t *testing.T) {
	const environmentName = "DROPKIT_TEST_EMPTY_PASSWORD"
	t.Setenv(environmentName, "")
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)
	plan := InstallationPlan{Request: InstallationRequest{AdminPasswordEnv: environmentName}}

	_, err := module.applyStep(context.Background(), plan, InstallationStep{ID: "drupal.site"}, func(Event) {})

	if err == nil || !strings.Contains(err.Error(), environmentName) || len(runner.calls) != 0 {
		t.Fatalf("applyStep() error = %v, calls = %#v", err, runner.calls)
	}
}

func TestVerifyReportsEveryFailedCheck(t *testing.T) {
	runner := &scriptedRunner{paths: installedTools(), onRun: func(request CommandRequest) CommandResult {
		if request.Name == "ddev" {
			return CommandResult{Output: "runtime unavailable", Err: errors.New("failed"), ExitCode: 1}
		}
		return CommandResult{}
	}}
	module := newTestModule(runner)
	plan, err := module.Plan(context.Background(), testRequest(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}

	result, err := module.Verify(context.Background(), plan, nil)

	if err == nil || result.Failure == nil || result.Failure.Code != "verification_failed" || result.Status != "failed" {
		t.Fatalf("Verify() result = %#v, error = %v", result, err)
	}
	if len(result.Verification) != 4 {
		t.Fatalf("checks = %#v", result.Verification)
	}
	for _, check := range result.Verification {
		if check.Passed {
			t.Errorf("check unexpectedly passed: %#v", check)
		}
	}
	if result.Verification[3].Message != "runtime unavailable" {
		t.Fatalf("DDEV check = %#v", result.Verification[3])
	}
}

func TestVerifyCommerceChecksInstalledPackage(t *testing.T) {
	parent := t.TempDir()
	runner := &scriptedRunner{paths: installedTools(), onRun: func(request CommandRequest) CommandResult {
		if len(request.Args) >= 2 && request.Args[0] == "describe" {
			return CommandResult{Output: `{"raw":[{"https_url":"https://store.ddev.site"}]}`}
		}
		if len(request.Args) >= 2 && request.Args[0] == "drush" && request.Args[1] == "pm:list" {
			return CommandResult{Output: strings.Join(commerceModules(), "\n")}
		}
		return CommandResult{}
	}}
	module := newTestModule(runner)
	request := testRequest(parent)
	request.InstallationType = commerceInstallation
	plan, err := module.Plan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{
		filepath.Join(plan.ProjectPath, ".ddev"),
		filepath.Join(plan.ProjectPath, "web", "sites", "default"),
	} {
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{
		filepath.Join(plan.ProjectPath, "composer.json"),
		filepath.Join(plan.ProjectPath, "web", "sites", "default", "settings.ddev.php"),
	} {
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := module.Verify(context.Background(), plan, nil)

	if err != nil || result.Status != "succeeded" || result.SiteURL != "https://store.ddev.site" {
		t.Fatalf("Verify() result = %#v, error = %v", result, err)
	}
	if len(result.Verification) != 6 || result.Verification[4].ID != "commerce.package" || !result.Verification[4].Passed || result.Verification[5].ID != "commerce.modules" || !result.Verification[5].Passed {
		t.Fatalf("checks = %#v", result.Verification)
	}
	lastCall := runner.calls[len(runner.calls)-1]
	if lastCall.Name != "ddev" || strings.Join(lastCall.Args, " ") != "drush pm:list --type=module --status=enabled --field=name" || lastCall.Dir != plan.ProjectPath {
		t.Fatalf("commerce module verification command = %#v", lastCall)
	}
}

func TestHelpersReturnStableFailureAndSiteData(t *testing.T) {
	failure := installationFailure("step_failed", "project.create", "failed", true, "retry")
	if failure.Error() != "failed" {
		t.Fatalf("failure.Error() = %q", failure.Error())
	}
	if got := failureFromError(errors.New("boom")); got.Code != "internal_error" || got.Message != "boom" {
		t.Fatalf("failureFromError() = %#v", got)
	}
	for _, test := range []struct {
		name   string
		output string
		want   string
	}{
		{name: "valid", output: `{"raw":[{"https_url":"https://site.ddev.site"}]}`, want: "https://site.ddev.site"},
		{name: "invalid", output: `{`, want: ""},
		{name: "empty", output: `{"raw":[]}`, want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := siteURLFromDescribe(test.output); got != test.want {
				t.Fatalf("siteURLFromDescribe() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPlanCommandWritesOnlyJSONToStdout(t *testing.T) {
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runPlanCommand(context.Background(), module, []string{"--name", "agent-site", "--parent", t.TempDir(), "--provider", "colima", "--drupal-version", "12", "--output", "json"}, &stdout, &stderr)

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
	if plan.Request.DrupalVersion != 12 {
		t.Fatalf("plan Drupal version = %d", plan.Request.DrupalVersion)
	}
}

func TestPlanCommandRequiresDrupalVersion(t *testing.T) {
	runner := &scriptedRunner{paths: installedTools()}
	module := newTestModule(runner)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runPlanCommand(context.Background(), module, []string{"--name", "agent-site", "--parent", t.TempDir(), "--provider", "colima"}, &stdout, &stderr)

	if exitCode != 2 || !strings.Contains(stderr.String(), "Drupal version must be between 8 and 12") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
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

func TestNonInteractiveCommerceNeverPrompts(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCommerceCommand(nil, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "interactive commerce requires a terminal") {
		t.Fatalf("runCommerceCommand() exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
	}
}

func TestCommerceSubcommandHelpIsDiscoverable(t *testing.T) {
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
			exitCode := runCommerceCommand([]string{test.command, "--help"}, strings.NewReader(""), &stdout, &stderr)
			if exitCode != 0 || !strings.Contains(stdout.String(), test.text) || stderr.Len() != 0 {
				t.Fatalf("help exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
			}
		})
	}
}

func TestInstallSubcommandHelpIsDiscoverable(t *testing.T) {
	for _, test := range []struct {
		command string
		text    string
	}{
		{command: "plan", text: "--drupal-version VERSION"},
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
