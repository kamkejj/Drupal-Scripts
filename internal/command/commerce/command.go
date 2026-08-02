package commerce

import (
	"fmt"
	"io"
	"strings"

	"dropkit/internal/command/installation"
)

var config = installation.InstallationConfig{
	CommandName:          "commerce",
	Type:                 "commerce",
	ProductName:          "Drupal Commerce",
	MinimumDrupalVersion: 10,
	MaximumDrupalVersion: 11,
	ComposerPackages:     []string{"drupal/commerce:^3.3"},
	EnabledModules: []string{
		"commerce",
		"commerce_cart",
		"commerce_checkout",
		"commerce_order",
		"commerce_store",
		"commerce_price",
		"commerce_tax",
		"commerce_product",
		"commerce_payment",
	},
}

var command = installation.NewCommand(config, PrintUsage)

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return command.Run(args, stdin, stdout, stderr)
}

func PrintUsage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage:")
	fmt.Fprintln(writer, "  dropkit commerce                                                Interactive TUI (terminal only)")
	fmt.Fprintln(writer, "  dropkit commerce plan --name NAME --parent DIR --provider docker|colima --drupal-version 10|11 [options]")
	fmt.Fprintln(writer, "  dropkit commerce apply --plan FILE [approvals] [options]")
	fmt.Fprintln(writer, "  dropkit commerce verify --plan FILE [options]")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Commands:")
	fmt.Fprintln(writer, "  plan      Inspect the host and create a read-only Drupal Commerce installation plan")
	fmt.Fprintln(writer, "  apply     Apply a saved Commerce plan with explicit effect approvals")
	fmt.Fprintln(writer, "  verify    Verify a saved Commerce plan without modifying the host")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Commerce package:")
	for _, packageName := range config.ComposerPackages {
		fmt.Fprintln(writer, "  "+packageName)
	}
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Enabled modules:")
	fmt.Fprintln(writer, "  "+strings.Join(config.EnabledModules, ", "))
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
