package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nowdone/internal/models"
)

// NoteRepository provides CRUD access to the notes table.
type NoteRepository struct {
	pool *pgxpool.Pool
}

func NewNoteRepository(pool *pgxpool.Pool) *NoteRepository {
	return &NoteRepository{pool: pool}
}

const noteColumns = `id, user_id, title, content, attachments, is_hidden, created_at`

func scanNote(row pgx.Row) (*models.Note, error) {
	var n models.Note
	var content, attachments []byte
	if err := row.Scan(&n.ID, &n.UserID, &n.Title, &content, &attachments, &n.IsHidden, &n.CreatedAt); err != nil {
		return nil, err
	}
	n.Content = models.JSONRaw(content)
	if err := json.Unmarshal(attachments, &n.Attachments); err != nil {
		return nil, fmt.Errorf("unmarshal attachments: %w", err)
	}
	return &n, nil
}

func (r *NoteRepository) ListByUser(ctx context.Context, userID uuid.UUID, includeHidden bool) ([]*models.Note, error) {
	query := `SELECT ` + noteColumns + ` FROM notes WHERE user_id = $1`
	if !includeHidden {
		query += ` AND is_hidden = false`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	result := make([]*models.Note, 0)
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, rows.Err()
}

func (r *NoteRepository) GetByID(ctx context.Context, userID, id uuid.UUID) (*models.Note, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+noteColumns+` FROM notes WHERE id = $1 AND user_id = $2`, id, userID)
	n, err := scanNote(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get note: %w", err)
	}
	return n, nil
}

func (r *NoteRepository) Create(ctx context.Context, n *models.Note) (*models.Note, error) {
	attachments, err := json.Marshal(n.Attachments)
	if err != nil {
		return nil, fmt.Errorf("marshal attachments: %w", err)
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO notes (user_id, title, content, attachments, is_hidden)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+noteColumns,
		n.UserID, n.Title, n.Content, attachments, n.IsHidden,
	)
	return scanNote(row)
}

func (r *NoteRepository) Update(ctx context.Context, n *models.Note) (*models.Note, error) {
	attachments, err := json.Marshal(n.Attachments)
	if err != nil {
		return nil, fmt.Errorf("marshal attachments: %w", err)
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE notes SET title = $1, content = $2, attachments = $3, is_hidden = $4
		WHERE id = $5 AND user_id = $6
		RETURNING `+noteColumns,
		n.Title, n.Content, attachments, n.IsHidden, n.ID, n.UserID,
	)
	note, err := scanNote(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return note, err
}

func (r *NoteRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM notes WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AttachmentsByID returns just the attachment list for one note. Used before
// deleting a note so its files can be purged from object storage first.
func (r *NoteRepository) AttachmentsByID(ctx context.Context, userID, id uuid.UUID) ([]models.Attachment, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx,
		`SELECT attachments FROM notes WHERE id = $1 AND user_id = $2`, id, userID,
	).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get note attachments: %w", err)
	}

	var attachments []models.Attachment
	if err := json.Unmarshal(raw, &attachments); err != nil {
		return nil, fmt.Errorf("unmarshal attachments: %w", err)
	}
	return attachments, nil
}
