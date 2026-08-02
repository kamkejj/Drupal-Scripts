package cms

import (
	"fmt"
	"io"
	"strings"

	"dropkit/internal/installer"
)

var config = installer.InstallationConfig{
	CommandName:          "cms",
	Type:                 "cms",
	ProductName:          "Drupal CMS",
	MinimumDrupalVersion: 11,
	MaximumDrupalVersion: 11,
	FixedDrupalVersion:   11,
	ProjectTemplate:      "drupal/cms",
	BrowserInstaller:     true,
	EnabledModules: []string{
		"config",
		"inline_form_errors",
		"settings_tray",
		"toolbar",
		"syslog",
		"workspaces",
		"workspaces_ui",
	},
}

var command = installer.NewCommand(config, PrintUsage)

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return command.Run(args, stdin, stdout, stderr)
}

func PrintUsage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage:")
	fmt.Fprintln(writer, "  dropkit cms                                                Interactive TUI (terminal only)")
	fmt.Fprintln(writer, "  dropkit cms plan --name NAME --parent DIR --provider docker|colima [options]")
	fmt.Fprintln(writer, "  dropkit cms apply --plan FILE [approvals] [options]")
	fmt.Fprintln(writer, "  dropkit cms verify --plan FILE [options]")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Commands:")
	fmt.Fprintln(writer, "  plan      Inspect the host and create a read-only Drupal CMS installation plan")
	fmt.Fprintln(writer, "  apply     Create the Drupal CMS project and launch its browser setup assistant")
	fmt.Fprintln(writer, "  verify    Verify the Drupal CMS project without modifying the host")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Project template:")
	fmt.Fprintln(writer, "  drupal/cms (latest stable release)")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Enabled modules:")
	fmt.Fprintln(writer, "  "+strings.Join(config.EnabledModules, ", "))
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Approvals:")
	fmt.Fprintln(writer, "  --allow-network       Allow downloads")
	fmt.Fprintln(writer, "  --allow-host-changes  Allow host package and runtime changes")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Machine output:")
	fmt.Fprintln(writer, "  --output json         Write one JSON document to stdout")
	fmt.Fprintln(writer, "  --events jsonl        Stream JSON events to stderr")
}
