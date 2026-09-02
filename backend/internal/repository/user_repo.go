package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nowdone/internal/models"
)

var ErrNotFound = errors.New("record not found")

// UserRepository provides CRUD access to the users table.
type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

const userColumns = `id, telegram_id, telegram_username, first_name, pin_hash, current_streak, max_streak, timezone, created_at`

func scanUser(row pgx.Row) (*models.User, error) {
	var u models.User
	var username, firstName, pinHash *string
	if err := row.Scan(&u.ID, &u.TelegramID, &username, &firstName, &pinHash, &u.CurrentStreak, &u.MaxStreak, &u.Timezone, &u.CreatedAt); err != nil {
		return nil, err
	}
	if username != nil {
		u.TelegramUsername = *username
	}
	if firstName != nil {
		u.FirstName = *firstName
	}
	if pinHash != nil {
		u.PINHash = *pinHash
	}
	return &u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

func (r *UserRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*models.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE telegram_id = $1`, telegramID)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by telegram id: %w", err)
	}
	return u, nil
}

// GetOrCreateByTelegramID returns the existing user for a Telegram account,
// creating one on first contact with the bot.
func (r *UserRepository) GetOrCreateByTelegramID(ctx context.Context, telegramID int64, username, firstName string) (*models.User, error) {
	u, err := r.GetByTelegramID(ctx, telegramID)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, telegram_username, first_name)
		VALUES ($1, $2, $3)
		RETURNING `+userColumns,
		telegramID, nullIfEmpty(username), nullIfEmpty(firstName),
	)
	return scanUser(row)
}

func (r *UserRepository) SetPINHash(ctx context.Context, userID uuid.UUID, hash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET pin_hash = $1 WHERE id = $2`, hash, userID)
	if err != nil {
		return fmt.Errorf("set pin hash: %w", err)
	}
	return nil
}

// UpdateTimezone stores the user's IANA timezone name. The caller is expected to
// have validated it with time.LoadLocation first.
func (r *UserRepository) UpdateTimezone(ctx context.Context, userID uuid.UUID, tz string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE users SET timezone = $1 WHERE id = $2`, tz, userID)
	if err != nil {
		return fmt.Errorf("update timezone: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepository) UpdateStreak(ctx context.Context, userID uuid.UUID, current, max int) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET current_streak = $1, max_streak = $2 WHERE id = $3`, current, max, userID)
	if err != nil {
		return fmt.Errorf("update streak: %w", err)
	}
	return nil
}

// AllUsers returns every user, used by the hourly streak worker.
func (r *UserRepository) AllUsers(ctx context.Context) ([]*models.User, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+userColumns+` FROM users`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
