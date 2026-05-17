package control

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type AuthUser struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (s *Store) HasAuthUsers(ctx context.Context) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM auth_users)`).Scan(&exists)
	return exists, err
}

func (s *Store) GetAuthUser(ctx context.Context, username string) (AuthUser, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, username, password_hash, created_at, updated_at
		FROM auth_users
		WHERE username=$1`, strings.TrimSpace(username))
	return scanAuthUser(row)
}

func (s *Store) GetAuthSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRow(ctx, `SELECT value FROM auth_settings WHERE key=$1`, key).Scan(&value)
	return value, dbErr(err)
}

func (s *Store) InitializeAuth(ctx context.Context, username, passwordHash, sessionSecret, runnerToken string) error {
	username = strings.TrimSpace(username)
	if username == "" || passwordHash == "" || sessionSecret == "" || runnerToken == "" {
		return ErrValidation
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)

	if _, err := tx.Exec(ctx, `LOCK TABLE auth_users IN EXCLUSIVE MODE`); err != nil {
		return err
	}
	var existing int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM auth_users`).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return ErrInvalidState
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_users (username, password_hash)
		VALUES ($1, $2)`, username, passwordHash); err != nil {
		return dbErr(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_settings (key, value)
		VALUES ('session_secret', $1), ('runner_token', $2)
		ON CONFLICT (key) DO UPDATE
		SET value=EXCLUDED.value, updated_at=now()`, sessionSecret, runnerToken); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
