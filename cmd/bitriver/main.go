package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"bitriver-live/internal/executil"
)

var (
	Version = "dev"
	Commit  = "dev"
	Date    = "unknown"
)

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
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <command>\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  version   Show BitRiver Live version information")
	fmt.Fprintln(os.Stderr, "  doctor    Check local environment for BitRiver Live")
}

func runVersion(args []string) {
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s version\n", os.Args[0])
	}
	_ = fs.Parse(args)

	printVersionInfo(os.Stdout)
}

func printVersionInfo(out io.Writer) {
	fmt.Fprintf(out, "Version: %s\n", valueOrFallback(Version, "dev"))
	fmt.Fprintf(out, "Commit: %s\n", valueOrFallback(Commit, "unknown"))
	fmt.Fprintf(out, "Date: %s\n", valueOrFallback(Date, "unknown"))
}

func runDoctor(args []string) bool {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s doctor\n", os.Args[0])
	}
	_ = fs.Parse(args)

	fmt.Println("BitRiver Live doctor")
	fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	if cwd, err := os.Getwd(); err == nil {
		fmt.Printf("Working directory: %s\n", cwd)
	} else {
		fmt.Printf("Working directory: (error: %v)\n", err)
	}

	dockerPath, err := executil.LookPath("docker")
	if err != nil {
		fmt.Printf("Docker: not found (%v)\n", err)
		return false
	}
	fmt.Printf("Docker: %s\n", dockerPath)

	fmt.Println()
	fmt.Println("Checking docker version...")
	if err := executil.Run(dockerPath, "version"); err != nil {
		fmt.Printf("docker version failed: %v\n", err)
		return false
	}
	fmt.Println("docker version: ok")

	fmt.Println()
	fmt.Println("Checking docker compose version...")
	if err := executil.Run(dockerPath, "compose", "version"); err != nil {
		fmt.Printf("docker compose version failed: %v\n", err)
		return false
	}
	fmt.Println("docker compose version: ok")

	fmt.Println()
	fmt.Println("All checks passed! You are ready to run BitRiver Live.")
	return true
}

func valueOrFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
