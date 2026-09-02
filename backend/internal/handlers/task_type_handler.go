package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nowdone/internal/repository"
	"nowdone/internal/service"
)

// TaskTypeHandler exposes CRUD for user-defined task categories.
type TaskTypeHandler struct {
	taskTypes *service.TaskTypeService
	log       *slog.Logger
}

func NewTaskTypeHandler(taskTypes *service.TaskTypeService, log *slog.Logger) *TaskTypeHandler {
	return &TaskTypeHandler{taskTypes: taskTypes, log: log}
}

// GET /task-types
func (h *TaskTypeHandler) List(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	types, err := h.taskTypes.List(c.Request.Context(), userID)
	if err != nil {
		h.log.Error("list task types", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось загрузить типы задач")
		return
	}
	c.JSON(http.StatusOK, gin.H{"task_types": types})
}

type createTaskTypeRequest struct {
	Emoji string `json:"emoji" binding:"required"`
	Name  string `json:"name" binding:"required"`
}

// POST /task-types
func (h *TaskTypeHandler) Create(c *gin.Context) {
	userID, _ := userIDFromContext(c)

	var req createTaskTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "укажите эмодзи и название")
		return
	}

	created, err := h.taskTypes.Create(c.Request.Context(), userID, req.Emoji, req.Name)
	if err != nil {
		h.log.Error("create task type", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось создать тип задачи")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"task_type": created})
}

// DELETE /task-types/:id
func (h *TaskTypeHandler) Delete(c *gin.Context) {
	userID, _ := userIDFromContext(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "некорректный идентификатор")
		return
	}

	if err := h.taskTypes.Delete(c.Request.Context(), userID, id); err != nil {
		if err == repository.ErrNotFound {
			respondError(c, http.StatusNotFound, "тип задачи не найден")
			return
		}
		h.log.Error("delete task type", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось удалить тип задачи")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
