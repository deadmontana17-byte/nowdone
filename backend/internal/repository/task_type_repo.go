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

// TaskTypeRepository provides CRUD access to the task_types table.
type TaskTypeRepository struct {
	pool *pgxpool.Pool
}

func NewTaskTypeRepository(pool *pgxpool.Pool) *TaskTypeRepository {
	return &TaskTypeRepository{pool: pool}
}

const taskTypeColumns = `id, user_id, emoji, name, created_at`

func scanTaskType(row pgx.Row) (*models.TaskType, error) {
	var t models.TaskType
	if err := row.Scan(&t.ID, &t.UserID, &t.Emoji, &t.Name, &t.CreatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TaskTypeRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*models.TaskType, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+taskTypeColumns+` FROM task_types WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list task types: %w", err)
	}
	defer rows.Close()

	result := make([]*models.TaskType, 0)
	for rows.Next() {
		t, err := scanTaskType(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (r *TaskTypeRepository) Create(ctx context.Context, userID uuid.UUID, emoji, name string) (*models.TaskType, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO task_types (user_id, emoji, name) VALUES ($1, $2, $3)
		RETURNING `+taskTypeColumns,
		userID, emoji, name,
	)
	return scanTaskType(row)
}

func (r *TaskTypeRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM task_types WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete task type: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByID is used to validate ownership before deletion elsewhere if needed.
func (r *TaskTypeRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.TaskType, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+taskTypeColumns+` FROM task_types WHERE id = $1`, id)
	t, err := scanTaskType(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}
