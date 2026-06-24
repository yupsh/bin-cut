// Command yup-cut is the CLI wrapper around github.com/gloo-foo/cmd-cut.
package main

import (
	"os"

	"github.com/spf13/afero"
)

// appVersion is the binary's version string. It defaults to "dev" for local
// builds and is overridden at release time via the linker:
// -ldflags "-X main.appVersion=<version>" (set by goreleaser).
var appVersion = "dev"

// Indirections so main's wiring (version, args → run → exit code) is testable
// without spawning a subprocess. Overridden in tests, restored after.
var (
	osExit = os.Exit
	runCLI = run
)

func main() {
	osExit(runCLI(appVersion, os.Args, os.Stdin, os.Stdout, os.Stderr, afero.NewOsFs()))
}
