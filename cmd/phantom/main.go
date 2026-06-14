// Command phantom is the PhantomTraffic operator CLI. In Plan 2 it exposes a
// single subcommand, `validate`, which statically validates a scenario file.
// It performs no network, traffic, audit, or credential operations.
package main

import (
	"io"
	"os"
	"strconv"
)

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stdout, os.Stderr))
}

// dispatch routes the first argument to a subcommand and returns a process
// exit code. It is a pure function of its arguments and writers so that every
// exit code and output byte is unit-testable without spawning a process.
func dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stdout)
		return 0
	}
	switch args[0] {
	case "", "help", "-h", "--help":
		usage(stdout)
		return 0
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "run":
		return runRun(args[1:], stdout, stderr)
	default:
		// Unknown subcommand is an operator/usage error: usage to stderr, code 2.
		_, _ = io.WriteString(stderr, "phantom: unknown subcommand "+strconv.Quote(args[0])+"\n") // #nosec G705 -- stderr is a CLI stream, not an HTML sink; XSS is inapplicable (arg also control-char-escaped via strconv.Quote)
		usage(stderr)
		return 2
	}
}

// usage writes the top-level usage string to w.
func usage(w io.Writer) {
	const text = `Usage: phantom <command> [flags] [args]

Commands:
  validate <file> [flags]   Statically validate a scenario file.
  run <file> [flags]        Load, validate, and execute a phantom-traffic run.
  help                      Show this help.

Run "phantom <command> -h" for per-command flags.
`
	_, _ = io.WriteString(w, text)
}
