package main

import (
	"debug/buildinfo"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

type stringList []string

func (values *stringList) String() string { return strings.Join(*values, ",") }

func (values *stringList) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("required module path must not be empty")
	}
	*values = append(*values, value)
	return nil
}

func main() {
	var requiredModules stringList
	flag.Var(&requiredModules, "require-module", "module path that must be linked from an upstream version (repeatable)")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: verify-production-binary [--require-module module] <binary>")
		os.Exit(2)
	}

	info, err := buildinfo.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read Go build metadata: %v\n", err)
		os.Exit(1)
	}
	if err := verifyBuildInfo(info, requiredModules); err != nil {
		fmt.Fprintf(os.Stderr, "verify production binary: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("production dependency metadata verified: %s\n", flag.Arg(0))
}

func verifyBuildInfo(info *debug.BuildInfo, requiredModules []string) error {
	linked := make(map[string]bool, len(info.Deps))
	for _, dependency := range info.Deps {
		linked[dependency.Path] = true
		if dependency.Replace == nil {
			continue
		}
		if isLocalReplacement(dependency.Replace.Path, dependency.Replace.Version) {
			return fmt.Errorf("module %s is linked from local replacement %s", dependency.Path, dependency.Replace.Path)
		}
	}
	for _, required := range requiredModules {
		if !linked[required] {
			return fmt.Errorf("required module %s is not linked", required)
		}
	}
	return nil
}

func isLocalReplacement(path, version string) bool {
	if version == "" || version == "(devel)" {
		return true
	}
	clean := filepath.ToSlash(path)
	return strings.HasPrefix(clean, "./") || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/third_party/")
}
