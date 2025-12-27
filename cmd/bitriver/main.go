package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

var (
	version = "dev"
	commit  = "dev"
	date    = "dev"
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
		runDoctor(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  version   Show BitRiver Live version information")
	fmt.Fprintln(os.Stderr, "  doctor    Run environment diagnostics")
}

func runVersion(args []string) {
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s version\n", os.Args[0])
	}
	_ = fs.Parse(args)

	fmt.Printf("Version: %s\n", versionValue(version))
	fmt.Printf("Commit: %s\n", versionValue(commit))
	fmt.Printf("Date: %s\n", versionValue(date))
}

func runDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s doctor\n", os.Args[0])
	}
	_ = fs.Parse(args)

	fmt.Println("BitRiver Live environment check")

	dockerPath, dockerErr := exec.LookPath("docker")
	if dockerErr != nil {
		fmt.Printf("- docker in PATH: no (%v)\n", dockerErr)
	} else {
		fmt.Printf("- docker in PATH: yes (%s)\n", dockerPath)
	}

	dockerVersionOutput, dockerVersionErr := runCommandOutput(dockerPath, dockerErr, "version")
	if dockerVersionErr != nil {
		fmt.Printf("- docker version: failed (%v)\n", dockerVersionErr)
		if len(dockerVersionOutput) > 0 {
			fmt.Println(indentOutput(dockerVersionOutput))
		}
	} else {
		fmt.Println("- docker version: ok")
	}

	composeOutput, composeErr := runCommandOutput(dockerPath, dockerErr, "compose", "version")
	if composeErr != nil {
		fmt.Printf("- docker compose version: failed (%v)\n", composeErr)
		if len(composeOutput) > 0 {
			fmt.Println(indentOutput(composeOutput))
		}
	} else {
		fmt.Println("- docker compose version: ok")
	}

	fmt.Printf("- OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("- Working directory: error (%v)\n", err)
	} else {
		fmt.Printf("- Working directory: %s\n", cwd)
	}
}

func runCommandOutput(binaryPath string, lookupErr error, args ...string) (string, error) {
	if lookupErr != nil {
		return "", lookupErr
	}

	cmd := exec.Command(binaryPath, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return buf.String(), err
	}

	return buf.String(), nil
}

func indentOutput(output string) string {
	if output == "" {
		return ""
	}

	var buf bytes.Buffer
	for _, line := range bytes.Split([]byte(output), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		buf.WriteString("    ")
		buf.Write(line)
		buf.WriteByte('\n')
	}

	return buf.String()
}

func versionValue(value string) string {
	if value == "" {
		return "dev"
	}

	return value
}
