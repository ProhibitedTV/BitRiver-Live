package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

func main() {
	expectedDriver := flag.String("expected-storage-driver", "", "storage driver expected by this build (postgres only)")
	flag.Parse()

	driver := resolveExpectedDriver(*expectedDriver)
	fmt.Printf("pgx.ErrNoRows=%q\n", pgx.ErrNoRows)
	fmt.Printf("expected_storage_driver=%s\n", driver)

	if err := validateDriver(driver); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// resolveExpectedDriver returns the normalized storage driver from the flag,
// defaulting to postgres when unset.
func resolveExpectedDriver(flagValue string) string {
	driver := strings.ToLower(strings.TrimSpace(flagValue))
	if driver == "" {
		driver = "postgres"
	}
	return driver
}

// validateDriver enforces supported storage driver values for release builds.
func validateDriver(driver string) error {
	switch driver {
	case "postgres":
	default:
		return fmt.Errorf("expected storage driver must be postgres (got %q)", driver)
	}
	return nil
}
