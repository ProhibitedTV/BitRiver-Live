package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type moduleVersion struct {
	Path    string
	Version string
}

type moduleFile struct {
	Replace []struct {
		Old moduleVersion
		New moduleVersion
	}
}

func main() {
	output := flag.String("output", "go.production.mod", "path to the generated production module file")
	flag.Parse()

	if err := prepareProductionModule("go.mod", *output); err != nil {
		fmt.Fprintf(os.Stderr, "prepare production module: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("prepared production module %s\n", *output)
}

func prepareProductionModule(source, output string) (returnErr error) {
	if filepath.Ext(output) != ".mod" {
		return fmt.Errorf("output path %q must end in .mod", output)
	}

	sourceAbs, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}
	outputAbs, err := filepath.Abs(output)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	if sourceAbs == outputAbs {
		return errors.New("output must not overwrite the source go.mod")
	}

	repoRoot := filepath.Dir(sourceAbs)
	parsed, err := readModuleFile(repoRoot, "")
	if err != nil {
		return err
	}

	localModules := make([]string, 0, len(parsed.Replace))
	thirdPartyRoot := filepath.Join(repoRoot, "third_party")
	for _, replacement := range parsed.Replace {
		if replacement.New.Version != "" {
			continue
		}
		target := replacement.New.Path
		if target == "" {
			return fmt.Errorf("replacement for %s has no target", replacement.Old.Path)
		}
		targetAbs := target
		if !filepath.IsAbs(targetAbs) {
			targetAbs = filepath.Join(repoRoot, targetAbs)
		}
		withinThirdParty, err := pathWithin(thirdPartyRoot, targetAbs)
		if err != nil {
			return fmt.Errorf("validate replacement for %s: %w", replacement.Old.Path, err)
		}
		if !withinThirdParty {
			return fmt.Errorf("local replacement for %s points outside third_party: %s", replacement.Old.Path, target)
		}
		localModules = append(localModules, replacement.Old.Path)
	}
	if len(localModules) == 0 {
		return errors.New("source go.mod has no local third_party replacements to remove")
	}

	if err := os.MkdirAll(filepath.Dir(outputAbs), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	outputSum := strings.TrimSuffix(outputAbs, ".mod") + ".sum"
	defer func() {
		if returnErr != nil {
			_ = os.Remove(outputAbs)
			_ = os.Remove(outputSum)
		}
	}()

	if err := copyFile(sourceAbs, outputAbs); err != nil {
		return err
	}
	sourceSum := filepath.Join(repoRoot, "go.sum")
	if err := copyFile(sourceSum, outputSum); err != nil {
		return err
	}

	for _, module := range localModules {
		if err := runGo(repoRoot, "mod", "edit", "-modfile="+outputAbs, "-dropreplace="+module); err != nil {
			return fmt.Errorf("drop replacement for %s: %w", module, err)
		}
	}

	production, err := readModuleFile(repoRoot, outputAbs)
	if err != nil {
		return err
	}
	for _, replacement := range production.Replace {
		if replacement.New.Version == "" {
			return fmt.Errorf("production module retains local replacement for %s", replacement.Old.Path)
		}
	}
	return nil
}

func readModuleFile(dir, modfile string) (moduleFile, error) {
	args := []string{"mod", "edit", "-json"}
	if modfile != "" {
		args = append(args, "-modfile="+modfile)
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return moduleFile{}, fmt.Errorf("go %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	var parsed moduleFile
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return moduleFile{}, fmt.Errorf("decode go mod edit output: %w", err)
	}
	return parsed, nil
}

func runGo(dir string, args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func pathWithin(parent, child string) (bool, error) {
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return false, err
	}
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return false, err
	}
	relative, err := filepath.Rel(parentAbs, childAbs)
	if err != nil {
		return false, err
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)), nil
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer input.Close()

	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", destination, err)
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return fmt.Errorf("copy %s to %s: %w", source, destination, err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close %s: %w", destination, err)
	}
	return nil
}
