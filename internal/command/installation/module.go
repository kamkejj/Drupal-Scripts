package installation

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

//go:embed config/environment_indicator.indicator.yml
var configIndicatorYML string

//go:embed config/environment_indicator.settings.yml
var configSettingsYML string

const (
	installationSchemaVersion = "4"
	defaultDrupalVersion      = 11
	minimumDrupalVersion      = 8
	maximumDrupalVersion      = 12
)

type InstallationType string

type InstallationConfig struct {
	CommandName          string
	Type                 InstallationType
	ProductName          string
	MinimumDrupalVersion int
	MaximumDrupalVersion int
	ComposerPackages     []string
	EnabledModules       []string
}

func (config InstallationConfig) dependencyStepID() string {
	return string(config.Type) + ".dependencies"
}

func (config InstallationConfig) modulesStepID() string {
	return string(config.Type) + ".modules"
}

type DockerProvider string

const (
	dockerDesktop DockerProvider = "docker"
	colima        DockerProvider = "colima"
)

type Effect string

const (
	effectFilesystem  Effect = "filesystem"
	effectProcess     Effect = "process"
	effectNetwork     Effect = "network"
	effectHostChange  Effect = "host_change"
	effectDestructive Effect = "destructive"
)

type Disposition string

const (
	dispositionNoOp    Disposition = "noop"
	dispositionCreate  Disposition = "create"
	dispositionModify  Disposition = "modify"
	dispositionBlocked Disposition = "blocked"
)

type RetrySemantics string

const (
	retrySafe      RetrySemantics = "safe"
	retryReconcile RetrySemantics = "reconcile"
	retryManual    RetrySemantics = "manual"
)

type InstallationRequest struct {
	SchemaVersion    string           `json:"schema_version"`
	InstallationType InstallationType `json:"installation_type"`
	ProjectName      string           `json:"project_name"`
	ParentDirectory  string           `json:"parent_directory"`
	DockerProvider   DockerProvider   `json:"docker_provider"`
	DrupalVersion    int              `json:"drupal_version"`
	GenerateContent  bool             `json:"generate_content"`
	AdminUsername    string           `json:"admin_username"`
	AdminPasswordEnv string           `json:"admin_password_env,omitempty"`
}

type ToolState struct {
	Installed bool   `json:"installed"`
	Path      string `json:"path,omitempty"`
}

type HostInspection struct {
	Platform       string               `json:"platform"`
	Architecture   string               `json:"architecture"`
	Tools          map[string]ToolState `json:"tools"`
	RuntimeRunning bool                 `json:"runtime_running"`
	TargetState    string               `json:"target_state"`
	Fingerprint    string               `json:"fingerprint"`
}

type InstallationStep struct {
	ID          string         `json:"id"`
	Summary     string         `json:"summary"`
	DependsOn   []string       `json:"depends_on,omitempty"`
	Disposition Disposition    `json:"disposition"`
	Effects     []Effect       `json:"effects,omitempty"`
	Retry       RetrySemantics `json:"retry"`
	Reason      string         `json:"reason,omitempty"`
}

type InstallationPlan struct {
	SchemaVersion     string                `json:"schema_version"`
	PlanID            string                `json:"plan_id"`
	Digest            string                `json:"digest"`
	Request           InstallationRequest   `json:"request"`
	ProjectPath       string                `json:"project_path"`
	Inspection        HostInspection        `json:"inspection"`
	Steps             []InstallationStep    `json:"steps"`
	RequiredApprovals []Effect              `json:"required_approvals,omitempty"`
	Blocked           bool                  `json:"blocked"`
	Blockers          []InstallationFailure `json:"blockers,omitempty"`
}

type Approval struct {
	PlanDigest     string
	AllowedEffects map[Effect]bool
}

type StepResult struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code,omitempty"`
	Message  string `json:"message,omitempty"`
}

type VerificationCheck struct {
	ID      string `json:"id"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

type InstallationFailure struct {
	Code      string `json:"code"`
	StepID    string `json:"step_id,omitempty"`
	Message   string `json:"message"`
	ExitCode  *int   `json:"exit_code,omitempty"`
	Retryable bool   `json:"retryable"`
	Recovery  string `json:"recovery,omitempty"`
}

func (failure InstallationFailure) Error() string {
	return failure.Message
}

type InstallationResult struct {
	SchemaVersion string               `json:"schema_version"`
	PlanID        string               `json:"plan_id,omitempty"`
	Status        string               `json:"status"`
	ProjectPath   string               `json:"project_path,omitempty"`
	SiteURL       string               `json:"site_url,omitempty"`
	Steps         []StepResult         `json:"steps,omitempty"`
	Verification  []VerificationCheck  `json:"verification,omitempty"`
	Failure       *InstallationFailure `json:"failure,omitempty"`
}

type Event struct {
	Sequence int               `json:"sequence"`
	Time     string            `json:"time"`
	Type     string            `json:"type"`
	Level    string            `json:"level"`
	StepID   string            `json:"step_id,omitempty"`
	Message  string            `json:"message"`
	Fields   map[string]string `json:"fields,omitempty"`
}

type EventSink interface {
	Emit(Event)
}

type discardEventSink struct{}

func (discardEventSink) Emit(Event) {}

type CommandRequest struct {
	Name string
	Args []string
	Dir  string
}

type CommandResult struct {
	ExitCode int
	Output   string
	Err      error
}

type CommandRunner interface {
	LookPath(string) (string, error)
	Run(context.Context, CommandRequest) CommandResult
}

type execCommandRunner struct{}

func (execCommandRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (execCommandRunner) Run(ctx context.Context, request CommandRequest) CommandResult {
	command := exec.CommandContext(ctx, request.Name, request.Args...)
	command.Dir = request.Dir
	output, err := command.CombinedOutput()
	result := CommandResult{Output: string(output), Err: err}
	if err == nil {
		return result
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		result.ExitCode = exitError.ExitCode()
	} else {
		result.ExitCode = -1
	}
	return result
}

type FileSystem interface {
	Abs(string) (string, error)
	Stat(string) (fs.FileInfo, error)
	ReadFile(string) ([]byte, error)
	MkdirAll(string, fs.FileMode) error
	WriteFile(string, []byte, fs.FileMode) error
}

type osFileSystem struct{}

func (osFileSystem) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

func (osFileSystem) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

func (osFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (osFileSystem) MkdirAll(path string, mode fs.FileMode) error {
	return os.MkdirAll(path, mode)
}

func (osFileSystem) WriteFile(path string, content []byte, mode fs.FileMode) error {
	return os.WriteFile(path, content, mode)
}

type InstallationModule interface {
	Plan(context.Context, InstallationRequest) (InstallationPlan, error)
	Apply(context.Context, InstallationPlan, Approval, EventSink) (InstallationResult, error)
	Verify(context.Context, InstallationPlan, EventSink) (InstallationResult, error)
}

type installationModule struct {
	runner       CommandRunner
	files        FileSystem
	config       InstallationConfig
	platform     string
	architecture string
}

func newInstallationModule(runner CommandRunner, files FileSystem, config InstallationConfig) InstallationModule {
	return &installationModule{runner: runner, files: files, config: config, platform: runtime.GOOS, architecture: runtime.GOARCH}
}

func newProductionInstallationModule(config InstallationConfig) InstallationModule {
	return newInstallationModule(execCommandRunner{}, osFileSystem{}, config)
}

func (module *installationModule) Plan(ctx context.Context, request InstallationRequest) (InstallationPlan, error) {
	normalized, projectPath, err := module.normalizeRequest(request)
	if err != nil {
		return InstallationPlan{}, err
	}
	inspection, err := module.inspect(ctx, normalized, projectPath)
	if err != nil {
		return InstallationPlan{}, err
	}
	plan := InstallationPlan{
		SchemaVersion: installationSchemaVersion,
		Request:       normalized,
		ProjectPath:   projectPath,
		Inspection:    inspection,
	}
	plan.Steps = module.buildSteps(normalized, inspection)
	approvals := map[Effect]bool{}
	for _, step := range plan.Steps {
		if step.Disposition == dispositionBlocked {
			plan.Blocked = true
			plan.Blockers = append(plan.Blockers, InstallationFailure{
				Code:     "plan_blocked",
				StepID:   step.ID,
				Message:  step.Reason,
				Recovery: "resolve the blocker and create a new plan",
			})
		}
		if step.Disposition == dispositionNoOp || step.Disposition == dispositionBlocked {
			continue
		}
		for _, effect := range step.Effects {
			if effect == effectNetwork || effect == effectHostChange || effect == effectDestructive {
				approvals[effect] = true
			}
		}
	}
	for effect := range approvals {
		plan.RequiredApprovals = append(plan.RequiredApprovals, effect)
	}
	sort.Slice(plan.RequiredApprovals, func(i, j int) bool {
		return plan.RequiredApprovals[i] < plan.RequiredApprovals[j]
	})
	digest, err := planDigest(plan)
	if err != nil {
		return InstallationPlan{}, installationFailure("internal_error", "", err.Error(), false, "")
	}
	plan.Digest = digest
	plan.PlanID = digest[:12]
	return plan, nil
}

func (module *installationModule) normalizeRequest(request InstallationRequest) (InstallationRequest, string, error) {
	request.SchemaVersion = installationSchemaVersion
	if request.InstallationType == "" {
		request.InstallationType = module.config.Type
	}
	if request.InstallationType != module.config.Type {
		return InstallationRequest{}, "", installationFailure("invalid_request", "", fmt.Sprintf("installation type must be %s", module.config.Type), false, "use the command that matches the requested installation type")
	}
	request.ProjectName = strings.ToLower(strings.TrimSpace(request.ProjectName))
	request.ProjectName = strings.ReplaceAll(request.ProjectName, " ", "-")
	if request.ProjectName == "" {
		return InstallationRequest{}, "", installationFailure("invalid_request", "", "project name is required", false, "provide --name")
	}
	for index, char := range request.ProjectName {
		valid := char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-'
		if !valid || (index == 0 && char == '-') {
			return InstallationRequest{}, "", installationFailure("invalid_request", "", "project name must contain only lowercase letters, numbers, and hyphens", false, "choose a valid project name")
		}
	}
	if strings.HasSuffix(request.ProjectName, "-") {
		return InstallationRequest{}, "", installationFailure("invalid_request", "", "project name cannot end with a hyphen", false, "choose a valid project name")
	}
	if request.DockerProvider != dockerDesktop && request.DockerProvider != colima {
		return InstallationRequest{}, "", installationFailure("invalid_request", "", "docker provider must be docker or colima", false, "provide --provider docker or --provider colima")
	}
	if request.DrupalVersion < minimumDrupalVersion || request.DrupalVersion > maximumDrupalVersion {
		return InstallationRequest{}, "", installationFailure("invalid_request", "", "Drupal version must be between 8 and 12", false, "provide --drupal-version with a major version from 8 through 12")
	}
	if request.DrupalVersion < module.config.MinimumDrupalVersion || request.DrupalVersion > module.config.MaximumDrupalVersion {
		return InstallationRequest{}, "", installationFailure("invalid_request", "", fmt.Sprintf("%s supports Drupal %s", module.config.ProductName, versionRange(module.config.MinimumDrupalVersion, module.config.MaximumDrupalVersion)), false, fmt.Sprintf("provide --drupal-version %s", versionChoices(module.config.MinimumDrupalVersion, module.config.MaximumDrupalVersion)))
	}
	if strings.TrimSpace(request.ParentDirectory) == "" {
		return InstallationRequest{}, "", installationFailure("invalid_request", "", "parent directory is required", false, "provide --parent")
	}
	parent, err := module.files.Abs(request.ParentDirectory)
	if err != nil {
		return InstallationRequest{}, "", installationFailure("invalid_request", "", "could not resolve parent directory", false, "provide an accessible parent directory")
	}
	request.ParentDirectory = filepath.Clean(parent)
	if request.AdminUsername == "" {
		request.AdminUsername = "admin"
	}
	projectPath := filepath.Join(request.ParentDirectory, request.ProjectName)
	return request, projectPath, nil
}

func (module *installationModule) inspect(ctx context.Context, request InstallationRequest, projectPath string) (HostInspection, error) {
	inspection := HostInspection{
		Platform:     module.platform,
		Architecture: module.architecture,
		Tools:        map[string]ToolState{},
		TargetState:  "absent",
	}
	for _, tool := range []string{"brew", "docker", "colima", "ddev", "composer"} {
		path, err := module.runner.LookPath(tool)
		inspection.Tools[tool] = ToolState{Installed: err == nil, Path: path}
	}
	if request.DockerProvider == colima && inspection.Tools["colima"].Installed {
		inspection.RuntimeRunning = module.runner.Run(ctx, CommandRequest{Name: "colima", Args: []string{"status"}}).Err == nil
	}
	if request.DockerProvider == dockerDesktop && inspection.Tools["docker"].Installed {
		inspection.RuntimeRunning = module.runner.Run(ctx, CommandRequest{Name: "docker", Args: []string{"info"}}).Err == nil
	}
	info, err := module.files.Stat(projectPath)
	if err == nil {
		if info.IsDir() {
			inspection.TargetState = "directory"
			if _, composerErr := module.files.Stat(filepath.Join(projectPath, "composer.json")); composerErr == nil {
				inspection.TargetState = "project"
			}
		} else {
			inspection.TargetState = "file"
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return HostInspection{}, installationFailure("inspection_failed", "", "could not inspect project target", true, "check target directory permissions")
	}
	fingerprint, err := inspectionFingerprint(inspection)
	if err != nil {
		return HostInspection{}, installationFailure("internal_error", "", err.Error(), false, "")
	}
	inspection.Fingerprint = fingerprint
	return inspection, nil
}

func (module *installationModule) buildSteps(request InstallationRequest, inspection HostInspection) []InstallationStep {
	steps := []InstallationStep{}
	appendStep := func(step InstallationStep) {
		steps = append(steps, step)
	}
	if inspection.Platform != "darwin" {
		appendStep(InstallationStep{ID: "host.platform", Summary: "Use a supported macOS host", Disposition: dispositionBlocked, Retry: retryManual, Reason: "Dropkit currently supports macOS only"})
	} else {
		appendStep(InstallationStep{ID: "host.platform", Summary: "Verify supported macOS host", Disposition: dispositionNoOp, Retry: retrySafe})
	}
	if !inspection.Tools["brew"].Installed {
		appendStep(InstallationStep{ID: "host.homebrew", Summary: "Install Homebrew", DependsOn: []string{"host.platform"}, Disposition: dispositionBlocked, Retry: retryManual, Reason: "Homebrew must be installed before Dropkit can manage host tools"})
	} else {
		appendStep(InstallationStep{ID: "host.homebrew", Summary: "Verify Homebrew", DependsOn: []string{"host.platform"}, Disposition: dispositionNoOp, Retry: retrySafe})
	}
	providerTool := "docker"
	providerSummary := "Docker Desktop"
	if request.DockerProvider == colima {
		providerTool = "colima"
		providerSummary = "Colima"
	}
	if inspection.Tools[providerTool].Installed {
		appendStep(InstallationStep{ID: "runtime.install", Summary: "Verify " + providerSummary, DependsOn: []string{"host.homebrew"}, Disposition: dispositionNoOp, Retry: retrySafe})
	} else {
		appendStep(InstallationStep{ID: "runtime.install", Summary: "Install " + providerSummary, DependsOn: []string{"host.homebrew"}, Disposition: dispositionCreate, Effects: []Effect{effectProcess, effectNetwork, effectHostChange}, Retry: retryReconcile})
	}
	if inspection.RuntimeRunning {
		appendStep(InstallationStep{ID: "runtime.start", Summary: "Verify container runtime", DependsOn: []string{"runtime.install"}, Disposition: dispositionNoOp, Retry: retrySafe})
	} else if request.DockerProvider == dockerDesktop {
		appendStep(InstallationStep{ID: "runtime.start", Summary: "Start Docker Desktop", DependsOn: []string{"runtime.install"}, Disposition: dispositionBlocked, Retry: retryManual, Reason: "Docker Desktop must be started before applying this plan"})
	} else {
		appendStep(InstallationStep{ID: "runtime.start", Summary: "Start Colima", DependsOn: []string{"runtime.install"}, Disposition: dispositionModify, Effects: []Effect{effectProcess, effectHostChange}, Retry: retrySafe})
	}
	appendToolStep := func(id, tool, summary, formula string) {
		if inspection.Tools[tool].Installed {
			appendStep(InstallationStep{ID: id, Summary: "Verify " + summary, DependsOn: []string{"host.homebrew"}, Disposition: dispositionNoOp, Retry: retrySafe})
			return
		}
		appendStep(InstallationStep{ID: id, Summary: "Install " + summary, DependsOn: []string{"host.homebrew"}, Disposition: dispositionCreate, Effects: []Effect{effectProcess, effectNetwork, effectHostChange}, Retry: retryReconcile, Reason: formula})
	}
	appendToolStep("host.composer", "composer", "Composer", "composer")
	appendToolStep("host.ddev", "ddev", "DDEV", "ddev/ddev/ddev")
	projectDisposition := dispositionCreate
	projectReason := ""
	if inspection.TargetState == "project" {
		projectDisposition = dispositionNoOp
	} else if inspection.TargetState != "absent" {
		projectDisposition = dispositionBlocked
		projectReason = "the project target already exists"
	}
	appendStep(InstallationStep{ID: "project.create", Summary: fmt.Sprintf("Create Drupal %d project", request.DrupalVersion), DependsOn: []string{"host.composer"}, Disposition: projectDisposition, Effects: []Effect{effectFilesystem, effectProcess, effectNetwork}, Retry: retryReconcile, Reason: projectReason})
	appendStep(InstallationStep{ID: "ddev.configure", Summary: fmt.Sprintf("Configure DDEV for Drupal %d", request.DrupalVersion), DependsOn: []string{"project.create", "host.ddev", "runtime.start"}, Disposition: dispositionCreate, Effects: []Effect{effectFilesystem, effectProcess}, Retry: retryReconcile})
	appendStep(InstallationStep{ID: "ddev.start", Summary: "Start DDEV project", DependsOn: []string{"ddev.configure"}, Disposition: dispositionModify, Effects: []Effect{effectProcess}, Retry: retrySafe})
	appendStep(InstallationStep{ID: "drupal.dependencies", Summary: "Install Drupal dependencies", DependsOn: []string{"ddev.start"}, Disposition: dispositionModify, Effects: []Effect{effectFilesystem, effectProcess, effectNetwork}, Retry: retryReconcile})
	appendStep(InstallationStep{ID: "drupal.settings", Summary: "Write Drupal development settings", DependsOn: []string{"drupal.dependencies"}, Disposition: dispositionModify, Effects: []Effect{effectFilesystem}, Retry: retrySafe})
	appendStep(InstallationStep{ID: "drupal.site", Summary: "Install Drupal site", DependsOn: []string{"drupal.settings"}, Disposition: dispositionCreate, Effects: []Effect{effectFilesystem, effectProcess}, Retry: retryReconcile})
	appendStep(InstallationStep{ID: "drupal.modules", Summary: "Enable development modules", DependsOn: []string{"drupal.site"}, Disposition: dispositionModify, Effects: []Effect{effectFilesystem, effectProcess}, Retry: retrySafe})
	appendStep(InstallationStep{ID: "drupal.config", Summary: "Import Drupal configuration", DependsOn: []string{"drupal.modules"}, Disposition: dispositionModify, Effects: []Effect{effectFilesystem, effectProcess}, Retry: retrySafe})
	if request.GenerateContent {
		appendStep(InstallationStep{ID: "drupal.sample_content", Summary: "Replace generated sample content", DependsOn: []string{"drupal.config"}, Disposition: dispositionModify, Effects: []Effect{effectFilesystem, effectProcess, effectDestructive}, Retry: retryManual})
	} else {
		appendStep(InstallationStep{ID: "drupal.sample_content", Summary: "Skip sample content", DependsOn: []string{"drupal.config"}, Disposition: dispositionNoOp, Retry: retrySafe})
	}
	lastStepID := "drupal.sample_content"
	if len(module.config.ComposerPackages) > 0 {
		lastStepID = module.config.dependencyStepID()
		appendStep(InstallationStep{ID: lastStepID, Summary: "Install " + module.config.ProductName, DependsOn: []string{"drupal.sample_content"}, Disposition: dispositionModify, Effects: []Effect{effectFilesystem, effectProcess, effectNetwork}, Retry: retryReconcile})
	}
	if len(module.config.EnabledModules) > 0 {
		appendStep(InstallationStep{ID: module.config.modulesStepID(), Summary: "Enable " + module.config.ProductName + " modules", DependsOn: []string{lastStepID}, Disposition: dispositionModify, Effects: []Effect{effectFilesystem, effectProcess}, Retry: retrySafe})
	}
	return steps
}

func (module *installationModule) Apply(ctx context.Context, plan InstallationPlan, approval Approval, sink EventSink) (InstallationResult, error) {
	if sink == nil {
		sink = discardEventSink{}
	}
	result := InstallationResult{SchemaVersion: installationSchemaVersion, PlanID: plan.PlanID, Status: "failed", ProjectPath: plan.ProjectPath}
	if failure := module.validatePlan(plan); failure != nil {
		result.Failure = failure
		return result, *failure
	}
	if plan.Blocked {
		failure := installationFailure("plan_blocked", "", "installation plan contains blockers", false, "resolve blockers and create a new plan")
		result.Failure = &failure
		return result, failure
	}
	if approval.PlanDigest != plan.Digest {
		failure := installationFailure("approval_required", "", "approval does not match this plan", false, "approve the current plan digest")
		result.Failure = &failure
		return result, failure
	}
	for _, effect := range plan.RequiredApprovals {
		if !approval.AllowedEffects[effect] {
			failure := installationFailure("approval_required", "", fmt.Sprintf("approval for %s is required", effect), false, "review the plan and explicitly allow the effect")
			result.Failure = &failure
			return result, failure
		}
	}
	inspection, err := module.inspect(ctx, plan.Request, plan.ProjectPath)
	if err != nil {
		failure := failureFromError(err)
		result.Failure = &failure
		return result, err
	}
	if inspection.Fingerprint != plan.Inspection.Fingerprint {
		failure := installationFailure("plan_stale", "", "host or target state changed after planning", true, "create and review a new plan")
		result.Failure = &failure
		return result, failure
	}
	expectedPlan, err := module.Plan(ctx, plan.Request)
	if err != nil {
		failure := failureFromError(err)
		result.Failure = &failure
		return result, err
	}
	if expectedPlan.Digest != plan.Digest {
		failure := installationFailure("invalid_plan", "", "installation plan does not match Dropkit's plan for this host", false, "create a new plan")
		result.Failure = &failure
		return result, failure
	}
	sequence := 0
	emit := func(event Event) {
		sequence++
		event.Sequence = sequence
		event.Time = time.Now().UTC().Format(time.RFC3339Nano)
		sink.Emit(event)
	}
	for _, step := range plan.Steps {
		if step.Disposition == dispositionNoOp {
			result.Steps = append(result.Steps, StepResult{ID: step.ID, Status: "skipped", Message: "already satisfied"})
			continue
		}
		emit(Event{Type: "step_started", Level: "info", StepID: step.ID, Message: step.Summary})
		commandResult, stepErr := module.applyStep(ctx, plan, step, emit)
		if stepErr != nil {
			failure := installationFailure("step_failed", step.ID, stepErr.Error(), step.Retry != retryManual, "create a new plan after resolving the failure")
			if commandResult.ExitCode != 0 {
				failure.ExitCode = &commandResult.ExitCode
			}
			result.Steps = append(result.Steps, StepResult{ID: step.ID, Status: "failed", ExitCode: failure.ExitCode, Message: failure.Message})
			result.Status = "partial"
			result.Failure = &failure
			emit(Event{Type: "step_finished", Level: "error", StepID: step.ID, Message: failure.Message})
			return result, failure
		}
		result.Steps = append(result.Steps, StepResult{ID: step.ID, Status: "succeeded"})
		emit(Event{Type: "step_finished", Level: "info", StepID: step.ID, Message: "completed"})
	}
	verified, verifyErr := module.Verify(ctx, plan, sink)
	result.Verification = verified.Verification
	result.SiteURL = verified.SiteURL
	if verifyErr != nil {
		result.Failure = verified.Failure
		return result, verifyErr
	}
	result.Status = "succeeded"
	return result, nil
}

func (module *installationModule) applyStep(ctx context.Context, plan InstallationPlan, step InstallationStep, emit func(Event)) (CommandResult, error) {
	run := func(name string, args []string, dir string) (CommandResult, error) {
		result := module.runner.Run(ctx, CommandRequest{Name: name, Args: args, Dir: dir})
		if strings.TrimSpace(result.Output) != "" {
			emit(Event{Type: "command_output", Level: "info", StepID: step.ID, Message: redactOutput(result.Output, plan.Request)})
		}
		return result, result.Err
	}
	if step.ID == module.config.dependencyStepID() && len(module.config.ComposerPackages) > 0 {
		args := append([]string{"composer", "require"}, module.config.ComposerPackages...)
		return run("ddev", args, plan.ProjectPath)
	}
	if step.ID == module.config.modulesStepID() && len(module.config.EnabledModules) > 0 {
		args := append([]string{"drush", "en", "-y"}, module.config.EnabledModules...)
		return run("ddev", args, plan.ProjectPath)
	}
	switch step.ID {
	case "runtime.install":
		formula := "docker"
		if plan.Request.DockerProvider == colima {
			formula = "colima"
		}
		return run("brew", []string{"install", formula}, "")
	case "runtime.start":
		return run("colima", []string{"start"}, "")
	case "host.composer":
		return run("brew", []string{"install", "composer"}, "")
	case "host.ddev":
		return run("brew", []string{"install", "ddev/ddev/ddev"}, "")
	case "project.create":
		return run("composer", []string{"create-project", "drupal/recommended-project:" + drupalProjectConstraint(plan.Request.DrupalVersion), plan.ProjectPath}, plan.Request.ParentDirectory)
	case "ddev.configure":
		return run("ddev", []string{"config", fmt.Sprintf("--project-type=drupal%d", plan.Request.DrupalVersion), "--docroot=web", "--create-docroot"}, plan.ProjectPath)
	case "ddev.start":
		return run("ddev", []string{"start"}, plan.ProjectPath)
	case "drupal.dependencies":
		contribCommand := append([]string{"composer", "require"}, drupalContribPackages(plan.Request.DrupalVersion)...)
		commands := [][]string{
			{"composer", "install"},
			{"composer", "require", "drupal/core-dev:" + drupalProjectConstraint(plan.Request.DrupalVersion), "--dev", "-W"},
			contribCommand,
		}
		for _, args := range commands {
			result, err := run("ddev", args, plan.ProjectPath)
			if err != nil {
				return result, err
			}
		}
		return CommandResult{}, nil
	case "drupal.settings":
		return CommandResult{}, module.writeDrupalSettings(plan.ProjectPath)
	case "drupal.site":
		password := "admin"
		if plan.Request.AdminPasswordEnv != "" {
			password = os.Getenv(plan.Request.AdminPasswordEnv)
			if password == "" {
				return CommandResult{}, fmt.Errorf("environment variable %s is empty", plan.Request.AdminPasswordEnv)
			}
		}
		args := []string{"drush", "site:install", "standard", "--yes", "--account-name=" + plan.Request.AdminUsername, "--account-pass=" + password, "--site-name=Super Awesome Site"}
		return run("ddev", args, plan.ProjectPath)
	case "drupal.modules":
		modules := drupalEnabledModules(plan.Request.DrupalVersion)
		args := append([]string{"drush", "en", "-y"}, modules...)
		return run("ddev", args, plan.ProjectPath)
	case "drupal.config":
		return run("ddev", []string{"drush", "config:import", "--partial", "--yes"}, plan.ProjectPath)
	case "drupal.sample_content":
		result, err := run("ddev", []string{"drush", "genu", "10", "--kill", "--roles=content_editor"}, plan.ProjectPath)
		if err != nil {
			return result, err
		}
		return run("ddev", []string{"drush", "genc", "25", "-y", "--kill", "--roles=content_editor", "--skip-fields=field_tags"}, plan.ProjectPath)
	default:
		return CommandResult{}, fmt.Errorf("unknown installation step %s", step.ID)
	}
}

func (module *installationModule) writeDrupalSettings(projectPath string) error {
	configSyncPath := filepath.Join(projectPath, "config", "sync")
	if err := module.files.MkdirAll(configSyncPath, 0755); err != nil {
		return err
	}
	settingsPath := filepath.Join(projectPath, "web", "sites", "default", "settings.ddev.php")
	content, err := module.files.ReadFile(settingsPath)
	if err != nil {
		return err
	}
	updated := strings.ReplaceAll(string(content), "sites/default/files/sync", "../config/sync")
	if err := module.files.WriteFile(settingsPath, []byte(updated), 0644); err != nil {
		return err
	}
	if err := module.files.WriteFile(filepath.Join(configSyncPath, "environment_indicator.indicator.yml"), []byte(configIndicatorYML), 0644); err != nil {
		return err
	}
	return module.files.WriteFile(filepath.Join(configSyncPath, "environment_indicator.settings.yml"), []byte(configSettingsYML), 0644)
}

func (module *installationModule) Verify(ctx context.Context, plan InstallationPlan, sink EventSink) (InstallationResult, error) {
	result := InstallationResult{SchemaVersion: installationSchemaVersion, PlanID: plan.PlanID, Status: "failed", ProjectPath: plan.ProjectPath}
	if failure := module.validatePlan(plan); failure != nil {
		result.Failure = failure
		return result, *failure
	}
	checks := []struct {
		id   string
		path string
	}{
		{id: "project.composer", path: filepath.Join(plan.ProjectPath, "composer.json")},
		{id: "project.ddev", path: filepath.Join(plan.ProjectPath, ".ddev")},
		{id: "drupal.settings", path: filepath.Join(plan.ProjectPath, "web", "sites", "default", "settings.ddev.php")},
	}
	passed := true
	for _, check := range checks {
		_, err := module.files.Stat(check.path)
		checkPassed := err == nil
		if !checkPassed {
			passed = false
		}
		message := "present"
		if !checkPassed {
			message = "missing"
		}
		result.Verification = append(result.Verification, VerificationCheck{ID: check.id, Passed: checkPassed, Message: message})
	}
	describe := module.runner.Run(ctx, CommandRequest{Name: "ddev", Args: []string{"describe", "--json-output"}, Dir: plan.ProjectPath})
	ddevPassed := describe.Err == nil
	if !ddevPassed {
		passed = false
	}
	result.Verification = append(result.Verification, VerificationCheck{ID: "ddev.describe", Passed: ddevPassed, Message: strings.TrimSpace(describe.Output)})
	if ddevPassed {
		result.SiteURL = siteURLFromDescribe(describe.Output)
	}
	for _, packageName := range module.config.ComposerPackages {
		packageResult := module.runner.Run(ctx, CommandRequest{Name: "ddev", Args: []string{"composer", "show", packageNameWithoutConstraint(packageName)}, Dir: plan.ProjectPath})
		packagePassed := packageResult.Err == nil
		if !packagePassed {
			passed = false
		}
		message := "installed"
		if !packagePassed {
			message = strings.TrimSpace(packageResult.Output)
			if message == "" {
				message = "missing"
			}
		}
		result.Verification = append(result.Verification, VerificationCheck{ID: string(module.config.Type) + ".package", Passed: packagePassed, Message: message})
	}
	if len(module.config.EnabledModules) > 0 {
		modules := module.runner.Run(ctx, CommandRequest{Name: "ddev", Args: []string{"drush", "pm:list", "--type=module", "--status=enabled", "--field=name"}, Dir: plan.ProjectPath})
		enabled := map[string]bool{}
		for _, name := range strings.Fields(modules.Output) {
			enabled[name] = true
		}
		missing := []string{}
		for _, name := range module.config.EnabledModules {
			if !enabled[name] {
				missing = append(missing, name)
			}
		}
		modulesPassed := modules.Err == nil && len(missing) == 0
		if !modulesPassed {
			passed = false
		}
		message := "enabled"
		if modules.Err != nil {
			message = strings.TrimSpace(modules.Output)
			if message == "" {
				message = "could not inspect enabled modules"
			}
		} else if len(missing) > 0 {
			message = "missing enabled modules: " + strings.Join(missing, ", ")
		}
		result.Verification = append(result.Verification, VerificationCheck{ID: string(module.config.Type) + ".modules", Passed: modulesPassed, Message: message})
	}
	if !passed {
		failure := installationFailure("verification_failed", "", "installation verification failed", true, "inspect failed checks and create a new plan")
		result.Failure = &failure
		return result, failure
	}
	result.Status = "succeeded"
	return result, nil
}

func (module *installationModule) validatePlan(plan InstallationPlan) *InstallationFailure {
	if plan.SchemaVersion != installationSchemaVersion {
		failure := installationFailure("invalid_plan", "", "unsupported installation plan schema", false, "create a new plan with this Dropkit version")
		return &failure
	}
	if plan.Request.SchemaVersion != installationSchemaVersion || plan.Request.DrupalVersion < minimumDrupalVersion || plan.Request.DrupalVersion > maximumDrupalVersion {
		failure := installationFailure("invalid_plan", "", "installation plan contains an unsupported Drupal version", false, "create a new plan with a Drupal version from 8 through 12")
		return &failure
	}
	if plan.Request.InstallationType != module.config.Type {
		failure := installationFailure("invalid_plan", "", "installation plan contains the wrong installation type", false, "create a new plan with the command that will apply it")
		return &failure
	}
	if plan.Request.DrupalVersion < module.config.MinimumDrupalVersion || plan.Request.DrupalVersion > module.config.MaximumDrupalVersion {
		failure := installationFailure("invalid_plan", "", fmt.Sprintf("%s plan requires Drupal %s", module.config.ProductName, versionRange(module.config.MinimumDrupalVersion, module.config.MaximumDrupalVersion)), false, "create a new plan with a supported Drupal version")
		return &failure
	}
	digest, err := planDigest(plan)
	if err != nil || digest != plan.Digest || plan.PlanID != plan.Digest[:12] {
		failure := installationFailure("invalid_plan", "", "installation plan digest is invalid", false, "create a new plan")
		return &failure
	}
	known := map[string]bool{"host.platform": true, "host.homebrew": true, "runtime.install": true, "runtime.start": true, "host.composer": true, "host.ddev": true, "project.create": true, "ddev.configure": true, "ddev.start": true, "drupal.dependencies": true, "drupal.settings": true, "drupal.site": true, "drupal.modules": true, "drupal.config": true, "drupal.sample_content": true}
	if len(module.config.ComposerPackages) > 0 {
		known[module.config.dependencyStepID()] = true
	}
	if len(module.config.EnabledModules) > 0 {
		known[module.config.modulesStepID()] = true
	}
	for _, step := range plan.Steps {
		if !known[step.ID] {
			failure := installationFailure("invalid_plan", step.ID, "installation plan contains an unknown step", false, "create a new plan")
			return &failure
		}
	}
	return nil
}

func drupalProjectConstraint(version int) string {
	if version == maximumDrupalVersion {
		return "main-dev@dev"
	}
	return fmt.Sprintf("^%d", version)
}

func drupalContribPackages(version int) []string {
	if version == maximumDrupalVersion {
		return []string{"drush/drush", "drupal/token", "drupal/devel", "drupal/environment_indicator"}
	}
	return []string{"drush/drush", "drupal/admin_toolbar", "drupal/token", "drupal/pathauto", "drupal/config_ignore", "drupal/config_split", "drupal/devel", "drupal/environment_indicator", "drupal/better_exposed_filters", "drupal/key", "drupal/webprofiler", "drupal/diff:^2.0@beta", "drupal/ultimate_cron:^2.0@beta"}
}

func drupalEnabledModules(version int) []string {
	if version == maximumDrupalVersion {
		return []string{"devel", "devel_generate", "environment_indicator", "environment_indicator_ui", "environment_indicator_toolbar", "token"}
	}
	return []string{"admin_toolbar", "admin_toolbar_tools", "config_split", "devel", "environment_indicator", "environment_indicator_ui", "environment_indicator_toolbar", "token", "pathauto", "config_ignore", "better_exposed_filters", "key", "webprofiler", "diff", "ultimate_cron", "devel_generate"}
}

func versionChoices(minimum, maximum int) string {
	values := make([]string, 0, maximum-minimum+1)
	for version := minimum; version <= maximum; version++ {
		values = append(values, fmt.Sprintf("%d", version))
	}
	return strings.Join(values, " or ")
}

func versionRange(minimum, maximum int) string {
	if minimum == maximum {
		return fmt.Sprintf("%d", minimum)
	}
	if maximum == minimum+1 {
		return fmt.Sprintf("%d or %d", minimum, maximum)
	}
	return fmt.Sprintf("%d through %d", minimum, maximum)
}

func packageNameWithoutConstraint(packageName string) string {
	if index := strings.Index(packageName, ":"); index >= 0 {
		return packageName[:index]
	}
	return packageName
}

func planDigest(plan InstallationPlan) (string, error) {
	plan.Digest = ""
	plan.PlanID = ""
	encoded, err := json.Marshal(plan)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func inspectionFingerprint(inspection HostInspection) (string, error) {
	inspection.Fingerprint = ""
	encoded, err := json.Marshal(inspection)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func installationFailure(code, stepID, message string, retryable bool, recovery string) InstallationFailure {
	return InstallationFailure{Code: code, StepID: stepID, Message: message, Retryable: retryable, Recovery: recovery}
}

func failureFromError(err error) InstallationFailure {
	var failure InstallationFailure
	if errors.As(err, &failure) {
		return failure
	}
	return installationFailure("internal_error", "", err.Error(), false, "")
}

func redactOutput(output string, request InstallationRequest) string {
	if request.AdminPasswordEnv == "" {
		return strings.ReplaceAll(output, "admin", "[redacted]")
	}
	secret := os.Getenv(request.AdminPasswordEnv)
	if secret == "" {
		return output
	}
	return strings.ReplaceAll(output, secret, "[redacted]")
}

func siteURLFromDescribe(output string) string {
	var result struct {
		Raw []struct {
			HTTPSURL string `json:"https_url"`
		} `json:"raw"`
	}
	if json.Unmarshal([]byte(output), &result) != nil || len(result.Raw) == 0 {
		return ""
	}
	return result.Raw[0].HTTPSURL
}
