package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"nowdone/internal/models"
	"nowdone/internal/repository"
)

// NoteService implements CRUD for global notes, including the hidden/PIN-gated ones.
type NoteService struct {
	notes *repository.NoteRepository
	s3    *S3Service // nil in processes without object storage (e.g. the bot)
	log   *slog.Logger
}

// NewNoteService wires the note repository together with object storage. Pass a
// nil *S3Service where attachment cleanup is not needed/available; a nil logger
// falls back to slog.Default().
func NewNoteService(notes *repository.NoteRepository, s3 *S3Service, log *slog.Logger) *NoteService {
	if log == nil {
		log = slog.Default()
	}
	return &NoteService{notes: notes, s3: s3, log: log}
}

// List returns notes for the user. includeHidden should only be true once the
// caller has verified the PIN for this session.
func (s *NoteService) List(ctx context.Context, userID uuid.UUID, includeHidden bool) ([]*models.Note, error) {
	return s.notes.ListByUser(ctx, userID, includeHidden)
}

func (s *NoteService) Get(ctx context.Context, userID, id uuid.UUID) (*models.Note, error) {
	return s.notes.GetByID(ctx, userID, id)
}

func (s *NoteService) Create(ctx context.Context, n *models.Note) (*models.Note, error) {
	return s.notes.Create(ctx, n)
}

func (s *NoteService) Update(ctx context.Context, n *models.Note) (*models.Note, error) {
	return s.notes.Update(ctx, n)
}

// Delete removes a note, purging its attachments from object storage first. If
// S3 cleanup fails the note is kept and the error is returned so a retry can
// finish the job — same contract as TaskService.Delete.
func (s *NoteService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	attachments, err := s.notes.AttachmentsByID(ctx, userID, id)
	if err != nil {
		return err
	}
	if err := s.deleteAttachments(ctx, id, attachments); err != nil {
		return err
	}
	return s.notes.Delete(ctx, userID, id)
}

// deleteAttachments best-effort removes a note's files from S3.
//
//   - A URL that is not one of our S3 objects, or a file already gone, counts
//     as done.
//   - Any real storage error aborts and is returned as ErrAttachmentCleanup
//     (defined in task_service.go) so the caller does not touch the DB.
func (s *NoteService) deleteAttachments(ctx context.Context, noteID uuid.UUID, attachments []models.Attachment) error {
	if len(attachments) == 0 {
		return nil
	}
	if s.s3 == nil {
		s.log.Warn("note attachment cleanup skipped: no object storage in this process",
			"note_id", noteID, "count", len(attachments))
		return nil
	}

	for _, a := range attachments {
		if a.URL == "" {
			continue
		}
		key, ok := s.s3.KeyFromURL(a.URL)
		if !ok {
			s.log.Warn("note attachment URL not recognised as an S3 object, skipping",
				"note_id", noteID, "url", a.URL)
			continue
		}
		if err := s.s3.DeleteObject(ctx, key); err != nil {
			s.log.Error("delete note attachment from S3", "note_id", noteID, "key", key, "error", err)
			return fmt.Errorf("%w: key %s: %v", ErrAttachmentCleanup, key, err)
		}
		s.log.Info("note attachment deleted from S3", "note_id", noteID, "key", key)
	}
	return nil
}
