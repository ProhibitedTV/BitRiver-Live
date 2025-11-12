// Command bootstrap-admin seeds or updates an administrator account in the datastore.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"bitriver-live/internal/models"
	"bitriver-live/internal/storage"
)

func main() {
	var (
		jsonPath    string
		postgresDSN string
		email       string
		displayName string
		password    string
	)

	flag.StringVar(&jsonPath, "json", "", "Path to the JSON datastore (store.json)")
	flag.StringVar(&postgresDSN, "postgres-dsn", "", "Postgres connection string")
	flag.StringVar(&email, "email", "", "Email address for the admin account")
	flag.StringVar(&displayName, "name", "Administrator", "Display name for the admin account")
	flag.StringVar(&password, "password", "", "Password for the admin account")
	flag.Parse()

	if jsonPath == "" && postgresDSN == "" {
		fatalf("either --json or --postgres-dsn must be provided")
	}
	if jsonPath != "" && postgresDSN != "" {
		fatalf("only one datastore option may be provided")
	}
	if strings.TrimSpace(email) == "" {
		fatalf("--email is required")
	}
	if len(password) < 8 {
		fatalf("--password must be at least 8 characters")
	}
	if strings.TrimSpace(displayName) == "" {
		fatalf("--name cannot be empty")
	}

	repo, err := openRepository(jsonPath, postgresDSN)
	if err != nil {
		fatalf("open datastore: %v", err)
	}
	defer closeRepository(repo)

	email = strings.TrimSpace(email)
	displayName = strings.TrimSpace(displayName)

	user, created, err := bootstrapAdmin(repo, email, displayName, password)
	if err != nil {
		fatalf("bootstrap admin: %v", err)
	}

	state := "updated"
	if created {
		state = "created"
	}
	fmt.Printf("Admin user %s (%s) %s successfully.\n", user.Email, user.DisplayName, state)
	fmt.Println("Remember to rotate this password after the first login.")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func openRepository(jsonPath, postgresDSN string) (storage.Repository, error) {
	if jsonPath != "" {
		return storage.NewJSONRepository(jsonPath)
	}
	return storage.NewPostgresRepository(postgresDSN)
}

func closeRepository(repo storage.Repository) {
	type closer interface {
		Close(context.Context) error
	}
	if c, ok := repo.(closer); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.Close(ctx)
	}
}

func bootstrapAdmin(repo storage.Repository, email, displayName, password string) (models.User, bool, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	users := repo.ListUsers()
	for _, existing := range users {
		if existing.Email == normalizedEmail {
			return updateAdmin(repo, existing, displayName, password)
		}
	}

	user, err := repo.CreateUser(storage.CreateUserParams{
		DisplayName: displayName,
		Email:       normalizedEmail,
		Roles:       []string{"admin"},
		Password:    password,
	})
	if err != nil {
		return models.User{}, false, err
	}
	return user, true, nil
}

func updateAdmin(repo storage.Repository, existing models.User, displayName, password string) (models.User, bool, error) {
	roles := ensureAdminRole(existing.Roles)

	var update storage.UserUpdate
	if existing.DisplayName != displayName {
		update.DisplayName = &displayName
	}
	if !equalStringSlices(existing.Roles, roles) {
		update.Roles = &roles
	}

	updated := existing
	var err error
	if update.DisplayName != nil || update.Roles != nil {
		updated, err = repo.UpdateUser(existing.ID, update)
		if err != nil {
			return models.User{}, false, err
		}
	}

	updated, err = repo.SetUserPassword(existing.ID, password)
	if err != nil {
		return models.User{}, false, err
	}
	return updated, false, nil
}

func ensureAdminRole(existing []string) []string {
	seen := make(map[string]struct{})
	for _, role := range existing {
		trimmed := strings.TrimSpace(role)
		if trimmed == "" {
			continue
		}
		normalized := strings.ToLower(trimmed)
		seen[normalized] = struct{}{}
	}
	seen["admin"] = struct{}{}
	roles := make([]string, 0, len(seen))
	for role := range seen {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
