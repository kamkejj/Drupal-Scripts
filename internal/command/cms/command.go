package cms

import (
	"fmt"
	"io"
	"strings"

	installconfig "dropkit/config"
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
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	modules, err := installconfig.LoadModules("cms")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	loaded := config
	loaded.ComposerPackages = modules.ComposerPackages
	loaded.EnabledModules = modules.EnabledModules
	return installer.NewCommand(loaded, PrintUsage).Run(args, stdin, stdout, stderr)
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
	fmt.Fprintln(writer, "Composer packages:")
	modules, err := installconfig.LoadModules("cms")
	if err == nil {
		for _, packageName := range modules.ComposerPackages {
			fmt.Fprintln(writer, "  "+packageName)
		}
	}
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Enabled modules:")
	if err == nil {
		fmt.Fprintln(writer, "  "+strings.Join(modules.EnabledModules, ", "))
	}
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Approvals:")
	fmt.Fprintln(writer, "  --allow-network       Allow downloads")
	fmt.Fprintln(writer, "  --allow-host-changes  Allow host package and runtime changes")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Machine output:")
	fmt.Fprintln(writer, "  --output json         Write one JSON document to stdout")
	fmt.Fprintln(writer, "  --events jsonl        Stream JSON events to stderr")
}
