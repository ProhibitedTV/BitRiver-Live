package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"bitriver-live/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (r *postgresRepository) CreateUser(params CreateUserParams) (models.User, error) {
	if r == nil || r.pool == nil {
		return models.User{}, ErrPostgresUnavailable
	}

	normalizedEmail := strings.TrimSpace(strings.ToLower(params.Email))
	if normalizedEmail == "" {
		return models.User{}, fmt.Errorf("email is required")
	}

	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		return models.User{}, fmt.Errorf("displayName is required")
	}

	roles := normalizeRoles(params.Roles)
	if roles == nil {
		roles = []string{}
	}
	if params.SelfSignup {
		if params.Password == "" {
			return models.User{}, fmt.Errorf("password is required for self-service signup")
		}
		if len(roles) == 0 {
			roles = []string{"viewer"}
		}
	}

	id, err := generateID()
	if err != nil {
		return models.User{}, err
	}

	var passwordHash string
	if params.Password != "" {
		hashed, hashErr := hashPassword(params.Password)
		if hashErr != nil {
			return models.User{}, fmt.Errorf("hash password: %w", hashErr)
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
		return models.User{}, createErr
	}

	return models.User{
		ID:           id,
		DisplayName:  displayName,
		Email:        normalizedEmail,
		Roles:        roles,
		PasswordHash: passwordHash,
		SelfSignup:   params.SelfSignup,
		CreatedAt:    createdAt.UTC(),
	}, nil
}

func (r *postgresRepository) AuthenticateUser(email, password string) (models.User, error) {
	if password == "" {
		return models.User{}, fmt.Errorf("password is required")
	}
	if r == nil || r.pool == nil {
		return models.User{}, ErrPostgresUnavailable
	}

	trimmedEmail := strings.TrimSpace(strings.ToLower(email))
	var user models.User
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
		return models.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return models.User{}, fmt.Errorf("authenticate user: %w", err)
	}
	if user.PasswordHash == "" {
		return models.User{}, ErrPasswordLoginUnsupported
	}
	if err := verifyPassword(user.PasswordHash, password); err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return models.User{}, ErrInvalidCredentials
		}
		return models.User{}, err
	}
	return user, nil
}

func (r *postgresRepository) ListUsers() []models.User {
	if r == nil || r.pool == nil {
		return nil
	}

	var users []models.User
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

func (r *postgresRepository) GetUser(id string) (models.User, bool) {
	if r == nil || r.pool == nil {
		return models.User{}, false
	}

	var user models.User
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
		return models.User{}, false
	}
	if err != nil {
		return models.User{}, false
	}
	return user, true
}

func (r *postgresRepository) UpdateUser(id string, update UserUpdate) (models.User, error) {
	if r == nil || r.pool == nil {
		return models.User{}, ErrPostgresUnavailable
	}

	var updated models.User
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
		return models.User{}, updateErr
	}

	return updated, nil
}

func (r *postgresRepository) SetUserPassword(id, password string) (models.User, error) {
	if r == nil || r.pool == nil {
		return models.User{}, ErrPostgresUnavailable
	}
	if len(password) < 8 {
		return models.User{}, fmt.Errorf("password must be at least 8 characters")
	}

	hashed, err := hashPassword(password)
	if err != nil {
		return models.User{}, fmt.Errorf("hash password: %w", err)
	}

	var user models.User
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
		return models.User{}, updateErr
	}

	user.Roles = roles
	return user, nil
}

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

func scanUser(row pgx.Row) (models.User, error) {
	var (
		id, displayName, email string
		roles                  []string
		passwordHash           pgtype.Text
		selfSignup             bool
		createdAt              time.Time
	)
	if err := row.Scan(&id, &displayName, &email, &roles, &passwordHash, &selfSignup, &createdAt); err != nil {
		return models.User{}, err
	}
	user := models.User{
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

func rolesFromDB(roles []string) []string {
	if len(roles) == 0 {
		return nil
	}
	cloned := append([]string(nil), roles...)
	return cloned
}
