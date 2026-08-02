package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type cliTestModule struct {
	planResult   InstallationPlan
	planErr      error
	applyResult  InstallationResult
	applyErr     error
	verifyResult InstallationResult
	verifyErr    error
	request      InstallationRequest
	plan         InstallationPlan
	approval     Approval
	applyEvent   *Event
	verifyEvent  *Event
}

func (module *cliTestModule) Plan(_ context.Context, request InstallationRequest) (InstallationPlan, error) {
	module.request = request
	return module.planResult, module.planErr
}

func (module *cliTestModule) Apply(_ context.Context, plan InstallationPlan, approval Approval, sink EventSink) (InstallationResult, error) {
	module.plan = plan
	module.approval = approval
	if module.applyEvent != nil {
		sink.Emit(*module.applyEvent)
	}
	return module.applyResult, module.applyErr
}

func (module *cliTestModule) Verify(_ context.Context, plan InstallationPlan, sink EventSink) (InstallationResult, error) {
	module.plan = plan
	if module.verifyEvent != nil {
		sink.Emit(*module.verifyEvent)
	}
	return module.verifyResult, module.verifyErr
}

func writeTestPlan(t *testing.T, plan InstallationPlan) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plan.json")
	content, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func decodeSingleJSON[T any](t *testing.T, content []byte) T {
	t.Helper()
	var value T
	decoder := json.NewDecoder(bytes.NewReader(content))
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, content)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		t.Fatalf("output contains more than one JSON document: %q", content)
	}
	return value
}

func TestRunApplyCommandHonorsApprovalsAndMachineStreams(t *testing.T) {
	plan := InstallationPlan{SchemaVersion: installationSchemaVersion, PlanID: "plan-id", Digest: "digest", ProjectPath: "/projects/site"}
	event := Event{Sequence: 1, Type: "step_started", Level: "info", StepID: "project.create", Message: "Creating project"}
	module := &cliTestModule{
		applyResult: InstallationResult{SchemaVersion: installationSchemaVersion, PlanID: plan.PlanID, Status: "succeeded", ProjectPath: plan.ProjectPath},
		applyEvent:  &event,
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runApplyCommandForConfig(context.Background(), module, testDrupalConfig, []string{
		"--plan", writeTestPlan(t, plan),
		"--allow-network",
		"--allow-host-changes",
		"--allow-destructive",
		"--output", "json",
		"--events", "jsonl",
	}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
	}
	result := decodeSingleJSON[InstallationResult](t, stdout.Bytes())
	if result.Status != "succeeded" || result.PlanID != plan.PlanID {
		t.Fatalf("result = %#v", result)
	}
	streamed := decodeSingleJSON[Event](t, stderr.Bytes())
	if streamed.StepID != event.StepID || streamed.Type != event.Type {
		t.Fatalf("event = %#v", streamed)
	}
	if module.approval.PlanDigest != plan.Digest {
		t.Fatalf("approval digest = %q", module.approval.PlanDigest)
	}
	for _, effect := range []Effect{effectNetwork, effectHostChange, effectDestructive} {
		if !module.approval.AllowedEffects[effect] {
			t.Errorf("approval omitted %s", effect)
		}
	}
}

func TestRunApplyCommandFailureStillWritesOneJSONResult(t *testing.T) {
	failure := installationFailure("step_failed", "project.create", "composer failed", true, "retry")
	module := &cliTestModule{
		applyResult: InstallationResult{SchemaVersion: installationSchemaVersion, Status: "partial", Failure: &failure},
		applyErr:    failure,
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runApplyCommandForConfig(context.Background(), module, testDrupalConfig, []string{
		"--plan", writeTestPlan(t, InstallationPlan{Digest: "digest"}),
		"--output", "json",
		"--events", "none",
	}, &stdout, &stderr)

	if exitCode != 5 || stderr.Len() != 0 {
		t.Fatalf("exit = %d, stderr = %q", exitCode, stderr.String())
	}
	result := decodeSingleJSON[InstallationResult](t, stdout.Bytes())
	if result.Failure == nil || result.Failure.Code != "step_failed" || result.Failure.StepID != "project.create" {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunVerifyCommandUsesSavedPlanAndEventFormat(t *testing.T) {
	plan := InstallationPlan{SchemaVersion: installationSchemaVersion, PlanID: "plan-id", Digest: "digest", ProjectPath: "/projects/site"}
	event := Event{Type: "verification", Level: "info", StepID: "project.composer", Message: "present"}
	module := &cliTestModule{
		verifyResult: InstallationResult{SchemaVersion: installationSchemaVersion, PlanID: plan.PlanID, Status: "succeeded"},
		verifyEvent:  &event,
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runVerifyCommandForConfig(context.Background(), module, testDrupalConfig, []string{
		"--plan", writeTestPlan(t, plan),
		"--output", "json",
		"--events", "human",
	}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
	}
	result := decodeSingleJSON[InstallationResult](t, stdout.Bytes())
	if result.Status != "succeeded" || module.plan.PlanID != plan.PlanID {
		t.Fatalf("result = %#v, plan = %#v", result, module.plan)
	}
	if !strings.Contains(stderr.String(), "[INFO] project.composer: present") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestConfiguredPlanCommandSetsInstallationType(t *testing.T) {
	module := &cliTestModule{planResult: InstallationPlan{PlanID: "store-plan"}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runPlanCommandForConfig(context.Background(), module, testExtensionConfig, []string{
		"--name", "store",
		"--parent", "/projects",
		"--provider", "colima",
		"--drupal-version", "10",
	}, &stdout, &stderr)

	if exitCode != 0 || stderr.Len() != 0 {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
	}
	if module.request.InstallationType != testExtensionConfig.Type || module.request.DrupalVersion != 10 {
		t.Fatalf("request = %#v", module.request)
	}
}

func TestConfiguredCommandRejectsPlanForDifferentInstallation(t *testing.T) {
	plan := InstallationPlan{Request: InstallationRequest{InstallationType: testDrupalConfig.Type}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runApplyCommandForConfig(context.Background(), &cliTestModule{}, testExtensionConfig, []string{
		"--plan", writeTestPlan(t, plan),
	}, &stdout, &stderr)

	if exitCode != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "plan is for drupal, not store") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
	}
}

func TestInstallCommandRejectsInvalidApplyAndVerifyInputs(t *testing.T) {
	validPlanPath := writeTestPlan(t, InstallationPlan{})
	invalidPlanPath := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(invalidPlanPath, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		run  func(io.Writer, io.Writer) int
		text string
	}{
		{
			name: "apply requires plan",
			run: func(stdout, stderr io.Writer) int {
				return runApplyCommandForConfig(context.Background(), &cliTestModule{}, testDrupalConfig, nil, stdout, stderr)
			},
			text: "--plan is required",
		},
		{
			name: "apply rejects invalid plan JSON",
			run: func(stdout, stderr io.Writer) int {
				return runApplyCommandForConfig(context.Background(), &cliTestModule{}, testDrupalConfig, []string{"--plan", invalidPlanPath}, stdout, stderr)
			},
			text: "invalid_plan",
		},
		{
			name: "apply rejects event format",
			run: func(stdout, stderr io.Writer) int {
				return runApplyCommandForConfig(context.Background(), &cliTestModule{}, testDrupalConfig, []string{"--plan", validPlanPath, "--events", "xml"}, stdout, stderr)
			},
			text: "unknown event format",
		},
		{
			name: "verify requires plan",
			run: func(stdout, stderr io.Writer) int {
				return runVerifyCommandForConfig(context.Background(), &cliTestModule{}, testDrupalConfig, nil, stdout, stderr)
			},
			text: "--plan is required",
		},
		{
			name: "verify rejects output format",
			run: func(stdout, stderr io.Writer) int {
				return runVerifyCommandForConfig(context.Background(), &cliTestModule{}, testDrupalConfig, []string{"--plan", validPlanPath, "--output", "xml"}, stdout, stderr)
			},
			text: "output must be human or json",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if exitCode := test.run(&stdout, &stderr); exitCode != 2 {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), test.text) {
				t.Fatalf("stdout = %q, stderr = %q, want %q", stdout.String(), stderr.String(), test.text)
			}
		})
	}
}

func TestReadPlanRequiresExactlyOneStrictJSONDocument(t *testing.T) {
	valid := InstallationPlan{PlanID: "one"}
	validPath := writeTestPlan(t, valid)
	plan, err := readPlan(validPath)
	if err != nil || plan.PlanID != valid.PlanID {
		t.Fatalf("readPlan() = %#v, %v", plan, err)
	}

	for _, test := range []struct {
		name    string
		content string
		text    string
	}{
		{name: "unknown field", content: `{"unknown":true}`, text: "unknown field"},
		{name: "multiple documents", content: `{} {}`, text: "more than one JSON document"},
		{name: "trailing garbage", content: `{} x`, text: "invalid character"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "plan.json")
			if err := os.WriteFile(path, []byte(test.content), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := readPlan(path); err == nil || !strings.Contains(err.Error(), test.text) {
				t.Fatalf("readPlan() error = %v, want %q", err, test.text)
			}
		})
	}
}

func TestEventSinksAndHumanFormatting(t *testing.T) {
	var human bytes.Buffer
	sink, err := selectEventSink("human", &human)
	if err != nil {
		t.Fatal(err)
	}
	sink.Emit(Event{Type: "step_started", Level: "info", StepID: "runtime.start", Message: "Starting"})
	sink.Emit(Event{Type: "command_output", StepID: "runtime.start", Message: " output\n"})
	if human.String() != "[INFO] runtime.start: Starting\n[runtime.start] output\n" {
		t.Fatalf("human events = %q", human.String())
	}

	var jsonl bytes.Buffer
	sink, err = selectEventSink("jsonl", &jsonl)
	if err != nil {
		t.Fatal(err)
	}
	sink.Emit(Event{Type: "verification", Message: "present"})
	if event := decodeSingleJSON[Event](t, jsonl.Bytes()); event.Type != "verification" {
		t.Fatalf("JSONL event = %#v", event)
	}

	sink, err = selectEventSink("none", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	sink.Emit(Event{})
	if _, err := selectEventSink("xml", io.Discard); err == nil {
		t.Fatal("selectEventSink() accepted an unknown format")
	}

	plan := InstallationPlan{
		PlanID:            "abc123",
		ProjectPath:       "/projects/site",
		Steps:             []InstallationStep{{ID: "project.create", Summary: "Create project", Disposition: dispositionCreate, Effects: []Effect{effectFilesystem}, Reason: "needed"}},
		RequiredApprovals: []Effect{effectNetwork},
	}
	var output bytes.Buffer
	if err := writePlan(&output, "human", plan); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Plan abc123 for /projects/site", "project.create", "[filesystem]", "Required approvals: network"} {
		if !strings.Contains(output.String(), text) {
			t.Errorf("human plan omitted %q: %q", text, output.String())
		}
	}
	output.Reset()
	failure := installationFailure("step_failed", "project.create", "failed", true, "retry")
	if err := writeResult(&output, "human", InstallationResult{Status: "partial", ProjectPath: plan.ProjectPath, SiteURL: "https://site.test", Failure: &failure}); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Installation partial", "Project: /projects/site", "Site: https://site.test", "Failure: failed (step_failed)", "Recovery: retry"} {
		if !strings.Contains(output.String(), text) {
			t.Errorf("human result omitted %q: %q", text, output.String())
		}
	}
	if err := writePlan(io.Discard, "xml", plan); err == nil {
		t.Fatal("writePlan() accepted an unknown format")
	}
	if err := writeResult(io.Discard, "xml", InstallationResult{}); err == nil {
		t.Fatal("writeResult() accepted an unknown format")
	}
}

func TestExitCodeForFailure(t *testing.T) {
	for _, test := range []struct {
		code string
		want int
	}{
		{code: "invalid_request", want: 2},
		{code: "invalid_plan", want: 2},
		{code: "plan_blocked", want: 3},
		{code: "approval_required", want: 3},
		{code: "inspection_failed", want: 4},
		{code: "plan_stale", want: 4},
		{code: "step_failed", want: 5},
		{code: "verification_failed", want: 6},
		{code: "internal_error", want: 1},
	} {
		t.Run(test.code, func(t *testing.T) {
			if got := exitCodeForFailure(installationFailure(test.code, "", "", false, "")); got != test.want {
				t.Fatalf("exitCodeForFailure(%q) = %d, want %d", test.code, got, test.want)
			}
		})
	}
}

func TestVerifyCommandMapsFailureExitCode(t *testing.T) {
	failure := installationFailure("verification_failed", "", "missing files", true, "inspect checks")
	module := &cliTestModule{
		verifyResult: InstallationResult{SchemaVersion: installationSchemaVersion, Status: "failed", Failure: &failure},
		verifyErr:    failure,
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runVerifyCommandForConfig(context.Background(), module, testDrupalConfig, []string{
		"--plan", writeTestPlan(t, InstallationPlan{}),
		"--events", "none",
	}, &stdout, &stderr)

	if exitCode != 6 || !strings.Contains(stdout.String(), "Installation failed") || stderr.Len() != 0 {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
	}
}
