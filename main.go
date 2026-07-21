package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	if exitCode := runCLI(os.Args[1:], os.Stdout, os.Stderr); exitCode != 0 {
		os.Exit(exitCode)
	}
}

func runCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 1
	}

	switch args[0] {
	case "help", "-h", "--help":
		if len(args) == 1 {
			printUsage(stdout)
			return 0
		}
		if len(args) == 2 && args[1] == "install" {
			printInstallUsage(stdout)
			return 0
		}
		fmt.Fprintf(stderr, "unknown help topic %q\n\n", args[1])
		printUsage(stderr)
		return 1
	case "install":
		if len(args) == 2 && (args[1] == "-h" || args[1] == "--help") {
			printInstallUsage(stdout)
			return 0
		}
		return runInstallCommand(args[1:], os.Stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 1
	}
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage: dropkit <command>")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Commands:")
	fmt.Fprintln(writer, "  install    Install a Drupal 11 development environment")
	fmt.Fprintln(writer, "  help       Show help for a command")
}

func printInstallUsage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage:")
	fmt.Fprintln(writer, "  dropkit install                                                Interactive TUI (terminal only)")
	fmt.Fprintln(writer, "  dropkit install plan --name NAME --parent DIR --provider docker|colima [options]")
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
