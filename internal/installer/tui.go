package installer

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

const (
	tuiReset    = "\x1b[0m"
	tuiBold     = "\x1b[1m"
	tuiDim      = "\x1b[2m"
	tuiPink     = "\x1b[38;5;212m"
	tuiCyan     = "\x1b[38;5;81m"
	tuiGreen    = "\x1b[38;5;84m"
	tuiYellow   = "\x1b[38;5;221m"
	tuiRed      = "\x1b[38;5;203m"
	tuiMuted    = "\x1b[38;5;245m"
	tuiPanel    = "\x1b[48;5;236m"
	tuiSelected = "\x1b[1;38;5;232;48;5;212m"
)

type installTUIStage int

const (
	installTUIProvider installTUIStage = iota
	installTUIVersion
	installTUIProject
	installTUIContent
	installTUIPlanning
	installTUIReview
	installTUIApplying
	installTUIFinished
)

type installPlanMsg struct {
	plan InstallationPlan
	err  error
}

type installApplyMsg struct {
	result InstallationResult
	err    error
}

type installEventMsg Event

type installTickMsg time.Time

type installTUIEventSink struct {
	send func(tea.Msg)
}

func (sink installTUIEventSink) Emit(event Event) {
	if sink.send != nil {
		sink.send(installEventMsg(event))
	}
}

type installTUIModel struct {
	ctx             context.Context
	cancel          context.CancelFunc
	module          InstallationModule
	config          InstallationConfig
	parent          string
	stage           installTUIStage
	provider        DockerProvider
	drupalVersion   int
	projectName     string
	generateContent bool
	plan            InstallationPlan
	result          InstallationResult
	err             error
	status          string
	events          []Event
	spinner         int
	width           int
	height          int
	cancelled       bool
	send            func(tea.Msg)
}

func newInstallTUIModel(ctx context.Context, cancel context.CancelFunc, module InstallationModule, config InstallationConfig, parent string) *installTUIModel {
	drupalVersion := min(defaultDrupalVersion, config.MaximumDrupalVersion)
	if config.FixedDrupalVersion != 0 {
		drupalVersion = config.FixedDrupalVersion
	}
	return &installTUIModel{
		ctx:           ctx,
		cancel:        cancel,
		module:        module,
		config:        config,
		parent:        parent,
		provider:      colima,
		drupalVersion: drupalVersion,
		width:         80,
		height:        24,
	}
}

func (model *installTUIModel) Init() tea.Cmd {
	return nil
}

func (model *installTUIModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		model.width = message.Width
		model.height = message.Height
		return model, nil
	case installPlanMsg:
		model.plan = message.plan
		model.err = message.err
		model.stage = installTUIReview
		return model, nil
	case installApplyMsg:
		model.result = message.result
		model.err = message.err
		model.stage = installTUIFinished
		return model, nil
	case installEventMsg:
		event := Event(message)
		model.events = append(model.events, event)
		if len(model.events) > 8 {
			model.events = model.events[len(model.events)-8:]
		}
		model.status = event.Message
		return model, nil
	case installTickMsg:
		if model.stage == installTUIPlanning || model.stage == installTUIApplying {
			model.spinner = (model.spinner + 1) % 10
			return model, installTUITick()
		}
		return model, nil
	case tea.PasteMsg:
		if model.stage == installTUIProject {
			model.projectName += strings.ReplaceAll(message.Content, "\n", "")
		}
		return model, nil
	case tea.KeyPressMsg:
		return model.handleKey(message)
	}
	return model, nil
}

func (model *installTUIModel) handleKey(message tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := message.Key()
	if message.String() == "ctrl+c" {
		model.cancelled = true
		model.cancel()
		return model, tea.Quit
	}
	if model.stage == installTUIApplying || model.stage == installTUIPlanning {
		return model, nil
	}
	if model.stage == installTUIFinished {
		if message.String() == "enter" || message.String() == "q" || message.String() == "esc" {
			return model, tea.Quit
		}
		return model, nil
	}
	if message.String() == "esc" || message.String() == "q" && model.stage != installTUIProject {
		model.cancelled = true
		return model, tea.Quit
	}
	switch model.stage {
	case installTUIProvider:
		switch message.String() {
		case "up", "down", "left", "right", "tab", "shift+tab", "space":
			if model.provider == colima {
				model.provider = dockerDesktop
			} else {
				model.provider = colima
			}
		case "enter":
			if model.config.FixedDrupalVersion != 0 {
				model.stage = installTUIProject
			} else {
				model.stage = installTUIVersion
			}
		}
	case installTUIVersion:
		minimumVersion, maximumVersion := model.config.MinimumDrupalVersion, model.config.MaximumDrupalVersion
		switch message.String() {
		case "up", "left", "shift+tab":
			model.drupalVersion--
			if model.drupalVersion < minimumVersion {
				model.drupalVersion = maximumVersion
			}
		case "down", "right", "tab", "space":
			model.drupalVersion++
			if model.drupalVersion > maximumVersion {
				model.drupalVersion = minimumVersion
			}
		case "enter":
			model.stage = installTUIProject
		}
	case installTUIProject:
		switch message.String() {
		case "backspace":
			if len(model.projectName) > 0 {
				_, size := utf8.DecodeLastRuneInString(model.projectName)
				model.projectName = model.projectName[:len(model.projectName)-size]
			}
		case "enter":
			if strings.TrimSpace(model.projectName) != "" {
				model.status = ""
				if model.config.BrowserInstaller {
					model.stage = installTUIPlanning
					return model, tea.Batch(model.planCommand(), installTUITick())
				}
				model.stage = installTUIContent
			} else {
				model.status = "Project name is required"
			}
		default:
			if key.Text != "" {
				model.projectName += key.Text
			}
		}
	case installTUIContent:
		switch message.String() {
		case "up", "down", "left", "right", "tab", "shift+tab", "space", "y", "n":
			if message.String() == "y" {
				model.generateContent = true
			} else if message.String() == "n" {
				model.generateContent = false
			} else {
				model.generateContent = !model.generateContent
			}
		case "enter":
			model.stage = installTUIPlanning
			return model, tea.Batch(model.planCommand(), installTUITick())
		}
	case installTUIReview:
		if model.err != nil || model.plan.Blocked {
			return model, nil
		}
		if message.String() == "enter" || message.String() == "a" {
			model.stage = installTUIApplying
			model.status = "Preparing installation"
			return model, tea.Batch(model.applyCommand(), installTUITick())
		}
	}
	return model, nil
}

func (model *installTUIModel) planCommand() tea.Cmd {
	adminUsername := "admin"
	if model.config.BrowserInstaller {
		adminUsername = ""
	}
	request := InstallationRequest{
		InstallationType: model.config.Type,
		ProjectName:      model.projectName,
		ParentDirectory:  model.parent,
		DockerProvider:   model.provider,
		DrupalVersion:    model.drupalVersion,
		GenerateContent:  model.generateContent,
		AdminUsername:    adminUsername,
	}
	return func() tea.Msg {
		plan, err := model.module.Plan(model.ctx, request)
		return installPlanMsg{plan: plan, err: err}
	}
}

func (model *installTUIModel) applyCommand() tea.Cmd {
	allowed := map[Effect]bool{}
	for _, effect := range model.plan.RequiredApprovals {
		allowed[effect] = true
	}
	approval := Approval{PlanDigest: model.plan.Digest, AllowedEffects: allowed}
	sink := installTUIEventSink{send: model.send}
	return func() tea.Msg {
		result, err := model.module.Apply(model.ctx, model.plan, approval, sink)
		return installApplyMsg{result: result, err: err}
	}
}

func installTUITick() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(now time.Time) tea.Msg {
		return installTickMsg(now)
	})
}

func (model *installTUIModel) View() tea.View {
	var body strings.Builder
	body.WriteString(tuiBold + tuiPink + "  DROP KIT" + tuiReset + tuiMuted + "  Drupal environment builder" + tuiReset + "\n")
	body.WriteString(tuiMuted + "  " + strings.Repeat("─", model.contentWidth()) + tuiReset + "\n\n")
	switch model.stage {
	case installTUIProvider:
		model.renderProvider(&body)
	case installTUIVersion:
		model.renderVersion(&body)
	case installTUIProject:
		model.renderProject(&body)
	case installTUIContent:
		model.renderContent(&body)
	case installTUIPlanning:
		model.renderBusy(&body, "Inspecting your development environment")
	case installTUIReview:
		model.renderReview(&body)
	case installTUIApplying:
		model.renderApply(&body)
	case installTUIFinished:
		model.renderFinished(&body)
	}
	view := tea.NewView(body.String())
	view.AltScreen = true
	view.WindowTitle = "Dropkit"
	return view
}

func (model *installTUIModel) renderVersion(body *strings.Builder) {
	minimumVersion, maximumVersion := model.config.MinimumDrupalVersion, model.config.MaximumDrupalVersion
	body.WriteString(tuiBold + "  Choose a Drupal version" + tuiReset + "\n")
	versionDescription := model.config.ProductName + " supports Drupal " + versionRange(minimumVersion, maximumVersion) + "."
	body.WriteString(tuiMuted + "  " + versionDescription + tuiReset + "\n\n")
	for version := minimumVersion; version <= maximumVersion; version++ {
		detail := ""
		if version == defaultDrupalVersion {
			detail = "Default"
		} else if version == maximumDrupalVersion {
			detail = "Development"
		}
		model.renderChoice(body, model.drupalVersion == version, fmt.Sprintf("Drupal %d", version), detail)
	}
	model.renderFooter(body, "↑/↓ choose", "enter continue", "q quit")
}

func (model *installTUIModel) renderProvider(body *strings.Builder) {
	body.WriteString(tuiBold + "  Choose a container runtime" + tuiReset + "\n")
	body.WriteString(tuiMuted + "  Use ↑/↓ to choose, then Enter" + tuiReset + "\n\n")
	model.renderChoice(body, model.provider == colima, "Colima", "Lightweight macOS runtime")
	model.renderChoice(body, model.provider == dockerDesktop, "Docker Desktop", "Docker's desktop application")
	model.renderFooter(body, "enter select", "q quit")
}

func (model *installTUIModel) renderProject(body *strings.Builder) {
	body.WriteString(tuiBold + "  Name your Drupal project" + tuiReset + "\n")
	body.WriteString(tuiMuted + "  It will be created inside " + tuiReset + model.parent + "\n\n")
	body.WriteString("  " + tuiPink + "›" + tuiReset + " " + model.projectName + tuiPink + "█" + tuiReset + "\n")
	if model.status != "" {
		body.WriteString("\n  " + tuiRed + model.status + tuiReset + "\n")
	}
	model.renderFooter(body, "enter continue", "esc cancel")
}

func (model *installTUIModel) renderContent(body *strings.Builder) {
	body.WriteString(tuiBold + "  Generate sample content?" + tuiReset + "\n")
	body.WriteString(tuiMuted + "  This adds test users and content after installation." + tuiReset + "\n\n")
	model.renderChoice(body, !model.generateContent, "No", "Start with a clean Drupal site")
	model.renderChoice(body, model.generateContent, "Yes", "Run destructive content generators")
	model.renderFooter(body, "space toggle", "enter inspect")
}

func (model *installTUIModel) renderBusy(body *strings.Builder, message string) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	body.WriteString("  " + tuiCyan + frames[model.spinner] + tuiReset + " " + tuiBold + message + tuiReset + "\n\n")
	body.WriteString(tuiMuted + "  Planning is read-only. No files or host state are being changed." + tuiReset + "\n")
	model.renderFooter(body, "ctrl+c cancel")
}

func (model *installTUIModel) renderReview(body *strings.Builder) {
	if model.err != nil {
		failure := failureFromError(model.err)
		body.WriteString("  " + tuiRed + "✕  Could not create a plan" + tuiReset + "\n\n")
		body.WriteString("  " + failure.Message + "\n")
		if failure.Recovery != "" {
			body.WriteString("  " + tuiYellow + "Recovery: " + tuiReset + failure.Recovery + "\n")
		}
		model.renderFooter(body, "q close")
		return
	}
	body.WriteString("  " + tuiGreen + "✓" + tuiReset + " " + tuiBold + "Installation plan" + tuiReset + tuiMuted + "  #" + model.plan.PlanID + tuiReset + "\n")
	body.WriteString("  " + tuiMuted + model.plan.ProjectPath + tuiReset + "\n\n")
	product := fmt.Sprintf("%s %d", model.config.ProductName, model.plan.Request.DrupalVersion)
	if model.config.BrowserInstaller {
		product = model.config.ProductName
	}
	body.WriteString(fmt.Sprintf("  %s%s%s\n\n", tuiBold, product, tuiReset))
	for _, step := range model.plan.Steps {
		icon, color := "○", tuiMuted
		switch step.Disposition {
		case dispositionCreate, dispositionModify:
			icon, color = "◆", tuiCyan
		case dispositionNoOp:
			icon, color = "✓", tuiGreen
		case dispositionBlocked:
			icon, color = "!", tuiRed
		}
		body.WriteString(fmt.Sprintf("  %s%s%s  %-22s %s\n", color, icon, tuiReset, step.ID, trimForWidth(step.Summary, model.contentWidth()-30)))
	}
	if model.plan.Blocked {
		body.WriteString("\n  " + tuiRed + tuiBold + "This plan is blocked and cannot be applied." + tuiReset + "\n")
		model.renderFooter(body, "q close")
		return
	}
	if len(model.plan.RequiredApprovals) > 0 {
		body.WriteString("\n  " + tuiYellow + "Authorization required: " + tuiReset + joinEffects(model.plan.RequiredApprovals) + "\n")
	}
	body.WriteString("  " + tuiMuted + "Enter authorizes these effects and starts installation." + tuiReset + "\n")
	model.renderFooter(body, "enter apply", "esc cancel")
}

func (model *installTUIModel) renderApply(body *strings.Builder) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	product := fmt.Sprintf("%s %d", model.config.ProductName, model.plan.Request.DrupalVersion)
	if model.config.BrowserInstaller {
		product = model.config.ProductName
	}
	body.WriteString(fmt.Sprintf("  %s%s%s %sInstalling %s%s\n", tuiCyan, frames[model.spinner], tuiReset, tuiBold, product, tuiReset))
	body.WriteString("  " + tuiMuted + trimForWidth(model.status, model.contentWidth()-2) + tuiReset + "\n\n")
	for _, event := range model.events {
		icon, color := "·", tuiMuted
		if event.Level == "error" {
			icon, color = "✕", tuiRed
		} else if event.Type != "command_output" {
			icon, color = "✓", tuiGreen
		}
		body.WriteString(fmt.Sprintf("  %s%s%s  %-22s %s\n", color, icon, tuiReset, event.StepID, trimForWidth(strings.TrimSpace(event.Message), model.contentWidth()-30)))
	}
	model.renderFooter(body, "ctrl+c stop current command")
}

func (model *installTUIModel) renderFinished(body *strings.Builder) {
	if model.err != nil || model.result.Status != "succeeded" {
		command := model.config.CommandName
		failure := installationFailure("internal_error", "", "installation did not complete", false, "run dropkit "+command+" again")
		if model.err != nil {
			failure = failureFromError(model.err)
		}
		if model.result.Failure != nil {
			failure = *model.result.Failure
		}
		body.WriteString("  " + tuiRed + "✕  Installation failed" + tuiReset + "\n\n")
		body.WriteString("  " + failure.Message + "\n")
		if failure.StepID != "" {
			body.WriteString("  " + tuiMuted + "Step: " + failure.StepID + tuiReset + "\n")
		}
		if failure.Recovery != "" {
			body.WriteString("  " + tuiYellow + "Recovery: " + tuiReset + failure.Recovery + "\n")
		}
	} else {
		if model.config.BrowserInstaller {
			body.WriteString(fmt.Sprintf("  %s%s✓  %s setup assistant launched%s\n\n", tuiGreen, tuiBold, model.config.ProductName, tuiReset))
		} else {
			body.WriteString(fmt.Sprintf("  %s%s✓  %s %d is ready%s\n\n", tuiGreen, tuiBold, model.config.ProductName, model.plan.Request.DrupalVersion, tuiReset))
		}
		body.WriteString("  Project  " + model.result.ProjectPath + "\n")
		if model.result.SiteURL != "" {
			body.WriteString("  Site     " + tuiCyan + model.result.SiteURL + tuiReset + "\n")
		}
	}
	model.renderFooter(body, "enter close")
}

func (model *installTUIModel) renderChoice(body *strings.Builder, selected bool, title, detail string) {
	if selected {
		body.WriteString("  " + tuiSelected + " › " + title + " " + tuiReset + "  " + detail + "\n\n")
		return
	}
	body.WriteString("    " + tuiBold + title + tuiReset + "  " + tuiMuted + detail + tuiReset + "\n\n")
}

func (model *installTUIModel) renderFooter(body *strings.Builder, hints ...string) {
	body.WriteString("\n  ")
	for index, hint := range hints {
		if index > 0 {
			body.WriteString(tuiMuted + "  •  " + tuiReset)
		}
		parts := strings.SplitN(hint, " ", 2)
		body.WriteString(tuiPanel + tuiBold + " " + parts[0] + " " + tuiReset)
		if len(parts) == 2 {
			body.WriteString(tuiMuted + " " + parts[1] + tuiReset)
		}
	}
	body.WriteString("\n")
}

func (model *installTUIModel) contentWidth() int {
	width := model.width - 6
	if width < 42 {
		return 42
	}
	if width > 100 {
		return 100
	}
	return width
}

func trimForWidth(value string, width int) string {
	value = strings.ReplaceAll(value, "\n", " ")
	if width < 2 || utf8.RuneCountInString(value) <= width {
		return value
	}
	runes := []rune(value)
	return string(runes[:width-1]) + "…"
}

func runInstallWizardForConfig(ctx context.Context, module InstallationModule, config InstallationConfig, stdin io.Reader, stdout, stderr io.Writer) int {
	parent, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	wizardContext, cancel := context.WithCancel(ctx)
	defer cancel()
	model := newInstallTUIModel(wizardContext, cancel, module, config, parent)
	program := tea.NewProgram(model, tea.WithContext(wizardContext), tea.WithInput(stdin), tea.WithOutput(stdout))
	model.send = program.Send
	if _, err := program.Run(); err != nil {
		if model.cancelled {
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	if model.err != nil {
		return exitCodeForFailure(failureFromError(model.err))
	}
	if model.plan.Blocked {
		return 3
	}
	if model.result.Failure != nil {
		return exitCodeForFailure(*model.result.Failure)
	}
	if model.cancelled {
		return 0
	}
	return 0
}
