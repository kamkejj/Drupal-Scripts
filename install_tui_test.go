package main

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
	model := newInstallTUIModel(ctx, cancel, module, "/projects")

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
	model := newInstallTUIModel(ctx, cancel, module, "/projects")
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
	model := newInstallTUIModel(ctx, cancel, module, "/projects")
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
	model := newInstallTUIModel(ctx, cancel, module, "/projects")
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
	model := newInstallTUIModel(ctx, cancel, module, "/projects")
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
	model := newInstallTUIModel(ctx, cancel, module, "/projects")
	message := model.planCommand()().(installPlanMsg)

	if message.err == nil || message.err.Error() != "inspection unavailable" {
		t.Fatalf("plan error = %v", message.err)
	}
}
