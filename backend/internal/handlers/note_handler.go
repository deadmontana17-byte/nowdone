package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nowdone/internal/models"
	"nowdone/internal/repository"
	"nowdone/internal/service"
)

// NoteHandler exposes CRUD for global notes. Hidden notes require the
// "X-PIN-Unlocked" session flag, set after a successful /auth/pin/verify call
// on the frontend for this browser session (see note below on statelessness).
type NoteHandler struct {
	notes *service.NoteService
	log   *slog.Logger
}

func NewNoteHandler(notes *service.NoteService, log *slog.Logger) *NoteHandler {
	return &NoteHandler{notes: notes, log: log}
}

// GET /notes?unlocked=true — unlocked must only be sent by the frontend after
// the user has verified their PIN in the current session.
func (h *NoteHandler) List(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	includeHidden := c.Query("unlocked") == "true"

	notes, err := h.notes.List(c.Request.Context(), userID, includeHidden)
	if err != nil {
		h.log.Error("list notes", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось загрузить заметки")
		return
	}
	c.JSON(http.StatusOK, gin.H{"notes": notes})
}

type noteRequest struct {
	Title       string              `json:"title" binding:"required"`
	Content     models.JSONRaw      `json:"content"`
	Attachments []models.Attachment `json:"attachments"`
	IsHidden    bool                `json:"is_hidden"`
}

// POST /notes
func (h *NoteHandler) Create(c *gin.Context) {
	userID, _ := userIDFromContext(c)

	var req noteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "проверьте поля заметки")
		return
	}

	note := &models.Note{
		UserID:      userID,
		Title:       req.Title,
		Content:     req.Content,
		Attachments: req.Attachments,
		IsHidden:    req.IsHidden,
	}
	created, err := h.notes.Create(c.Request.Context(), note)
	if err != nil {
		h.log.Error("create note", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось создать заметку")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"note": created})
}

// PATCH /notes/:id
func (h *NoteHandler) Update(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "некорректный идентификатор заметки")
		return
	}

	var req noteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "проверьте поля заметки")
		return
	}

	note := &models.Note{
		ID:          id,
		UserID:      userID,
		Title:       req.Title,
		Content:     req.Content,
		Attachments: req.Attachments,
		IsHidden:    req.IsHidden,
	}
	updated, err := h.notes.Update(c.Request.Context(), note)
	if err != nil {
		if err == repository.ErrNotFound {
			respondError(c, http.StatusNotFound, "заметка не найдена")
			return
		}
		h.log.Error("update note", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось обновить заметку")
		return
	}
	c.JSON(http.StatusOK, gin.H{"note": updated})
}

// DELETE /notes/:id
func (h *NoteHandler) Delete(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "некорректный идентификатор заметки")
		return
	}

	if err := h.notes.Delete(c.Request.Context(), userID, id); err != nil {
		if err == repository.ErrNotFound {
			respondError(c, http.StatusNotFound, "заметка не найдена")
			return
		}
		h.log.Error("delete note", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось удалить заметку")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
