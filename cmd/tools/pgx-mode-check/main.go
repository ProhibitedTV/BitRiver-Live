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
	allowStub := flag.Bool("allow-stub-for-postgres", false, "allow postgres expectation to pass even when pgx is stubbed")
	flag.Parse()

	driver := resolveExpectedDriver(*expectedDriver)
	fmt.Printf("pgx.IsStub=%t\n", pgx.IsStub)
	fmt.Printf("expected_storage_driver=%s\n", driver)

	if err := validateDriver(driver, pgx.IsStub, *allowStub); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if driver == "postgres" && pgx.IsStub && *allowStub {
		fmt.Fprintln(os.Stderr, "warning: postgres storage expected but pgx is stubbed; this build cannot run postgres storage until a non-stub pgx source is configured")
	}
}

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

func validateDriver(driver string, isStub bool, allowStub bool) error {
	switch driver {
	case "json", "postgres":
	default:
		return fmt.Errorf("expected storage driver must be json or postgres (got %q)", driver)
	}

	if driver == "postgres" && isStub && !allowStub {
		return errors.New("postgres storage requires a non-stub pgx module; configure the release build to use the real pgx source path")
	}
	return nil
}
