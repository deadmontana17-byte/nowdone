package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"nowdone/internal/models"
	"nowdone/internal/repository"
	"nowdone/internal/service"
)

// TaskHandler exposes task CRUD, grouped by date for the planner view.
type TaskHandler struct {
	tasks    *service.TaskService
	users    *repository.UserRepository
	validate *validator.Validate
	log      *slog.Logger
}

func NewTaskHandler(tasks *service.TaskService, users *repository.UserRepository, log *slog.Logger) *TaskHandler {
	return &TaskHandler{tasks: tasks, users: users, validate: validator.New(), log: log}
}

// reminderLayouts accepted from the client, in addition to full RFC3339.
// A "naive" value (no offset) comes from <input type="datetime-local"> and is
// interpreted in the user's timezone.
var reminderLayouts = []string{"2006-01-02T15:04:05", "2006-01-02T15:04"}

// parseReminderTime turns a client-supplied reminder string into a concrete
// instant. A value carrying an offset/Z is trusted as-is; a bare wall-clock
// value is read in loc (the user's timezone). Empty input yields (nil, nil).
func parseReminderTime(raw string, loc *time.Location) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &t, nil
	}
	for _, layout := range reminderLayouts {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return &t, nil
		}
	}
	return nil, errors.New("invalid reminder_time")
}

// userLocation loads the timezone for the current user, defaulting to UTC.
func (h *TaskHandler) userLocation(c *gin.Context, userID uuid.UUID) *time.Location {
	user, err := h.users.GetByID(c.Request.Context(), userID)
	if err != nil {
		h.log.Warn("load user timezone, falling back to UTC", "user_id", userID, "error", err)
		return time.UTC
	}
	return user.Location()
}

// GET /tasks?from=YYYY-MM-DD&to=YYYY-MM-DD
func (h *TaskHandler) List(c *gin.Context) {
	userID, _ := userIDFromContext(c)

	from, to, err := parseDateRange(c.Query("from"), c.Query("to"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "некорректный диапазон дат")
		return
	}

	tasks, err := h.tasks.ListRange(c.Request.Context(), userID, from, to)
	if err != nil {
		h.log.Error("list tasks", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось загрузить задачи")
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

type createTaskRequest struct {
	TypeID      *uuid.UUID          `json:"type_id"`
	Title       string              `json:"title" binding:"required"`
	Description  models.JSONRaw      `json:"description"`
	Attachments []models.Attachment `json:"attachments"`
	Date        string              `json:"date" binding:"required"`
	// Wall-clock "2006-01-02T15:04" (interpreted in the user's timezone) or a
	// full RFC3339 timestamp. Null/omitted means no reminder.
	ReminderTime   *string                `json:"reminder_time"`
	IsRecurring    bool                   `json:"is_recurring"`
	RecurrenceRule *models.RecurrenceRule `json:"recurrence_rule"`
}

// POST /tasks
func (h *TaskHandler) Create(c *gin.Context) {
	userID, _ := userIDFromContext(c)

	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "проверьте поля задачи")
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		respondError(c, http.StatusBadRequest, "некорректная дата")
		return
	}

	var reminder *time.Time
	if req.ReminderTime != nil {
		reminder, err = parseReminderTime(*req.ReminderTime, h.userLocation(c, userID))
		if err != nil {
			respondError(c, http.StatusBadRequest, "некорректное время напоминания")
			return
		}
	}

	task := &models.Task{
		UserID:         userID,
		TypeID:         req.TypeID,
		Title:          req.Title,
		Description:    req.Description,
		Attachments:    req.Attachments,
		Date:           models.NewDate(date),
		ReminderTime:   reminder,
		IsRecurring:    req.IsRecurring,
		RecurrenceRule: req.RecurrenceRule,
	}

	created, err := h.tasks.Create(c.Request.Context(), task)
	if err != nil {
		h.log.Error("create task", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось создать задачу")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"task": created})
}

type updateTaskRequest struct {
	TypeID         *uuid.UUID             `json:"type_id"`
	ClearTypeID    bool                   `json:"clear_type_id"`
	Title          *string                `json:"title"`
	Description    *models.JSONRaw        `json:"description"`
	Attachments    *[]models.Attachment   `json:"attachments"`
	Date           *string                `json:"date"`
	IsDone         *bool                  `json:"is_done"`
	ReminderTime   *string                `json:"reminder_time"`
	ClearReminder  bool                   `json:"clear_reminder"`
	IsRecurring    *bool                  `json:"is_recurring"`
	RecurrenceRule *models.RecurrenceRule `json:"recurrence_rule"`
}

// PATCH /tasks/:id
func (h *TaskHandler) Update(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "некорректный идентификатор задачи")
		return
	}

	var req updateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "проверьте поля задачи")
		return
	}

	update := repository.TaskUpdate{
		TypeID:         req.TypeID,
		ClearTypeID:    req.ClearTypeID,
		Title:          req.Title,
		Description:    req.Description,
		Attachments:    req.Attachments,
		IsDone:         req.IsDone,
		ClearReminder:  req.ClearReminder,
		IsRecurring:    req.IsRecurring,
		RecurrenceRule: req.RecurrenceRule,
	}
	if req.Date != nil {
		date, err := time.Parse("2006-01-02", *req.Date)
		if err != nil {
			respondError(c, http.StatusBadRequest, "некорректная дата")
			return
		}
		update.Date = &date
	}
	// A non-empty reminder_time replaces the reminder; an explicit empty string
	// (or clear_reminder:true) removes it; omitting the field leaves it as-is.
	if req.ReminderTime != nil {
		if strings.TrimSpace(*req.ReminderTime) == "" {
			update.ClearReminder = true
		} else {
			reminder, err := parseReminderTime(*req.ReminderTime, h.userLocation(c, userID))
			if err != nil {
				respondError(c, http.StatusBadRequest, "некорректное время напоминания")
				return
			}
			update.ReminderTime = reminder
		}
	}

	updated, err := h.tasks.Update(c.Request.Context(), userID, id, update)
	if err != nil {
		if err == repository.ErrNotFound {
			respondError(c, http.StatusNotFound, "задача не найдена")
			return
		}
		if errors.Is(err, service.ErrAttachmentCleanup) {
			h.log.Error("update task: attachment cleanup failed", "error", err)
			respondError(c, http.StatusBadGateway, "не удалось удалить файл из хранилища, изменения не сохранены")
			return
		}
		h.log.Error("update task", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось обновить задачу")
		return
	}
	c.JSON(http.StatusOK, gin.H{"task": updated})
}

// DELETE /tasks/:id
func (h *TaskHandler) Delete(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "некорректный идентификатор задачи")
		return
	}

	if err := h.tasks.Delete(c.Request.Context(), userID, id); err != nil {
		if err == repository.ErrNotFound {
			respondError(c, http.StatusNotFound, "задача не найдена")
			return
		}
		if errors.Is(err, service.ErrAttachmentCleanup) {
			h.log.Error("delete task: attachment cleanup failed", "error", err)
			respondError(c, http.StatusBadGateway, "не удалось удалить файлы задачи из хранилища, задача не удалена")
			return
		}
		h.log.Error("delete task", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось удалить задачу")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func parseDateRange(fromStr, toStr string) (time.Time, time.Time, error) {
	layout := "2006-01-02"
	now := time.Now()

	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, -1)

	var err error
	if fromStr != "" {
		from, err = time.Parse(layout, fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if toStr != "" {
		to, err = time.Parse(layout, toStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	return from, to, nil
}
