package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

type humanEventSink struct {
	writer io.Writer
}

func (sink humanEventSink) Emit(event Event) {
	if event.Type == "command_output" {
		fmt.Fprintf(sink.writer, "[%s] %s\n", event.StepID, strings.TrimSpace(event.Message))
		return
	}
	fmt.Fprintf(sink.writer, "[%s] %s: %s\n", strings.ToUpper(event.Level), event.StepID, event.Message)
}

type jsonEventSink struct {
	writer io.Writer
}

func (sink jsonEventSink) Emit(event Event) {
	_ = json.NewEncoder(sink.writer).Encode(event)
}

func runInstallCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return runInstallationCommand(drupalInstallation, args, stdin, stdout, stderr)
}

func runCommerceCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return runInstallationCommand(commerceInstallation, args, stdin, stdout, stderr)
}

func runInstallationCommand(installationType InstallationType, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	usage := printInstallUsage
	command := "install"
	if installationType == commerceInstallation {
		usage = printCommerceUsage
		command = "commerce"
	}
	if len(args) == 0 {
		if !readerIsTerminal(stdin) {
			failure := installationFailure("invalid_request", "", "interactive "+command+" requires a terminal", false, "use dropkit "+command+" plan with explicit arguments")
			fmt.Fprintln(stderr, failure.Message)
			return exitCodeForFailure(failure)
		}
		return runInstallWizardForType(context.Background(), newProductionInstallationModule(), installationType, stdin, stdout, stderr)
	}
	switch args[0] {
	case "plan":
		return runPlanCommandForType(context.Background(), newProductionInstallationModule(), installationType, args[1:], stdout, stderr)
	case "apply":
		return runApplyCommandForType(context.Background(), newProductionInstallationModule(), installationType, args[1:], stdout, stderr)
	case "verify":
		return runVerifyCommandForType(context.Background(), newProductionInstallationModule(), installationType, args[1:], stdout, stderr)
	case "-h", "--help", "help":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown %s command %q\n\n", command, args[0])
		usage(stderr)
		return 2
	}
}

func runPlanCommand(ctx context.Context, module InstallationModule, args []string, stdout, stderr io.Writer) int {
	return runPlanCommandForType(ctx, module, drupalInstallation, args, stdout, stderr)
}

func runPlanCommandForType(ctx context.Context, module InstallationModule, installationType InstallationType, args []string, stdout, stderr io.Writer) int {
	if helpRequested(args) {
		printPlanUsageForType(stdout, installationType)
		return 0
	}
	flags := flag.NewFlagSet("dropkit install plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	name := flags.String("name", "", "Drupal project name")
	parent := flags.String("parent", "", "parent directory")
	provider := flags.String("provider", "", "docker or colima")
	drupalVersion := flags.Int("drupal-version", 0, "Drupal major version from 8 through 12")
	generateContent := flags.Bool("generate-content", false, "generate sample content")
	adminUsername := flags.String("admin-user", "admin", "Drupal administrator username")
	adminPasswordEnv := flags.String("admin-password-env", "", "environment variable containing the Drupal administrator password")
	output := flags.String("output", "human", "human or json")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return writeCommandFailure(stdout, stderr, *output, installationFailure("invalid_request", "", "invalid plan arguments", false, "run dropkit help install"))
	}
	if !validOutputFormat(*output) {
		return writeCommandFailure(stdout, stderr, "human", installationFailure("invalid_request", "", "output must be human or json", false, "use --output human or --output json"))
	}
	plan, err := module.Plan(ctx, InstallationRequest{
		InstallationType: installationType,
		ProjectName:      *name,
		ParentDirectory:  *parent,
		DockerProvider:   DockerProvider(*provider),
		DrupalVersion:    *drupalVersion,
		GenerateContent:  *generateContent,
		AdminUsername:    *adminUsername,
		AdminPasswordEnv: *adminPasswordEnv,
	})
	if err != nil {
		return writeCommandFailure(stdout, stderr, *output, failureFromError(err))
	}
	if err := writePlan(stdout, *output, plan); err != nil {
		return writeCommandFailure(stdout, stderr, *output, installationFailure("output_failed", "", err.Error(), false, "check the output destination"))
	}
	if plan.Blocked {
		return 3
	}
	return 0
}

func runApplyCommand(ctx context.Context, module InstallationModule, args []string, stdout, stderr io.Writer) int {
	return runApplyCommandForType(ctx, module, drupalInstallation, args, stdout, stderr)
}

func runApplyCommandForType(ctx context.Context, module InstallationModule, installationType InstallationType, args []string, stdout, stderr io.Writer) int {
	if helpRequested(args) {
		printApplyUsageForType(stdout, installationType)
		return 0
	}
	flags := flag.NewFlagSet("dropkit install apply", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	planPath := flags.String("plan", "", "path to an installation plan")
	allowNetwork := flags.Bool("allow-network", false, "allow network access")
	allowHostChanges := flags.Bool("allow-host-changes", false, "allow host package installation and runtime changes")
	allowDestructive := flags.Bool("allow-destructive", false, "allow destructive project operations")
	output := flags.String("output", "human", "human or json")
	events := flags.String("events", "human", "human, jsonl, or none")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *planPath == "" {
		return writeCommandFailure(stdout, stderr, *output, installationFailure("invalid_request", "", "--plan is required and arguments must be valid", false, "run dropkit help install"))
	}
	if !validOutputFormat(*output) {
		return writeCommandFailure(stdout, stderr, "human", installationFailure("invalid_request", "", "output must be human or json", false, "use --output human or --output json"))
	}
	plan, err := readPlan(*planPath)
	if err != nil {
		return writeCommandFailure(stdout, stderr, *output, installationFailure("invalid_plan", "", err.Error(), false, "create a new plan"))
	}
	if plan.Request.InstallationType != "" && plan.Request.InstallationType != installationType {
		return writeCommandFailure(stdout, stderr, *output, installationFailure("invalid_plan", "", fmt.Sprintf("plan is for %s, not %s", plan.Request.InstallationType, installationType), false, "use the command that created this plan"))
	}
	allowed := map[Effect]bool{}
	if *allowNetwork {
		allowed[effectNetwork] = true
	}
	if *allowHostChanges {
		allowed[effectHostChange] = true
	}
	if *allowDestructive {
		allowed[effectDestructive] = true
	}
	sink, sinkErr := selectEventSink(*events, stderr)
	if sinkErr != nil {
		return writeCommandFailure(stdout, stderr, *output, installationFailure("invalid_request", "", sinkErr.Error(), false, "use --events human, jsonl, or none"))
	}
	result, applyErr := module.Apply(ctx, plan, Approval{PlanDigest: plan.Digest, AllowedEffects: allowed}, sink)
	if err := writeResult(stdout, *output, result); err != nil {
		return writeCommandFailure(stdout, stderr, *output, installationFailure("output_failed", "", err.Error(), false, "check the output destination"))
	}
	if applyErr != nil {
		return exitCodeForFailure(failureFromError(applyErr))
	}
	return 0
}

func runVerifyCommand(ctx context.Context, module InstallationModule, args []string, stdout, stderr io.Writer) int {
	return runVerifyCommandForType(ctx, module, drupalInstallation, args, stdout, stderr)
}

func runVerifyCommandForType(ctx context.Context, module InstallationModule, installationType InstallationType, args []string, stdout, stderr io.Writer) int {
	if helpRequested(args) {
		printVerifyUsageForType(stdout, installationType)
		return 0
	}
	flags := flag.NewFlagSet("dropkit install verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	planPath := flags.String("plan", "", "path to an installation plan")
	output := flags.String("output", "human", "human or json")
	events := flags.String("events", "none", "human, jsonl, or none")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *planPath == "" {
		return writeCommandFailure(stdout, stderr, *output, installationFailure("invalid_request", "", "--plan is required and arguments must be valid", false, "run dropkit help install"))
	}
	if !validOutputFormat(*output) {
		return writeCommandFailure(stdout, stderr, "human", installationFailure("invalid_request", "", "output must be human or json", false, "use --output human or --output json"))
	}
	plan, err := readPlan(*planPath)
	if err != nil {
		return writeCommandFailure(stdout, stderr, *output, installationFailure("invalid_plan", "", err.Error(), false, "create a new plan"))
	}
	if plan.Request.InstallationType != "" && plan.Request.InstallationType != installationType {
		return writeCommandFailure(stdout, stderr, *output, installationFailure("invalid_plan", "", fmt.Sprintf("plan is for %s, not %s", plan.Request.InstallationType, installationType), false, "use the command that created this plan"))
	}
	sink, sinkErr := selectEventSink(*events, stderr)
	if sinkErr != nil {
		return writeCommandFailure(stdout, stderr, *output, installationFailure("invalid_request", "", sinkErr.Error(), false, "use --events human, jsonl, or none"))
	}
	result, verifyErr := module.Verify(ctx, plan, sink)
	if err := writeResult(stdout, *output, result); err != nil {
		return writeCommandFailure(stdout, stderr, *output, installationFailure("output_failed", "", err.Error(), false, "check the output destination"))
	}
	if verifyErr != nil {
		return exitCodeForFailure(failureFromError(verifyErr))
	}
	return 0
}

func readerIsTerminal(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func selectEventSink(format string, writer io.Writer) (EventSink, error) {
	switch format {
	case "human":
		return humanEventSink{writer: writer}, nil
	case "jsonl":
		return jsonEventSink{writer: writer}, nil
	case "none":
		return discardEventSink{}, nil
	default:
		return nil, fmt.Errorf("unknown event format %q", format)
	}
}

func readPlan(path string) (InstallationPlan, error) {
	file, err := os.Open(path)
	if err != nil {
		return InstallationPlan{}, err
	}
	defer file.Close()
	var plan InstallationPlan
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return InstallationPlan{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return InstallationPlan{}, fmt.Errorf("plan file contains more than one JSON document")
		}
		return InstallationPlan{}, err
	}
	return plan, nil
}

func writePlan(writer io.Writer, format string, plan InstallationPlan) error {
	if format == "json" {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
	}
	if format != "human" {
		return fmt.Errorf("unknown output format %q", format)
	}
	fmt.Fprintf(writer, "Plan %s for %s\n", plan.PlanID, plan.ProjectPath)
	for _, step := range plan.Steps {
		fmt.Fprintf(writer, "  %-22s %-8s %s", step.ID, step.Disposition, step.Summary)
		if len(step.Effects) > 0 {
			fmt.Fprintf(writer, " [%s]", joinEffects(step.Effects))
		}
		if step.Reason != "" {
			fmt.Fprintf(writer, ": %s", step.Reason)
		}
		fmt.Fprintln(writer)
	}
	if len(plan.RequiredApprovals) > 0 {
		fmt.Fprintf(writer, "Required approvals: %s\n", joinEffects(plan.RequiredApprovals))
	}
	return nil
}

func writeResult(writer io.Writer, format string, result InstallationResult) error {
	if format == "json" {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	if format != "human" {
		return fmt.Errorf("unknown output format %q", format)
	}
	fmt.Fprintf(writer, "Installation %s\n", result.Status)
	if result.ProjectPath != "" {
		fmt.Fprintf(writer, "Project: %s\n", result.ProjectPath)
	}
	if result.SiteURL != "" {
		fmt.Fprintf(writer, "Site: %s\n", result.SiteURL)
	}
	if result.Failure != nil {
		fmt.Fprintf(writer, "Failure: %s (%s)\n", result.Failure.Message, result.Failure.Code)
		if result.Failure.Recovery != "" {
			fmt.Fprintf(writer, "Recovery: %s\n", result.Failure.Recovery)
		}
	}
	return nil
}

func writeCommandFailure(stdout, stderr io.Writer, format string, failure InstallationFailure) int {
	if format == "json" {
		_ = writeResult(stdout, "json", InstallationResult{SchemaVersion: installationSchemaVersion, Status: "failed", Failure: &failure})
	} else {
		fmt.Fprintf(stderr, "%s: %s\n", failure.Code, failure.Message)
		if failure.Recovery != "" {
			fmt.Fprintf(stderr, "Recovery: %s\n", failure.Recovery)
		}
	}
	return exitCodeForFailure(failure)
}

func exitCodeForFailure(failure InstallationFailure) int {
	switch failure.Code {
	case "invalid_request", "invalid_plan":
		return 2
	case "plan_blocked", "approval_required":
		return 3
	case "inspection_failed", "plan_stale":
		return 4
	case "step_failed":
		return 5
	case "verification_failed":
		return 6
	default:
		return 1
	}
}

func joinEffects(effects []Effect) string {
	values := make([]string, 0, len(effects))
	for _, effect := range effects {
		values = append(values, string(effect))
	}
	return strings.Join(values, ", ")
}

func helpRequested(args []string) bool {
	return len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help")
}

func validOutputFormat(format string) bool {
	return format == "human" || format == "json"
}

func printPlanUsage(writer io.Writer) {
	printPlanUsageForType(writer, drupalInstallation)
}

func printPlanUsageForType(writer io.Writer, installationType InstallationType) {
	command := "install"
	versions := "8|9|10|11|12"
	versionDescription := "Drupal major version from 8 through 12"
	if installationType == commerceInstallation {
		command = "commerce"
		versions = "10|11"
		versionDescription = "Drupal major version 10 or 11"
	}
	fmt.Fprintf(writer, "Usage: dropkit %s plan --name NAME --parent DIR --provider docker|colima --drupal-version %s [options]\n", command, versions)
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Required inputs:")
	fmt.Fprintf(writer, "  --drupal-version VERSION   %s\n", versionDescription)
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Read-only options:")
	fmt.Fprintln(writer, "  --generate-content          Include destructive sample-content generation")
	fmt.Fprintln(writer, "  --admin-user NAME           Drupal administrator username (default: admin)")
	fmt.Fprintln(writer, "  --admin-password-env NAME   Environment variable containing the administrator password")
	fmt.Fprintln(writer, "  --output human|json         Output format (default: human)")
}

func printApplyUsage(writer io.Writer) {
	printApplyUsageForType(writer, drupalInstallation)
}

func printApplyUsageForType(writer io.Writer, installationType InstallationType) {
	fmt.Fprintf(writer, "Usage: dropkit %s apply --plan FILE [options]\n", installationType)
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Approvals:")
	fmt.Fprintln(writer, "  --allow-network")
	fmt.Fprintln(writer, "  --allow-host-changes")
	fmt.Fprintln(writer, "  --allow-destructive")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Output:")
	fmt.Fprintln(writer, "  --output human|json")
	fmt.Fprintln(writer, "  --events human|jsonl|none")
}

func printVerifyUsage(writer io.Writer) {
	printVerifyUsageForType(writer, drupalInstallation)
}

func printVerifyUsageForType(writer io.Writer, installationType InstallationType) {
	fmt.Fprintf(writer, "Usage: dropkit %s verify --plan FILE [options]\n", installationType)
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Verify is read-only.")
	fmt.Fprintln(writer, "  --output human|json")
	fmt.Fprintln(writer, "  --events human|jsonl|none")
}
