package main

import (
	"fmt"
	"io"
	"os"

	"dropkit/internal/command/commerce"
	"dropkit/internal/command/install"
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
			install.PrintUsage(stdout)
			return 0
		}
		if len(args) == 2 && args[1] == "commerce" {
			commerce.PrintUsage(stdout)
			return 0
		}
		fmt.Fprintf(stderr, "unknown help topic %q\n\n", args[1])
		printUsage(stderr)
		return 1
	case "install":
		if len(args) == 2 && (args[1] == "-h" || args[1] == "--help") {
			install.PrintUsage(stdout)
			return 0
		}
		return install.Run(args[1:], os.Stdin, stdout, stderr)
	case "commerce":
		if len(args) == 2 && (args[1] == "-h" || args[1] == "--help") {
			commerce.PrintUsage(stdout)
			return 0
		}
		return commerce.Run(args[1:], os.Stdin, stdout, stderr)
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
	fmt.Fprintln(writer, "  install    Install a Drupal 8-12 development environment")
	fmt.Fprintln(writer, "  commerce   Install Drupal Commerce on Drupal 10 or 11")
	fmt.Fprintln(writer, "  help       Show help for a command")
}
