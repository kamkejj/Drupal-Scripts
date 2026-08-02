package drupal

import (
	"fmt"
	"io"

	"dropkit/internal/installer"
)

var config = installer.InstallationConfig{
	CommandName:          "install",
	Type:                 "drupal",
	ProductName:          "Drupal",
	MinimumDrupalVersion: 8,
	MaximumDrupalVersion: 12,
}

var command = installer.NewCommand(config, PrintUsage)

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return command.Run(args, stdin, stdout, stderr)
}

func PrintUsage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage:")
	fmt.Fprintln(writer, "  dropkit install                                                Interactive TUI (terminal only)")
	fmt.Fprintln(writer, "  dropkit install plan --name NAME --parent DIR --provider docker|colima --drupal-version VERSION [options]")
	fmt.Fprintln(writer, "  dropkit install apply --plan FILE [approvals] [options]")
	fmt.Fprintln(writer, "  dropkit install verify --plan FILE [options]")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Commands:")
	fmt.Fprintln(writer, "  plan      Inspect the host and create a read-only installation plan")
	fmt.Fprintln(writer, "  apply     Apply a saved plan with explicit effect approvals")
	fmt.Fprintln(writer, "  verify    Verify a saved plan without modifying the host")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Approvals:")
	fmt.Fprintln(writer, "  --allow-network       Allow downloads")
	fmt.Fprintln(writer, "  --allow-host-changes  Allow host package and runtime changes")
	fmt.Fprintln(writer, "  --allow-destructive   Allow destructive project operations")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Machine output:")
	fmt.Fprintln(writer, "  --output json         Write one JSON document to stdout")
	fmt.Fprintln(writer, "  --events jsonl        Stream JSON events to stderr")
}
