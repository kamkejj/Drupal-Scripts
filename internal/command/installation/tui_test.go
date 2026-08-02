package installation

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

type tuiTestModule struct {
	request  InstallationRequest
	plan     InstallationPlan
	approval Approval
	result   InstallationResult
	planErr  error
	applyErr error
	applyHit bool
}

func (module *tuiTestModule) Plan(_ context.Context, request InstallationRequest) (InstallationPlan, error) {
	module.request = request
	return module.plan, module.planErr
}

func (module *tuiTestModule) Apply(_ context.Context, plan InstallationPlan, approval Approval, sink EventSink) (InstallationResult, error) {
	module.applyHit = true
	module.plan = plan
	module.approval = approval
	if sink != nil {
		sink.Emit(Event{Type: "step_completed", Level: "info", StepID: "runtime.start", Message: "Runtime started"})
	}
	return module.result, module.applyErr
}

func (module *tuiTestModule) Verify(context.Context, InstallationPlan, EventSink) (InstallationResult, error) {
	return InstallationResult{}, nil
}

func tuiKey(code rune, text string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Text: text})
}

func TestInstallTUIBuildsPlanThroughGuidedFlow(t *testing.T) {
	module := &tuiTestModule{plan: InstallationPlan{PlanID: "abc123", ProjectPath: "/projects/quick-site"}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := newInstallTUIModel(ctx, cancel, module, testDrupalConfig, "/projects")

	model.Update(tuiKey(tea.KeyEnter, ""))
	if !strings.Contains(model.View().Content, "Choose a Drupal version") {
		t.Fatalf("version view = %q", model.View().Content)
	}
	model.Update(tuiKey(tea.KeyDown, ""))
	model.Update(tuiKey(tea.KeyEnter, ""))
	model.Update(tuiKey('q', "q"))
	model.Update(tuiKey('u', "u"))
	model.Update(tuiKey('i', "i"))
	model.Update(tuiKey('c', "c"))
	model.Update(tuiKey('k', "k"))
	model.Update(tuiKey('-', "-"))
	model.Update(tuiKey('s', "s"))
	model.Update(tuiKey('i', "i"))
	model.Update(tuiKey('t', "t"))
	model.Update(tuiKey('e', "e"))
	model.Update(tuiKey(tea.KeyEnter, ""))
	model.Update(tuiKey(' ', " "))
	_, command := model.Update(tuiKey(tea.KeyEnter, ""))

	if model.stage != installTUIPlanning {
		t.Fatalf("stage = %d, want planning", model.stage)
	}
	commands, ok := command().(tea.BatchMsg)
	if !ok || len(commands) != 2 {
		t.Fatalf("planning command = %#v", command())
	}
	message, ok := commands[0]().(installPlanMsg)
	if !ok {
		t.Fatalf("plan message = %#v", commands[0]())
	}
	model.Update(message)

	if module.request.ProjectName != "quick-site" || module.request.ParentDirectory != "/projects" {
		t.Fatalf("request = %#v", module.request)
	}
	if module.request.DockerProvider != colima || module.request.DrupalVersion != 12 || !module.request.GenerateContent || module.request.AdminUsername != "admin" {
		t.Fatalf("request = %#v", module.request)
	}
	if module.request.AdminPasswordEnv != "" {
		t.Fatalf("password environment = %q, want default password behavior", module.request.AdminPasswordEnv)
	}
	if model.stage != installTUIReview {
		t.Fatalf("stage = %d, want review", model.stage)
	}
}

func TestConfiguredTUIOffersConfiguredDrupalVersions(t *testing.T) {
	module := &tuiTestModule{plan: InstallationPlan{PlanID: "store-plan"}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := newInstallTUIModel(ctx, cancel, module, testExtensionConfig, "/projects")
	model.stage = installTUIVersion

	view := model.View().Content
	if !strings.Contains(view, "Drupal 10") || !strings.Contains(view, "Drupal 11") || strings.Contains(view, "Drupal 12") {
		t.Fatalf("configured version view = %q", view)
	}
	model.drupalVersion = testExtensionConfig.MinimumDrupalVersion
	model.Update(tuiKey(tea.KeyUp, ""))
	if model.drupalVersion != testExtensionConfig.MaximumDrupalVersion {
		t.Fatalf("wrapped version = %d", model.drupalVersion)
	}
	model.projectName = "store"
	message := model.planCommand()().(installPlanMsg)
	model.Update(message)
	if module.request.InstallationType != testExtensionConfig.Type || module.request.DrupalVersion != 11 {
		t.Fatalf("request = %#v", module.request)
	}
}

func TestInstallTUIRequiresExplicitApplyConfirmation(t *testing.T) {
	plan := InstallationPlan{
		PlanID:            "abc123",
		Digest:            "digest",
		ProjectPath:       "/projects/site",
		RequiredApprovals: []Effect{effectNetwork, effectHostChange},
		Steps: []InstallationStep{{
			ID:          "runtime.start",
			Summary:     "Start the container runtime",
			Disposition: dispositionModify,
		}},
	}
	module := &tuiTestModule{plan: plan, result: InstallationResult{Status: "succeeded", ProjectPath: plan.ProjectPath}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := newInstallTUIModel(ctx, cancel, module, testDrupalConfig, "/projects")
	model.stage = installTUIReview
	model.plan = plan
	model.send = func(message tea.Msg) {
		model.Update(message)
	}

	view := model.View().Content
	if !strings.Contains(view, "Authorization required") || !strings.Contains(view, "Enter authorizes") {
		t.Fatalf("review view omitted authorization contract: %q", view)
	}
	if module.applyHit {
		t.Fatal("apply ran before confirmation")
	}
	_, command := model.Update(tuiKey(tea.KeyEnter, ""))
	commands := command().(tea.BatchMsg)
	message := commands[0]()
	model.Update(message)

	if !module.applyHit {
		t.Fatal("apply did not run after confirmation")
	}
	if module.approval.PlanDigest != plan.Digest || !module.approval.AllowedEffects[effectNetwork] || !module.approval.AllowedEffects[effectHostChange] {
		t.Fatalf("approval = %#v", module.approval)
	}
	if model.stage != installTUIFinished || model.result.Status != "succeeded" {
		t.Fatalf("model = %#v", model)
	}
}

func TestInstallTUIBlockedPlanCannotApply(t *testing.T) {
	module := &tuiTestModule{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := newInstallTUIModel(ctx, cancel, module, testDrupalConfig, "/projects")
	model.stage = installTUIReview
	model.plan = InstallationPlan{Blocked: true}

	_, command := model.Update(tuiKey(tea.KeyEnter, ""))

	if command != nil || module.applyHit {
		t.Fatal("blocked plan was applyable")
	}
	if !strings.Contains(model.View().Content, "blocked and cannot be applied") {
		t.Fatalf("blocked view = %q", model.View().Content)
	}
}

func TestInstallTUIShowsPlanningFailureRecovery(t *testing.T) {
	module := &tuiTestModule{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := newInstallTUIModel(ctx, cancel, module, testDrupalConfig, "/projects")
	model.stage = installTUIReview
	model.err = installationFailure("invalid_request", "", "bad project", false, "choose another name")

	view := model.View().Content

	if !strings.Contains(view, "bad project") || !strings.Contains(view, "choose another name") {
		t.Fatalf("failure view = %q", view)
	}
}

func TestInstallTUIFinishedHandlesMissingStructuredFailure(t *testing.T) {
	module := &tuiTestModule{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := newInstallTUIModel(ctx, cancel, module, testDrupalConfig, "/projects")
	model.stage = installTUIFinished
	model.result = InstallationResult{Status: "failed"}

	view := model.View().Content

	if !strings.Contains(view, "installation did not complete") {
		t.Fatalf("failure view = %q", view)
	}
}

func TestInstallTUIPlanErrorMessage(t *testing.T) {
	module := &tuiTestModule{planErr: errors.New("inspection unavailable")}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := newInstallTUIModel(ctx, cancel, module, testDrupalConfig, "/projects")
	message := model.planCommand()().(installPlanMsg)

	if message.err == nil || message.err.Error() != "inspection unavailable" {
		t.Fatalf("plan error = %v", message.err)
	}
}

func TestInstallTUIHandlesEditingCancellationAndBusyUpdates(t *testing.T) {
	module := &tuiTestModule{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := newInstallTUIModel(ctx, cancel, module, testDrupalConfig, "/projects")
	if model.Init() != nil {
		t.Fatal("Init() returned a command")
	}

	model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if model.width != 120 || model.height != 40 {
		t.Fatalf("window = %dx%d", model.width, model.height)
	}
	model.stage = installTUIProject
	model.Update(tea.PasteMsg{Content: "café\nsite"})
	if model.projectName != "cafésite" {
		t.Fatalf("pasted project = %q", model.projectName)
	}
	model.Update(tuiKey(tea.KeyBackspace, ""))
	model.Update(tuiKey(tea.KeyBackspace, ""))
	model.Update(tuiKey(tea.KeyBackspace, ""))
	model.Update(tuiKey(tea.KeyBackspace, ""))
	if model.projectName != "café" {
		t.Fatalf("edited project = %q", model.projectName)
	}

	model.stage = installTUIPlanning
	_, command := model.Update(installTickMsg{})
	if model.spinner != 1 || command == nil {
		t.Fatalf("spinner = %d, command = %#v", model.spinner, command)
	}
	for sequence := 1; sequence <= 10; sequence++ {
		model.Update(installEventMsg(Event{Sequence: sequence, Type: "step_finished", Message: "event"}))
	}
	if len(model.events) != 8 || model.events[0].Sequence != 3 || model.status != "event" {
		t.Fatalf("events = %#v, status = %q", model.events, model.status)
	}

	_, command = model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	if !model.cancelled || command == nil {
		t.Fatalf("cancelled = %t, command = %#v", model.cancelled, command)
	}
}

func TestInstallTUIProjectValidationAndNavigation(t *testing.T) {
	module := &tuiTestModule{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := newInstallTUIModel(ctx, cancel, module, testDrupalConfig, "/projects")

	model.Update(tuiKey(tea.KeyDown, ""))
	if model.provider != dockerDesktop {
		t.Fatalf("provider = %q", model.provider)
	}
	model.Update(tuiKey(tea.KeyEnter, ""))
	model.drupalVersion = minimumDrupalVersion
	model.Update(tuiKey(tea.KeyUp, ""))
	if model.drupalVersion != maximumDrupalVersion {
		t.Fatalf("wrapped version = %d", model.drupalVersion)
	}
	model.Update(tuiKey(tea.KeyEnter, ""))
	_, command := model.Update(tuiKey(tea.KeyEnter, ""))
	if command != nil || model.stage != installTUIProject || model.status != "Project name is required" {
		t.Fatalf("stage = %d, status = %q, command = %#v", model.stage, model.status, command)
	}
	model.Update(tuiKey('q', "q"))
	if model.projectName != "q" || model.cancelled {
		t.Fatalf("project = %q, cancelled = %t", model.projectName, model.cancelled)
	}
	model.Update(tuiKey(tea.KeyEscape, ""))
	if !model.cancelled {
		t.Fatal("escape did not cancel project entry")
	}
}

func TestInstallTUIRendersEveryStage(t *testing.T) {
	module := &tuiTestModule{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := newInstallTUIModel(ctx, cancel, module, testDrupalConfig, "/projects")
	model.projectName = "site"
	model.plan = InstallationPlan{
		PlanID:      "abc123",
		ProjectPath: "/projects/site",
		Request:     InstallationRequest{DrupalVersion: 11},
		Steps:       []InstallationStep{{ID: "project.create", Summary: strings.Repeat("create project ", 10), Disposition: dispositionCreate}},
	}
	model.status = "working"
	model.events = []Event{
		{Type: "command_output", Level: "info", StepID: "project.create", Message: "composer output"},
		{Type: "step_finished", Level: "error", StepID: "project.create", Message: "failed"},
	}
	model.result = InstallationResult{Status: "succeeded", ProjectPath: "/projects/site", SiteURL: "https://site.ddev.site"}

	tests := []struct {
		stage installTUIStage
		text  string
	}{
		{stage: installTUIProvider, text: "Choose a container runtime"},
		{stage: installTUIVersion, text: "Choose a Drupal version"},
		{stage: installTUIProject, text: "Name your Drupal project"},
		{stage: installTUIContent, text: "Generate sample content?"},
		{stage: installTUIPlanning, text: "Planning is read-only"},
		{stage: installTUIReview, text: "Installation plan"},
		{stage: installTUIApplying, text: "Installing Drupal 11"},
		{stage: installTUIFinished, text: "Drupal 11 is ready"},
	}
	for _, test := range tests {
		model.stage = test.stage
		view := model.View()
		if !strings.Contains(view.Content, test.text) || !view.AltScreen || view.WindowTitle != "Dropkit" {
			t.Errorf("stage %d view = %#v, want %q", test.stage, view, test.text)
		}
	}
}

func TestInstallTUIFinishedPrefersStructuredFailure(t *testing.T) {
	module := &tuiTestModule{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := newInstallTUIModel(ctx, cancel, module, testDrupalConfig, "/projects")
	model.stage = installTUIFinished
	model.err = errors.New("generic error")
	failure := installationFailure("step_failed", "drupal.site", "site install failed", true, "retry the site install")
	model.result = InstallationResult{Status: "partial", Failure: &failure}

	view := model.View().Content

	for _, text := range []string{"site install failed", "Step: drupal.site", "retry the site install"} {
		if !strings.Contains(view, text) {
			t.Errorf("failure view omitted %q: %q", text, view)
		}
	}
	_, command := model.Update(tuiKey(tea.KeyEnter, ""))
	if command == nil {
		t.Fatal("finished view did not quit on Enter")
	}
}

func TestTUILayoutHelpersBoundAndTrimContent(t *testing.T) {
	model := &installTUIModel{width: 20}
	if got := model.contentWidth(); got != 42 {
		t.Fatalf("small content width = %d", got)
	}
	model.width = 200
	if got := model.contentWidth(); got != 100 {
		t.Fatalf("large content width = %d", got)
	}
	model.width = 70
	if got := model.contentWidth(); got != 64 {
		t.Fatalf("content width = %d", got)
	}

	for _, test := range []struct {
		name  string
		value string
		width int
		want  string
	}{
		{name: "newline", value: "one\ntwo", width: 20, want: "one two"},
		{name: "runes", value: "café-site", width: 5, want: "café…"},
		{name: "tiny", value: "unchanged", width: 1, want: "unchanged"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := trimForWidth(test.value, test.width); got != test.want {
				t.Fatalf("trimForWidth(%q, %d) = %q, want %q", test.value, test.width, got, test.want)
			}
		})
	}
}
