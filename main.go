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
		if len(args) > 1 {
			fmt.Fprintln(stderr, "install does not accept arguments")
			printInstallUsage(stderr)
			return 1
		}
		return runInstall()
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
	fmt.Fprintln(writer, "Usage: dropkit install")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Install a Drupal 11 development environment.")
}
