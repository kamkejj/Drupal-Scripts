package delete

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func PrintUsage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage: dropkit delete [--yes]")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Delete the DDEV project in the current directory without a database snapshot.")
	fmt.Fprintln(writer, "This removes the project's DDEV data, including its database, but not its source files.")
	fmt.Fprintln(writer, "Requires .ddev/config.yaml in the current directory; does not accept a project name.")
	fmt.Fprintln(writer, "Prompts for confirmation on a terminal. Use --yes for non-interactive approval.")
	fmt.Fprintln(writer, "Runs ddev delete --omit-snapshot --yes after approval; DDEV must be installed.")
	fmt.Fprintln(writer, "Exit codes: 0 success, 2 invalid request or declined confirmation, 1 execution failure.")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Example: cd /path/to/site && dropkit delete --yes")
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run(args, stdin, stdout, stderr, os.Getwd, isTerminal, execute)
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, getwd func() (string, error), terminal func(io.Reader) bool, command func(string, io.Writer, io.Writer) error) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		PrintUsage(stdout)
		return 0
	}
	approved := false
	for _, arg := range args {
		switch arg {
		case "--yes":
			if approved {
				fmt.Fprintln(stderr, "duplicate --yes flag")
				return 2
			}
			approved = true
		default:
			fmt.Fprintf(stderr, "unknown delete argument %q\n", arg)
			PrintUsage(stderr)
			return 2
		}
	}
	dir, err := getwd()
	if err != nil {
		fmt.Fprintf(stderr, "cannot determine current directory: %v\n", err)
		return 1
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(stderr, "cannot resolve current directory: %v\n", err)
		return 1
	}
	config := filepath.Join(dir, ".ddev", "config.yaml")
	info, err := os.Stat(config)
	if err != nil || info.IsDir() {
		fmt.Fprintf(stderr, "no DDEV project at %s (expected .ddev/config.yaml); nothing deleted\n", dir)
		return 2
	}

	if !approved {
		if !terminal(stdin) {
			fmt.Fprintln(stderr, "deletion requires explicit approval; use dropkit delete --yes in non-interactive mode")
			return 2
		}
		fmt.Fprintf(stderr, "Delete the DDEV project at %s without a database snapshot? Type 'delete' to confirm: ", dir)
		answer, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Fprintf(stderr, "cannot read confirmation: %v\n", err)
			return 1
		}
		if strings.TrimSpace(answer) != "delete" {
			fmt.Fprintln(stderr, "deletion cancelled")
			return 2
		}
	}

	if err := command(dir, stdout, stderr); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			fmt.Fprintf(stderr, "ddev delete failed with exit code %d\n", exitError.ExitCode())
			return exitError.ExitCode()
		}
		fmt.Fprintf(stderr, "cannot run ddev delete: %v\n", err)
		return 1
	}
	return 0
}

func isTerminal(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func execute(dir string, stdout, stderr io.Writer) error {
	cmd := exec.Command("ddev", "delete", "--omit-snapshot", "--yes")
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
