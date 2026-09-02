package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nowdone/internal/models"
)

// AuthCodeRepository provides CRUD access to the auth_codes table.
type AuthCodeRepository struct {
	pool *pgxpool.Pool
}

func NewAuthCodeRepository(pool *pgxpool.Pool) *AuthCodeRepository {
	return &AuthCodeRepository{pool: pool}
}

const authCodeColumns = `id, code, purpose, user_id, telegram_chat_id, is_used, expires_at, created_at`

func scanAuthCode(row pgx.Row) (*models.AuthCode, error) {
	var a models.AuthCode
	if err := row.Scan(&a.ID, &a.Code, &a.Purpose, &a.UserID, &a.TelegramChatID, &a.IsUsed, &a.ExpiresAt, &a.CreatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

// Create inserts a new one-time code. userID is set for reset codes (tied to
// an existing account) and left nil for fresh login codes.
func (r *AuthCodeRepository) Create(ctx context.Context, code string, purpose models.AuthCodePurpose, userID *uuid.UUID, ttl time.Duration) (*models.AuthCode, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO auth_codes (code, purpose, user_id, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING `+authCodeColumns,
		code, purpose, userID, time.Now().Add(ttl),
	)
	return scanAuthCode(row)
}

func (r *AuthCodeRepository) GetByCode(ctx context.Context, code string, purpose models.AuthCodePurpose) (*models.AuthCode, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+authCodeColumns+` FROM auth_codes
		WHERE code = $1 AND purpose = $2
		ORDER BY created_at DESC LIMIT 1`,
		code, purpose,
	)
	a, err := scanAuthCode(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get auth code: %w", err)
	}
	return a, nil
}

// ConfirmByCode is called by the bot once it has delivered the code to the
// user's chat, binding the code to that chat and (for auth codes) that user.
func (r *AuthCodeRepository) ConfirmByCode(ctx context.Context, code string, purpose models.AuthCodePurpose, userID uuid.UUID, chatID int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE auth_codes SET telegram_chat_id = $1, user_id = $2
		WHERE code = $3 AND purpose = $4 AND is_used = false AND expires_at > now()`,
		chatID, userID, code, purpose,
	)
	if err != nil {
		return fmt.Errorf("confirm auth code: %w", err)
	}
	return nil
}

func (r *AuthCodeRepository) MarkUsed(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE auth_codes SET is_used = true WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark auth code used: %w", err)
	}
	return nil
}
