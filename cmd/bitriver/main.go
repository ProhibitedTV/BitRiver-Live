package main

import (
	"fmt"
	"os"
)

// Version metadata is stamped at build time by release automation.
// Keeping these in a tiny file makes it straightforward for new contributors
// to locate the command entrypoint and understand where build information comes
// from before diving into command-specific implementations.
var (
	Version = "dev"
	Commit  = "dev"
	Date    = "unknown"
)

// cachedRepoRoot memoizes repoRoot() lookups across subcommands.
var cachedRepoRoot string

// main is intentionally lightweight: it dispatches to subcommands implemented
// in the other files in this package.
func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version":
		runVersion(os.Args[2:])
	case "doctor":
		if !runDoctor(os.Args[2:]) {
			os.Exit(1)
		}
	case "env":
		if err := runEnv(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "compose":
		if err := runCompose(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "desktop":
		if err := runDesktop(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "install":
		if err := runInstall(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "quickstart":
		if err := runQuickstart(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "ome":
		if err := runOME(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

// usage prints the top-level CLI help. Subcommands define additional help in
// their own flag sets.
func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <command>\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  version   Show BitRiver Live version information")
	fmt.Fprintln(os.Stderr, "  doctor    Check local environment for BitRiver Live")
	fmt.Fprintln(os.Stderr, "  env       Initialize or validate environment files")
	fmt.Fprintln(os.Stderr, "  compose   Run docker compose up/down with defaults")
	fmt.Fprintln(os.Stderr, "  desktop   Launch the Docker Compose control panel with tray shortcuts")
	fmt.Fprintln(os.Stderr, "  install   Stage binaries/configs and emit service templates")
	fmt.Fprintln(os.Stderr, "  quickstart  Run doctor, env init/validate, render OME config, migrations, and compose up")
	fmt.Fprintln(os.Stderr, "  ome       Render OME configuration from .env")
}
