// Command phantom is the PhantomTraffic operator CLI. In Plan 2 it exposes a
// single subcommand, `validate`, which statically validates a scenario file.
// It performs no network, traffic, audit, or credential operations.
package main

import (
	"io"
	"os"
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
	default:
		// Unknown subcommand is an operator/usage error: usage to stderr, code 2.
		_, _ = io.WriteString(stderr, "phantom: unknown subcommand "+quote(args[0])+"\n")
		usage(stderr)
		return 2
	}
}

// usage writes the top-level usage string to w.
func usage(w io.Writer) {
	const text = `Usage: phantom <command> [flags] [args]

Commands:
  validate <file> [flags]   Statically validate a scenario file.
  help                      Show this help.

Run "phantom validate -h" for validate flags.
`
	_, _ = io.WriteString(w, text)
}

// quote wraps s in double quotes for stable, injection-free error messages.
// It avoids fmt to keep the dispatch hot path dependency-free; s is an
// operator-supplied subcommand token, never a secret.
func quote(s string) string {
	return "\"" + s + "\""
}
