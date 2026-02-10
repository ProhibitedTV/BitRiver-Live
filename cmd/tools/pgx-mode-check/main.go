package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

func main() {
	expectedDriver := flag.String("expected-storage-driver", "", "storage driver expected by this build (json or postgres)")
	flag.Parse()

	driver := resolveExpectedDriver(*expectedDriver)
	fmt.Printf("pgx.IsStub=%t\n", pgx.IsStub)
	fmt.Printf("expected_storage_driver=%s\n", driver)

	if err := validateDriver(driver, pgx.IsStub); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// resolveExpectedDriver returns the normalized storage driver from the flag,
// then BITRIVER_LIVE_STORAGE_DRIVER, defaulting to json when unset.
func resolveExpectedDriver(flagValue string) string {
	driver := strings.ToLower(strings.TrimSpace(flagValue))
	if driver == "" {
		driver = strings.ToLower(strings.TrimSpace(os.Getenv("BITRIVER_LIVE_STORAGE_DRIVER")))
	}
	if driver == "" {
		driver = "json"
	}
	return driver
}

// validateDriver enforces supported storage driver values and verifies that
// postgres builds do not link against the stub pgx module.
func validateDriver(driver string, isStub bool) error {
	switch driver {
	case "json", "postgres":
	default:
		return fmt.Errorf("expected storage driver must be json or postgres (got %q)", driver)
	}

	if driver == "postgres" && isStub {
		return errors.New("postgres storage requires a non-stub pgx module; configure the release build to use the real pgx source path")
	}
	return nil
}
