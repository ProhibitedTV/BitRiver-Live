package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"bitriver-live/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateUser creates user and returns an error when persistence or validation fails.
func (r *postgresRepository) CreateUser(params CreateUserParams) (domain.User, error) {
	if r == nil || r.pool == nil {
		return domain.User{}, ErrPostgresUnavailable
	}

	normalizedEmail := strings.TrimSpace(strings.ToLower(params.Email))
	if normalizedEmail == "" {
		return domain.User{}, fmt.Errorf("email is required")
	}

	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		return domain.User{}, fmt.Errorf("displayName is required")
	}

	roles := normalizeRoles(params.Roles)
	if roles == nil {
		roles = []string{}
	}
	if params.SelfSignup {
		if params.Password == "" {
			return domain.User{}, fmt.Errorf("password is required for self-service signup")
		}
		if len(roles) == 0 {
			roles = []string{"viewer"}
		}
	}

	id, err := generateID()
	if err != nil {
		return domain.User{}, err
	}

	var passwordHash string
	if params.Password != "" {
		hashed, hashErr := hashPassword(params.Password)
		if hashErr != nil {
			return domain.User{}, fmt.Errorf("hash password: %w", hashErr)
		}
		passwordHash = hashed
	}

	var createdAt time.Time
	createErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin create user tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var existingID string
		err = tx.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", normalizedEmail).Scan(&existingID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check existing email: %w", err)
		}
		if err == nil {
			return fmt.Errorf("email %s already in use", params.Email)
		}

		err = tx.QueryRow(ctx, "INSERT INTO users (id, display_name, email, roles, password_hash, self_signup) VALUES ($1, $2, $3, $4, $5, $6) RETURNING created_at", id, displayName, normalizedEmail, roles, passwordHash, params.SelfSignup).Scan(&createdAt)
		if err != nil {
			return fmt.Errorf("insert user: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit create user: %w", err)
		}
		return nil
	})
	if createErr != nil {
		return domain.User{}, createErr
	}

	return domain.User{
		ID:           id,
		DisplayName:  displayName,
		Email:        normalizedEmail,
		Roles:        roles,
		PasswordHash: passwordHash,
		SelfSignup:   params.SelfSignup,
		CreatedAt:    createdAt.UTC(),
	}, nil
}

// AuthenticateUser performs authenticate user and returns an error when dependent systems reject the operation.
func (r *postgresRepository) AuthenticateUser(email, password string) (domain.User, error) {
	if password == "" {
		return domain.User{}, fmt.Errorf("password is required")
	}
	if r == nil || r.pool == nil {
		return domain.User{}, ErrPostgresUnavailable
	}

	trimmedEmail := strings.TrimSpace(strings.ToLower(email))
	var user domain.User
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		row := conn.QueryRow(ctx, "SELECT id, display_name, email, roles, password_hash, self_signup, created_at FROM users WHERE email = $1", trimmedEmail)
		scanned, scanErr := scanUser(row)
		if scanErr != nil {
			return scanErr
		}
		user = scanned
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("authenticate user: %w", err)
	}
	if user.PasswordHash == "" {
		return domain.User{}, ErrPasswordLoginUnsupported
	}
	if err := verifyPassword(user.PasswordHash, password); err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return domain.User{}, ErrInvalidCredentials
		}
		return domain.User{}, err
	}
	return user, nil
}

// ListUsers returns users from the configured backing services.
func (r *postgresRepository) ListUsers() []domain.User {
	if r == nil || r.pool == nil {
		return nil
	}

	var users []domain.User
	listErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		rows, err := conn.Query(ctx, "SELECT id, display_name, email, roles, password_hash, self_signup, created_at FROM users ORDER BY created_at ASC")
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			user, scanErr := scanUser(rows)
			if scanErr != nil {
				return scanErr
			}
			users = append(users, user)
		}
		return rows.Err()
	})
	if listErr != nil {
		return nil
	}
	return users
}

// GetUser returns user from the configured backing services.
func (r *postgresRepository) GetUser(id string) (domain.User, bool) {
	if r == nil || r.pool == nil {
		return domain.User{}, false
	}

	var user domain.User
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		row := conn.QueryRow(ctx, "SELECT id, display_name, email, roles, password_hash, self_signup, created_at FROM users WHERE id = $1", id)
		scanned, scanErr := scanUser(row)
		if scanErr != nil {
			return scanErr
		}
		user = scanned
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, false
	}
	if err != nil {
		return domain.User{}, false
	}
	return user, true
}

// UpdateUser updates user and returns an error when persistence or validation fails.
func (r *postgresRepository) UpdateUser(id string, update UserUpdate) (domain.User, error) {
	if r == nil || r.pool == nil {
		return domain.User{}, ErrPostgresUnavailable
	}

	var updated domain.User
	updateErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin update user tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		row := tx.QueryRow(ctx, "SELECT id, display_name, email, roles, password_hash, self_signup, created_at FROM users WHERE id = $1 FOR UPDATE", id)
		user, err := scanUser(row)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("user %s not found", id)
		}
		if err != nil {
			return fmt.Errorf("load user %s: %w", id, err)
		}

		if update.DisplayName != nil {
			name := strings.TrimSpace(*update.DisplayName)
			if name == "" {
				return fmt.Errorf("displayName cannot be empty")
			}
			user.DisplayName = name
		}

		if update.Email != nil {
			email := strings.TrimSpace(strings.ToLower(*update.Email))
			if email == "" {
				return fmt.Errorf("email cannot be empty")
			}
			var existingID string
			err = tx.QueryRow(ctx, "SELECT id FROM users WHERE email = $1 AND id <> $2", email, id).Scan(&existingID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("check email uniqueness: %w", err)
			}
			if err == nil {
				return fmt.Errorf("email %s already in use", email)
			}
			user.Email = email
		}

		if update.Roles != nil {
			user.Roles = normalizeRoles(*update.Roles)
			if user.Roles == nil {
				user.Roles = []string{}
			}
		}

		_, err = tx.Exec(ctx, "UPDATE users SET display_name = $1, email = $2, roles = $3 WHERE id = $4", user.DisplayName, user.Email, user.Roles, id)
		if err != nil {
			return fmt.Errorf("update user %s: %w", id, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit update user: %w", err)
		}

		updated = user
		return nil
	})
	if updateErr != nil {
		return domain.User{}, updateErr
	}

	return updated, nil
}

// SetUserPassword parses and stores a flag assignment, returning an error when the format is invalid.
func (r *postgresRepository) SetUserPassword(id, password string) (domain.User, error) {
	if r == nil || r.pool == nil {
		return domain.User{}, ErrPostgresUnavailable
	}
	if len(password) < 8 {
		return domain.User{}, fmt.Errorf("password must be at least 8 characters")
	}

	hashed, err := hashPassword(password)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash password: %w", err)
	}

	var user domain.User
	var roles []string
	updateErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		row := conn.QueryRow(ctx, "UPDATE users SET password_hash = $1 WHERE id = $2 RETURNING id, display_name, email, roles, password_hash, self_signup, created_at", hashed, id)
		if err := row.Scan(&user.ID, &user.DisplayName, &user.Email, &roles, &user.PasswordHash, &user.SelfSignup, &user.CreatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("user %s not found", id)
			}
			return fmt.Errorf("update user password: %w", err)
		}
		return nil
	})
	if updateErr != nil {
		return domain.User{}, updateErr
	}

	user.Roles = roles
	return user, nil
}

// DeleteUser deletes user and returns an error when persistence or validation fails.
func (r *postgresRepository) DeleteUser(id string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}

	deleteErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin delete user tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var userExists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)", id).Scan(&userExists); err != nil {
			return fmt.Errorf("check user %s existence: %w", id, err)
		}
		if !userExists {
			return fmt.Errorf("user %s not found", id)
		}

		var ownedChannelID string
		err = tx.QueryRow(ctx, "SELECT id FROM channels WHERE owner_id = $1 LIMIT 1", id).Scan(&ownedChannelID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check owned channels: %w", err)
		}
		if err == nil {
			return fmt.Errorf("user %s owns channel %s; transfer or delete the channel first", id, ownedChannelID)
		}

		if _, err := tx.Exec(ctx, "UPDATE profiles SET top_friends = array_remove(top_friends, $1), updated_at = NOW() WHERE $1 = ANY(top_friends)", id); err != nil {
			return fmt.Errorf("remove user %s from top friends: %w", id, err)
		}

		if _, err := tx.Exec(ctx, "DELETE FROM users WHERE id = $1", id); err != nil {
			return fmt.Errorf("delete user %s: %w", id, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit delete user: %w", err)
		}

		return nil
	})
	if deleteErr != nil {
		return deleteErr
	}

	return nil
}

// scanUser scans user from database rows and returns an error when type conversion fails.
func scanUser(row pgx.Row) (domain.User, error) {
	var (
		id, displayName, email string
		roles                  []string
		passwordHash           pgtype.Text
		selfSignup             bool
		createdAt              time.Time
	)
	if err := row.Scan(&id, &displayName, &email, &roles, &passwordHash, &selfSignup, &createdAt); err != nil {
		return domain.User{}, err
	}
	user := domain.User{
		ID:          id,
		DisplayName: displayName,
		Email:       email,
		Roles:       rolesFromDB(roles),
		SelfSignup:  selfSignup,
		CreatedAt:   createdAt.UTC(),
	}
	if passwordHash.Valid {
		user.PasswordHash = passwordHash.String
	}
	return user, nil
}

// rolesFromDB performs roles from db and propagates validation or dependency failures to the caller.
func rolesFromDB(roles []string) []string {
	if len(roles) == 0 {
		return nil
	}
	cloned := append([]string(nil), roles...)
	return cloned
}
